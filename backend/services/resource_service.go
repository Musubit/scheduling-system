package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"scheduling-system/backend/database"
	"scheduling-system/backend/models"
	"gorm.io/gorm"
	"strings"
)

// ResourceService handles CRUD for teachers, classrooms, courses, and class groups.
type ResourceService struct {
	db database.DB
}

func NewResourceService(db database.DB) *ResourceService {
	return &ResourceService{db: db}
}

// ===== Code auto-generation =====

// generateTeacherCode creates a code like "T001" if none provided.
func (s *ResourceService) generateTeacherCode() (string, error) {
	var teachers []models.Teacher
	if err := s.db.Order("code DESC").Find(&teachers).Error(); err != nil {
		return "", fmt.Errorf("生成教师编码失败: %w", err)
	}
	return nextCodeSlice(teachers, "T"), nil
}

// generateClassroomCode creates a code like "R001" if none provided.
func (s *ResourceService) generateClassroomCode() (string, error) {
	var classrooms []models.Classroom
	if err := s.db.Order("code DESC").Find(&classrooms).Error(); err != nil {
		return "", fmt.Errorf("生成教室编码失败: %w", err)
	}
	return nextCodeSlice(classrooms, "R"), nil
}

// generateCourseCode creates a code like "C001" if none provided.
func (s *ResourceService) generateCourseCode() (string, error) {
	var courses []models.Course
	if err := s.db.Order("code DESC").Find(&courses).Error(); err != nil {
		return "", fmt.Errorf("生成课程编码失败: %w", err)
	}
	return nextCodeSlice(courses, "C"), nil
}

// generateClassGroupCode creates a code like "G001" if none provided.
func (s *ResourceService) generateClassGroupCode() (string, error) {
	var groups []models.ClassGroup
	if err := s.db.Order("code DESC").Find(&groups).Error(); err != nil {
		return "", fmt.Errorf("生成班级编码失败: %w", err)
	}
	return nextCodeSlice(groups, "G"), nil
}

// nextCode extracts the numeric part after prefix and returns prefix + padded counter.
func nextCode(maxCode, prefix string) string {
	if maxCode == "" {
		return fmt.Sprintf("%s001", prefix)
	}
	num := 0
	rest := strings.TrimPrefix(maxCode, prefix)
	if rest != maxCode && rest != "" {
		fmt.Sscanf(rest, "%d", &num)
	}
	return fmt.Sprintf("%s%03d", prefix, num+1)
}

// nextCodeSlice extracts the numeric suffix from the first element's code.
func nextCodeSlice(items interface{}, prefix string) string {
	// Use reflection-free approach by using Order + Find then accessing
	// Each model has Code field
	switch v := items.(type) {
	case []models.Teacher:
		if len(v) > 0 {
			return nextCode(v[0].Code, prefix)
		}
	case []models.Classroom:
		if len(v) > 0 {
			return nextCode(v[0].Code, prefix)
		}
	case []models.Course:
		if len(v) > 0 {
			return nextCode(v[0].Code, prefix)
		}
	case []models.ClassGroup:
		if len(v) > 0 {
			return nextCode(v[0].Code, prefix)
		}
	}
	return fmt.Sprintf("%s001", prefix)
}

// ===== Teachers =====

func (s *ResourceService) GetTeachers() ([]models.Teacher, error) {
	var teachers []models.Teacher
	result := s.db.Find(&teachers)
	return teachers, result.Error()
}

func (s *ResourceService) CreateTeacher(t models.Teacher) error {
	if err := validateTeacherSlots(t.UnavailableSlots); err != nil {
		return err
	}
	if strings.TrimSpace(t.Code) == "" {
		code, err := s.generateTeacherCode()
		if err != nil {
			return err
		}
		t.Code = code
	}
	return s.db.Create(&t).Error()
}

func (s *ResourceService) UpdateTeacher(t models.Teacher) error {
	if err := validateTeacherSlots(t.UnavailableSlots); err != nil {
		return err
	}
	return s.db.Save(&t).Error()
}

