package services

import (
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
