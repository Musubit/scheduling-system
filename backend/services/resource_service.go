package services

import (
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
func (s *ResourceService) generateTeacherCode() string {
	var teachers []models.Teacher
	s.db.Order("code DESC").Find(&teachers)
	return nextCodeSlice(teachers, "T")
}

// generateClassroomCode creates a code like "R001" if none provided.
func (s *ResourceService) generateClassroomCode() string {
	var classrooms []models.Classroom
	s.db.Order("code DESC").Find(&classrooms)
	return nextCodeSlice(classrooms, "R")
}

// generateCourseCode creates a code like "C001" if none provided.
func (s *ResourceService) generateCourseCode() string {
	var courses []models.Course
	s.db.Order("code DESC").Find(&courses)
	return nextCodeSlice(courses, "C")
}

// generateClassGroupCode creates a code like "G001" if none provided.
func (s *ResourceService) generateClassGroupCode() string {
	var groups []models.ClassGroup
	s.db.Order("code DESC").Find(&groups)
	return nextCodeSlice(groups, "G")
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
	if strings.TrimSpace(t.Code) == "" {
		t.Code = s.generateTeacherCode()
	}
	return s.db.Create(&t).Error()
}

func (s *ResourceService) UpdateTeacher(t models.Teacher) error {
	return s.db.Save(&t).Error()
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
		c.Code = s.generateCourseCode()
	}
	return s.db.Create(&c).Error()
}

func (s *ResourceService) UpdateCourse(c models.Course) error {
	return s.db.Save(&c).Error()
}

func (s *ResourceService) DeleteCourse(id uint) error {
	return s.db.Delete(&models.Course{}, id).Error()
}

// ===== Classrooms =====

func (s *ResourceService) GetClassrooms() ([]models.Classroom, error) {
	var classrooms []models.Classroom
	result := s.db.Find(&classrooms)
	return classrooms, result.Error()
}

func (s *ResourceService) CreateClassroom(c models.Classroom) error {
	if strings.TrimSpace(c.Code) == "" {
		c.Code = s.generateClassroomCode()
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
		c.Code = s.generateClassGroupCode()
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

func (s *ResourceService) GetScheduleEntries(semester string) ([]models.ScheduleEntry, error) {
	var entries []models.ScheduleEntry
	// Preload TeachingTask + its Classes so the schedule-center class filter can
	// resolve entries by class group (auto-scheduled entries carry TeachingTaskID,
	// not the legacy single ClassGroupID; 合班 tasks have multiple classes).
	query := s.db.Preload("Course").Preload("Teacher").Preload("Classroom").
			Preload("TeachingTask.Classes.ClassGroup")
	if semester != "" {
		query = query.Where("semester = ?", semester)
	}
	result := query.Find(&entries)
	return entries, result.Error()
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

func (s *ResourceService) GetActiveSemester() (*models.Semester, error) {
	var semester models.Semester
	if err := s.db.Where("is_active = ?", true).First(&semester).Error(); err != nil {
		return nil, err
	}
	return &semester, nil
}

func (s *ResourceService) CreateSemester(sem models.Semester) error {
	// If this is the first semester or marked active, deactivate all others
	if sem.IsActive {
		var active []models.Semester
		if err := s.db.Where("is_active = ?", true).Find(&active).Error(); err != nil {
			return err
		}
		for _, a := range active {
			a.IsActive = false
			if err := s.db.Save(&a).Error(); err != nil {
				return err
			}
		}
	}
	return s.db.Create(&sem).Error()
}

func (s *ResourceService) UpdateSemester(sem models.Semester) error {
	// If activating this semester, deactivate all others
	if sem.IsActive {
		var active []models.Semester
		if err := s.db.Where("is_active = ?", true).Find(&active).Error(); err != nil {
			return err
		}
		for _, a := range active {
			if a.ID == sem.ID {
				continue
			}
			a.IsActive = false
			if err := s.db.Save(&a).Error(); err != nil {
				return err
			}
		}
	}
	return s.db.Save(&sem).Error()
}

func (s *ResourceService) DeleteSemester(id uint) error {
	return s.db.Delete(&models.Semester{}, id).Error()
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