// validateTeacherSlots checks UnavailableSlots JSON value range.
func validateTeacherSlots(raw string) error {
	if raw == "" {
		return nil
	}
	var slots []struct {
		DayOfWeek   int `json:"dayOfWeek"`
		StartPeriod int `json:"startPeriod"`
		Span        int `json:"span"`
	}
	if err := json.Unmarshal([]byte(raw), &slots); err != nil {
		return fmt.Errorf("UnavailableSlots 格式错误: %w", err)
	}
	for _, s := range slots {
		if s.DayOfWeek < 0 || s.DayOfWeek > 6 {
			return fmt.Errorf("非法时段: dayOfWeek=%d (应为 0-6)", s.DayOfWeek)
		}
		if s.StartPeriod < 0 || s.StartPeriod > 10 {
			return fmt.Errorf("非法时段: startPeriod=%d (应为 0-10)", s.StartPeriod)
		}
		if s.Span < 1 || s.Span > 3 {
			return fmt.Errorf("非法时段: span=%d (应为 1-3)", s.Span)
		}
		if s.StartPeriod+s.Span > 11 {
			return fmt.Errorf("非法时段: startPeriod(%d)+span(%d) > 11", s.StartPeriod, s.Span)
		}
	}
	return nil
}

func (s *ResourceService) DeleteTeacher(id uint) error {
	return s.db.Delete(&models.Teacher{}, id).Error()
}

// ===== Courses =====

func (s *ResourceService) GetCourses() ([]models.Course, error) {
	var courses []models.Course
	result := s.db.Find(&courses)
	return courses, result.Error()
}

func (s *ResourceService) CreateCourse(c models.Course) error {
	if strings.TrimSpace(c.Code) == "" {
		code, err := s.generateCourseCode()
		if err != nil {
			return err
		}
		c.Code = code
	}
	return s.db.Create(&c).Error()
}

func (s *ResourceService) UpdateCourse(c models.Course) error {
	return s.db.Save(&c).Error()
}

func (s *ResourceService) DeleteCourse(id uint) error {
	return s.db.Delete(&models.Course{}, id).Error()
}

// ===== Buildings =====

// GetBuildings 返回全部教学楼列表（v0.5.5 Stage B 新增，供前端下拉选择）。
// 仅返回 code/name/category/status 等元数据；classrooms 通过 GetClassrooms 独立查询。
func (s *ResourceService) GetBuildings() ([]models.Building, error) {
	var buildings []models.Building
	result := s.db.Order("code asc").Find(&buildings)
	return buildings, result.Error()
}

// ===== Classrooms =====

func (s *ResourceService) GetClassrooms() ([]models.Classroom, error) {
	var classrooms []models.Classroom
	// v0.5.5 Stage B: Preload Building 以便前端渲染教学楼名称。
	result := s.db.Preload("Building").Find(&classrooms)
	return classrooms, result.Error()
}

func (s *ResourceService) CreateClassroom(c models.Classroom) error {
	if strings.TrimSpace(c.Code) == "" {
		code, err := s.generateClassroomCode()
		if err != nil {
			return err
		}
		c.Code = code
	}
	return s.db.Create(&c).Error()
}

func (s *ResourceService) UpdateClassroom(c models.Classroom) error {
	return s.db.Save(&c).Error()
}

func (s *ResourceService) DeleteClassroom(id uint) error {
	return s.db.Delete(&models.Classroom{}, id).Error()
}

// ===== Class Groups =====

func (s *ResourceService) GetClassGroups() ([]models.ClassGroup, error) {
	var groups []models.ClassGroup
	result := s.db.Find(&groups)
	return groups, result.Error()
}

func (s *ResourceService) CreateClassGroup(c models.ClassGroup) error {
	if strings.TrimSpace(c.Code) == "" {
		code, err := s.generateClassGroupCode()
		if err != nil {
			return err
		}
		c.Code = code
	}
	return s.db.Create(&c).Error()
}

func (s *ResourceService) UpdateClassGroup(c models.ClassGroup) error {
	return s.db.Save(&c).Error()
}

func (s *ResourceService) DeleteClassGroup(id uint) error {
	return s.db.Delete(&models.ClassGroup{}, id).Error()
}

// ===== Schedule =====

