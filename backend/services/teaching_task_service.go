package services

import (
	"fmt"
	"strings"

	"scheduling-system/backend/database"
	"scheduling-system/backend/models"
)

// TeachingTaskService handles CRUD and intelligent detection for teaching tasks.
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

// ===== Intelligent Detection =====

// MergeableGroup represents a group of teaching tasks that could be merged
// (same course name + same teacher).
type MergeableGroup struct {
	CourseName  string              `json:"courseName"`
	TeacherName string              `json:"teacherName"`
	Tasks       []models.TeachingTask `json:"tasks"`
	ClassGroups []models.ClassGroup   `json:"classGroups"` // all classes across the group
}

// DetectMergeableTasks scans teaching tasks for the same semester and finds groups
// that share the same course name and teacher, suggesting they could be merged
// into a combined class offering.
func (s *TeachingTaskService) DetectMergeableTasks(semesterID uint) ([]MergeableGroup, error) {
	allTasks, err := s.ListTeachingTasks(semesterID)
	if err != nil {
		return nil, err
	}

	// Group by (courseName, teacherID)
	type key struct {
		courseName string
		teacherID  uint
	}
	groups := make(map[key][]models.TeachingTask)
	for _, t := range allTasks {
		// Load course and teacher if not preloaded
		if t.Course.ID == 0 {
			s.db.First(&t.Course, t.CourseID)
		}
		if t.Teacher.ID == 0 {
			s.db.First(&t.Teacher, t.TeacherID)
		}
		k := key{courseName: t.Course.Name, teacherID: t.TeacherID}
		groups[k] = append(groups[k], t)
	}

	var result []MergeableGroup
	for k, tasks := range groups {
		if len(tasks) < 2 {
			continue // only suggest merging when there are 2+ tasks
		}

		mg := MergeableGroup{
			CourseName:  k.courseName,
			TeacherName: tasks[0].Teacher.Name,
			Tasks:       tasks,
		}
		// Collect all class groups across tasks
		seen := make(map[uint]bool)
		for _, t := range tasks {
			for _, c := range t.Classes {
				if !seen[c.ClassGroupID] {
					seen[c.ClassGroupID] = true
					mg.ClassGroups = append(mg.ClassGroups, c.ClassGroup)
				}
			}
		}
		result = append(result, mg)
	}
	return result, nil
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

// SplitMergedTeachingTask splits a merged teaching task (one task covering
// multiple class groups) into individual single-class teaching tasks.
// Existing schedule entries are reassigned to the new task whose single class
// matches the entry's ClassGroupID (entries without a class match are attached
// to the first new task). The original merged task is then removed.
// Returns the number of new single-class tasks created.
func (s *TeachingTaskService) SplitMergedTeachingTask(taskID uint) (int, error) {
	var task models.TeachingTask
	if err := s.db.Preload("Classes").Preload("Classes.ClassGroup").First(&task, taskID).Error(); err != nil {
		return 0, fmt.Errorf("教学任务不存在: %w", err)
	}
	if len(task.Classes) < 2 {
		return 0, fmt.Errorf("该教学任务仅含 %d 个班级，无需拆班", len(task.Classes))
	}

	// Existing schedule entries that reference this merged task.
	var entries []models.ScheduleEntry
	s.db.Where("teaching_task_id = ?", taskID).Find(&entries)

	created := 0
	err := s.db.Transaction(func(tx database.DB) error {
		newTaskByClass := make(map[uint]uint)
		firstNewTaskID := uint(0)
		for _, tc := range task.Classes {
			cgID := tc.ClassGroupID
			newTask := models.TeachingTask{
				CourseID:        task.CourseID,
				TeacherID:       task.TeacherID,
				SemesterID:      task.SemesterID,
				Status:          task.Status,
				TotalHours:      task.TotalHours,
				StartWeek:       task.StartWeek,
				EndWeek:         task.EndWeek,
				MaxHoursPerWeek: task.MaxHoursPerWeek,
			}
			if err := tx.Create(&newTask).Error(); err != nil {
				return err
			}
			if err := tx.Create(&models.TeachingTaskClass{TeachingTaskID: newTask.ID, ClassGroupID: cgID}).Error(); err != nil {
				return err
			}
			newTaskByClass[cgID] = newTask.ID
			if firstNewTaskID == 0 {
				firstNewTaskID = newTask.ID
			}
			created++
		}

		// Reassign each existing entry to the matching new single-class task.
		for i := range entries {
			e := entries[i]
			targetID := firstNewTaskID
			if e.ClassGroupID != nil {
				if id, ok := newTaskByClass[*e.ClassGroupID]; ok {
					targetID = id
				}
			}
			e.TeachingTaskID = &targetID
			if err := tx.Save(&e).Error(); err != nil {
				return err
			}
		}

		// Remove the original merged task and its class associations.
		if err := tx.Delete(&models.TeachingTaskClass{}, "teaching_task_id = ?", taskID).Error(); err != nil {
			return err
		}
		if err := tx.Delete(&models.TeachingTask{}, taskID).Error(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return created, nil
}
