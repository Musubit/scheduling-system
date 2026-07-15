package services

import (
	"fmt"
	"strings"

	"scheduling-system/backend/database"
	"scheduling-system/backend/models"
)

// TeachingTaskService handles CRUD, import/export for teaching tasks.
// v0.5.4: 自动智能合班能力已移除；多班绑定仅通过显式创建/更新/导入完成。
type TeachingTaskService struct {
	db database.DB
}

func NewTeachingTaskService(db database.DB) *TeachingTaskService {
	return &TeachingTaskService{db: db}
}

// ===== CRUD =====

// CreateTeachingTask creates a teaching task with its class associations.
func (s *TeachingTaskService) CreateTeachingTask(task models.TeachingTask, classGroupIDs []uint) error {
	return s.db.Transaction(func(tx database.DB) error {
		if err := tx.Create(&task).Error(); err != nil {
			return err
		}
		for _, cgID := range classGroupIDs {
			ttc := models.TeachingTaskClass{
				TeachingTaskID: task.ID,
				ClassGroupID:   cgID,
			}
			if err := tx.Create(&ttc).Error(); err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateTeachingTask updates a teaching task and replaces its class associations.
func (s *TeachingTaskService) UpdateTeachingTask(id uint, task models.TeachingTask, classGroupIDs []uint) error {
	return s.db.Transaction(func(tx database.DB) error {
		task.ID = id
		if err := tx.Save(&task).Error(); err != nil {
			return err
		}
		// Delete old class associations (hard delete to avoid UNIQUE constraint)
		if err := tx.Unscoped().Delete(&models.TeachingTaskClass{}, "teaching_task_id = ?", id).Error(); err != nil {
			return err
		}
		// Create new class associations
		for _, cgID := range classGroupIDs {
			ttc := models.TeachingTaskClass{
				TeachingTaskID: id,
				ClassGroupID:   cgID,
			}
			if err := tx.Create(&ttc).Error(); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteTeachingTask soft-deletes a teaching task and its class associations.
func (s *TeachingTaskService) DeleteTeachingTask(id uint) error {
	return s.db.Transaction(func(tx database.DB) error {
		if err := tx.Delete(&models.TeachingTaskClass{}, "teaching_task_id = ?", id).Error(); err != nil {
			return err
		}
		return tx.Delete(&models.TeachingTask{}, id).Error()
	})
}

// ListTeachingTasks returns all teaching tasks for a semester, with preloaded relations.
func (s *TeachingTaskService) ListTeachingTasks(semesterID uint) ([]models.TeachingTask, error) {
	var tasks []models.TeachingTask
	result := s.db.
		Where("semester_id = ?", semesterID).
		Preload("Course").
		Preload("Teacher").
		Find(&tasks)
	if result.Error() != nil {
		return nil, result.Error()
	}
	// Manually populate Classes for each task and ClassGroup within each class
	for i := range tasks {
		var classes []models.TeachingTaskClass
		if err := s.db.Where("teaching_task_id = ?", tasks[i].ID).
			Preload("ClassGroup").Find(&classes).Error(); err != nil {
			return nil, err
		}
		tasks[i].Classes = classes
	}
	return tasks, nil
}

// GetTeachingTask returns a single teaching task with relations.
func (s *TeachingTaskService) GetTeachingTask(id uint) (*models.TeachingTask, error) {
	var task models.TeachingTask
	if err := s.db.First(&task, id).Error(); err != nil {
		return nil, err
	}
	if err := s.db.Where("teaching_task_id = ?", task.ID).
		Preload("ClassGroup").Find(&task.Classes).Error(); err != nil {
		return nil, err
	}
	// Preload Course and Teacher
	if err := s.db.First(&task.Course, task.CourseID).Error(); err != nil {
		return nil, err
	}
	if err := s.db.First(&task.Teacher, task.TeacherID).Error(); err != nil {
		return nil, err
	}
	return &task, nil
}

// ===== Import / Export =====

// ImportTeachingTask represents one row of the Excel import template.
type ImportTeachingTask struct {
	CourseCode      string `json:"courseCode"`
	TeacherCode     string `json:"teacherCode"`
	ClassGroupIDs   string `json:"classGroupIDs"`   // comma-separated class group codes
	TotalHours      int    `json:"totalHours"`       // optional, 0 = use course default
	StartWeek       int    `json:"startWeek"`        // optional, default 1
	EndWeek         int    `json:"endWeek"`          // optional, default 16
	MaxHoursPerWeek int    `json:"maxHoursPerWeek"`  // optional, 0 = no limit
}

// ImportTeachingTasks imports teaching tasks from a 2D string array.
// Columns: [课程代码, 教师编号, 班级编号, 总学时?, 起始周?, 结束周?, 周最大学时?]
func (s *TeachingTaskService) ImportTeachingTasks(semesterID uint, rows [][]string) (int, []string, error) {
	if len(rows) > 5000 {
		return 0, nil, fmt.Errorf("导入行数超过上限 5000（当前 %d 行）", len(rows))
	}

	var errors []string
	imported := 0

	for i, row := range rows {
		if len(row) < 3 {
			errors = append(errors, fmt.Sprintf("第%d行: 列数不足（至少需要3列：课程代码、工号、班级代码）", i+2))
			continue
		}
		courseCode := strings.TrimSpace(row[0])
		teacherCode := strings.TrimSpace(row[1])
		classCodesStr := strings.TrimSpace(row[2])

		// Cell length validation
		if len(courseCode) > 128 || len(teacherCode) > 128 || len(classCodesStr) > 512 {
			errors = append(errors, fmt.Sprintf("第%d行: 字段长度超限", i+2))
			continue
		}

		// Find course by code
		var course models.Course
		if err := s.db.Where("code = ?", courseCode).First(&course).Error(); err != nil {
			errors = append(errors, fmt.Sprintf("第%d行: 课程代码 %s 未找到", i+2, courseCode))
			continue
		}

		// Find teacher by code
		var teacher models.Teacher
		if err := s.db.Where("code = ?", teacherCode).First(&teacher).Error(); err != nil {
			errors = append(errors, fmt.Sprintf("第%d行: 工号 %s 未找到", i+2, teacherCode))
			continue
		}

		// Parse class group codes
		classCodes := strings.Split(classCodesStr, ",")
		var classGroupIDs []uint
		for _, cc := range classCodes {
			cc = strings.TrimSpace(cc)
			if cc == "" {
				continue
			}
			var cg models.ClassGroup
			if err := s.db.Where("code = ?", cc).First(&cg).Error(); err != nil {
				errors = append(errors, fmt.Sprintf("第%d行: 班级代码 %s 未找到", i+2, cc))
				continue
			}
			classGroupIDs = append(classGroupIDs, cg.ID)
		}

		if len(classGroupIDs) == 0 {
			errors = append(errors, fmt.Sprintf("第%d行: 未指定有效班级", i+2))
			continue
		}

		// Parse optional time fields
		totalHours := 0
		startWeek := 1
		endWeek := 16
		maxHoursPerWeek := 0

		if len(row) > 3 {
			if v := strings.TrimSpace(row[3]); v != "" {
				fmt.Sscanf(v, "%d", &totalHours)
			}
		}
		if totalHours == 0 {
			totalHours = course.Hours // fallback to course default
		}
		if len(row) > 4 {
			if v := strings.TrimSpace(row[4]); v != "" {
				fmt.Sscanf(v, "%d", &startWeek)
			}
		}
		if len(row) > 5 {
			if v := strings.TrimSpace(row[5]); v != "" {
				fmt.Sscanf(v, "%d", &endWeek)
			}
		}
		if len(row) > 6 {
			if v := strings.TrimSpace(row[6]); v != "" {
				fmt.Sscanf(v, "%d", &maxHoursPerWeek)
			}
		}
		if startWeek < 1 { startWeek = 1 }
		if endWeek < startWeek { endWeek = startWeek }
		if endWeek > 30 { endWeek = 30 }
		if totalHours < 0 { totalHours = 0 }
		if maxHoursPerWeek < 0 { maxHoursPerWeek = 0 }

		task := models.TeachingTask{
			CourseID:        course.ID,
			TeacherID:       teacher.ID,
			SemesterID:      semesterID,
			Status:          "active",
			TotalHours:      totalHours,
			StartWeek:       startWeek,
			EndWeek:         endWeek,
			MaxHoursPerWeek: maxHoursPerWeek,
		}

		if err := s.CreateTeachingTask(task, classGroupIDs); err != nil {
			errors = append(errors, fmt.Sprintf("第%d行: 创建失败 - %v", i+2, err))
			continue
		}
		imported++
	}

	return imported, errors, nil
}

// ExportTeachingTasks returns teaching task data formatted for Excel export.
func (s *TeachingTaskService) ExportTeachingTasks(semesterID uint) ([][]string, error) {
	tasks, err := s.ListTeachingTasks(semesterID)
	if err != nil {
		return nil, err
	}

	rows := [][]string{
		{"课程代码", "工号", "班级代码", "总学时", "起始周", "结束周", "周最大学时"},
	}
	for _, t := range tasks {
		// Load course and teacher
		var course models.Course
		s.db.First(&course, t.CourseID)
		var teacher models.Teacher
		s.db.First(&teacher, t.TeacherID)

		var classCodes []string
		for _, c := range t.Classes {
			classCodes = append(classCodes, c.ClassGroup.Code)
		}
		row := []string{
			course.Code,
			teacher.Code,
			strings.Join(classCodes, ","),
			fmt.Sprintf("%d", t.TotalHours),
			fmt.Sprintf("%d", t.StartWeek),
			fmt.Sprintf("%d", t.EndWeek),
			fmt.Sprintf("%d", t.MaxHoursPerWeek),
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// SplitMergedTeachingTask 已于 v0.5.4 移除。
// 若需要拆分手动多班任务，请在 UI 中删除该任务后重新按单班分别创建。