func (s *ResourceService) GetScheduleEntries(semesterID uint) ([]EnrichedScheduleEntry, error) {
	// v0.6.2: Read from time_assignments (time data) + LEFT JOIN schedule_entries
	// (room data) + teaching_tasks (course/teacher/class info).
	// The old ScheduleEntry model no longer carries time fields after TA+SE split.

	// 1. Load all TimeAssignments for the semester.
	var tas []models.TimeAssignment
	q := s.db.Where("semester_id = ?", semesterID)
	if err := q.Order("id ASC").Find(&tas).Error(); err != nil {
		return nil, err
	}
	if len(tas) == 0 {
		return nil, nil
	}

	// 2. Collect TeachingTaskIDs for batch load.
	ttIDs := make([]uint, 0, len(tas))
	seenTT := make(map[uint]bool)
	for _, ta := range tas {
		if !seenTT[ta.TeachingTaskID] {
			seenTT[ta.TeachingTaskID] = true
			ttIDs = append(ttIDs, ta.TeachingTaskID)
		}
	}

	// 3. Batch load TeachingTasks with Course + Teacher + Classes.
	var tts []models.TeachingTask
	if err := s.db.Where("id IN ?", ttIDs).
		Preload("Course").Preload("Teacher").
		Preload("Classes.ClassGroup").Find(&tts).Error(); err != nil {
		return nil, err
	}
	ttByID := make(map[uint]models.TeachingTask, len(tts))
	for _, tt := range tts {
		ttByID[tt.ID] = tt
	}

	// 4. Batch load ScheduleEntries (room data) for this semester.
	var ses []models.ScheduleEntry
	taIDs := make([]uint, len(tas))
	for i, ta := range tas {
		taIDs[i] = ta.ID
	}
	s.db.Where("time_assignment_id IN ?", taIDs).
		Preload("Classroom.Building").Find(&ses)
	seByTA := make(map[uint]models.ScheduleEntry, len(ses))
	for _, se := range ses {
		seByTA[se.TimeAssignmentID] = se
	}

	// 5. Build EnrichedScheduleEntry for each TimeAssignment.
	result := make([]EnrichedScheduleEntry, 0, len(tas))
	for _, ta := range tas {
		tt, ok := ttByID[ta.TeachingTaskID]
		if !ok {
			continue
		}
		enriched := EnrichedScheduleEntry{
			ID:                ta.ID,
			DayOfWeek:         int(ta.DayOfWeek),
			StartPeriod:       int(ta.StartPeriod),
			Span:              ta.Span,
			Weeks:             fmt.Sprintf("%d-%d", tt.StartWeek, tt.EndWeek),
			TeacherID:         tt.TeacherID,
			TeacherName:       tt.Teacher.Name,
			CourseID:          tt.CourseID,
			CourseName:        tt.Course.Name,
			CourseCode:        tt.Course.Code,
			CourseCredit:      tt.Course.Credit,
			TeachingTaskID:    &ta.TeachingTaskID,
			SemesterID:        ta.SemesterID,
			ScheduleVersionID: ta.ScheduleVersionID,
		}
		// Class info from TeachingTask
		for _, cls := range tt.Classes {
			enriched.ClassGroupIDs = append(enriched.ClassGroupIDs, cls.ClassGroupID)
			enriched.ClassGroupNames = append(enriched.ClassGroupNames, cls.ClassGroup.Name)
		}
		// Room info from ScheduleEntry (FULL mode only)
		if se, hasRoom := seByTA[ta.ID]; hasRoom {
			cid := se.ClassroomID
			enriched.ClassroomID = &cid
			cname := se.Classroom.Name
			enriched.ClassroomName = &cname
			floor := se.Classroom.Floor
			enriched.ClassroomFloor = &floor
			rtype := se.Classroom.RoomType
			enriched.ClassroomType = &rtype
			ccode := se.Classroom.Code
			enriched.ClassroomCode = &ccode
			if se.Classroom.Building.ID != 0 {
				bname := se.Classroom.Building.Name
				enriched.BuildingName = &bname
			}
		}
		result = append(result, enriched)
	}
	return result, nil
}

// ===== Settings =====

func (s *ResourceService) SaveSetting(key, value string) error {
	var setting models.Setting
	err := s.db.Where("key = ?", key).First(&setting).Error()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new
		return s.db.Create(&models.Setting{Key: key, Value: value}).Error()
	}
	// Update existing
	setting.Value = value
	return s.db.Save(&setting).Error()
}

