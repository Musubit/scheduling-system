package services

import (
	"io"
	"os"
	"scheduling-system/database"
	"scheduling-system/models"
)

// ResourceService handles CRUD for teachers, classrooms, courses, and class groups.
type ResourceService struct {
	db database.DB
}

func NewResourceService(db database.DB) *ResourceService {
	return &ResourceService{db: db}
}

// ===== Teachers =====

func (s *ResourceService) GetTeachers() ([]models.Teacher, error) {
	var teachers []models.Teacher
	result := s.db.Find(&teachers)
	return teachers, result.Error()
}

func (s *ResourceService) CreateTeacher(t models.Teacher) error {
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
	query := s.db.Preload("Course").Preload("Teacher").Preload("Classroom").Preload("ClassGroup")
	if semester != "" {
		query = query.Where("semester = ?", semester)
	}
	result := query.Find(&entries)
	return entries, result.Error()
}

// ===== Settings =====

func (s *ResourceService) SaveSetting(key, value string) error {
	var setting models.Setting
	result := s.db.Where("key = ?", key).First(&setting)
	if result.Error() != nil {
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

// ===== Database Backup / Restore =====

// GetDatabasePath returns the database file path.
func (s *ResourceService) GetDatabasePath() string {
	return "scheduling.db"
}

// BackupDatabase copies the database file to a backup location.
// Returns the backup file path.
func (s *ResourceService) BackupDatabase(backupPath string) error {
	src, err := os.Open("scheduling.db")
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

// RestoreDatabase replaces the current database with a backup file.
// WARNING: this will overwrite all current data. The application should
// restart after restore to reload the database.
func (s *ResourceService) RestoreDatabase(backupPath string) error {
	src, err := os.Open(backupPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create("scheduling.db")
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}