func (s *ResourceService) GetSetting(key string) (string, error) {
	var setting models.Setting
	if err := s.db.Where("key = ?", key).First(&setting).Error(); err != nil {
		// "not found" is a normal empty result, not an error — frontend has its own fallback
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return setting.Value, nil
}

// ===== Semesters =====

func (s *ResourceService) GetSemesters() ([]models.Semester, error) {
	var semesters []models.Semester
	result := s.db.Order("id desc").Find(&semesters)
	return semesters, result.Error()
}

// GetActiveSemester returns the semester with Status="active", or (nil, nil)
// when none exists. v0.5.5 修订：新 seed 只造 planned 学期，未启用前查询会命中
// gorm.ErrRecordNotFound —— 这不是错误状态，前端 initSemester 需要区分
// "查询失败" 与 "尚无 active 学期"，因此正常返回空指针而不是抛错。
func (s *ResourceService) GetActiveSemester() (*models.Semester, error) {
	var semester models.Semester
	err := s.db.Where("status = ?", models.SemesterStatusActive).First(&semester).Error()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &semester, nil
}

func (s *ResourceService) CreateSemester(sem models.Semester) error {
	normalizeSemester(&sem)
	if err := s.db.Create(&sem).Error(); err != nil {
		return err
	}
	if sem.Status == models.SemesterStatusActive {
		return s.demoteOtherActiveSemesters(sem.ID)
	}
	return nil
}

func (s *ResourceService) UpdateSemester(sem models.Semester) error {
	normalizeSemester(&sem)
	// Save() 会写全字段；EndDate 已由 normalizeSemester 补齐。
	if err := s.db.Save(&sem).Error(); err != nil {
		return err
	}
	if sem.Status == models.SemesterStatusActive {
		return s.demoteOtherActiveSemesters(sem.ID)
	}
	return nil
}

func (s *ResourceService) DeleteSemester(id uint) error {
	return s.db.Delete(&models.Semester{}, id).Error()
}

// normalizeSemester 做两件事：
//  1. StartDate 非空但 EndDate 空/零值时，自动补 = StartDate + 18 周 - 1 天。
//  2. Status 为空时默认 planned（前端只发 active/planned 二选一 switch 时兜底）。
func normalizeSemester(sem *models.Semester) {
	if sem.Status == "" {
		sem.Status = models.SemesterStatusPlanned
	}
	if !sem.StartDate.IsZero() && sem.EndDate.IsZero() {
		sem.EndDate = sem.StartDate.AddDate(0, 0, 18*7-1)
	}
}

// demoteOtherActiveSemesters 保证任一时刻仅一个 active 学期：
// 把除 keepID 外所有 active 学期改为 archived。前端"设为当前学期"开关只做启用，
// 不触发降级到 planned —— 归档语义更贴合业务（历史学期不再动）。
//
// 使用 Find+Save 组合而非批量 UPDATE：DB interface 未暴露 Update；
// active 学期在业务上通常最多 1 条，逐条 Save 的开销可忽略。
func (s *ResourceService) demoteOtherActiveSemesters(keepID uint) error {
	var others []models.Semester
	if err := s.db.Where("status = ? AND id <> ?", models.SemesterStatusActive, keepID).
		Find(&others).Error(); err != nil {
		return err
	}
	for i := range others {
		others[i].Status = models.SemesterStatusArchived
		if err := s.db.Save(&others[i]).Error(); err != nil {
			return err
		}
	}
	return nil
}

// ===== Departments =====

func (s *ResourceService) GetDepartments() ([]models.Department, error) {
	var departments []models.Department
	result := s.db.Order("id asc").Find(&departments)
	return departments, result.Error()
}

func (s *ResourceService) CreateDepartment(dept models.Department) error {
	return s.db.Create(&dept).Error()
}

func (s *ResourceService) UpdateDepartment(dept models.Department) error {
	return s.db.Save(&dept).Error()
}

func (s *ResourceService) DeleteDepartment(id uint) error {
	return s.db.Delete(&models.Department{}, id).Error()
}

// ===== Database Backup / Restore =====

// GetDatabasePath returns the absolute path to the active database file.
func (s *ResourceService) GetDatabasePath() string {
	return database.GetDBPath()
}

// copyFile copies src to dst, creating parent dirs as needed.
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

// BackupDatabase copies the database file to a backup location.
func (s *ResourceService) BackupDatabase(backupPath string) error {
	return copyFile(database.GetDBPath(), backupPath)
}

// RestoreDatabase replaces the current database with a backup file.
// WARNING: this will overwrite all current data. The application should
// restart after restore to reload the database.
func (s *ResourceService) RestoreDatabase(backupPath string) error {
	return copyFile(backupPath, database.GetDBPath())
}

// OpenDownloads opens the user's Downloads folder in the system file explorer.
func (s *ResourceService) OpenDownloads() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	downloads := home + string(os.PathSeparator) + "Downloads"
	openInExplorer(downloads)
}

func openInExplorer(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	cmd.Start()
}
