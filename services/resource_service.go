package services

import (
	"scheduling-system/database"
	"scheduling-system/models"

	"gorm.io/gorm"
)

// ResourceService handles CRUD for teachers, classrooms, courses, and class groups.
type ResourceService struct{}

func NewResourceService() *ResourceService {
	return &ResourceService{}
}

// DB returns the global database instance.
func (s *ResourceService) db() *gorm.DB {
	return database.DB
}

// ===== Teachers =====

func (s *ResourceService) GetTeachers() ([]models.Teacher, error) {
	var teachers []models.Teacher
	result := s.db().Find(&teachers)
	return teachers, result.Error
}

func (s *ResourceService) CreateTeacher(t models.Teacher) error {
	return s.db().Create(&t).Error
}

func (s *ResourceService) UpdateTeacher(t models.Teacher) error {
	return s.db().Save(&t).Error
}

func (s *ResourceService) DeleteTeacher(id uint) error {
	return s.db().Delete(&models.Teacher{}, id).Error
}

// ===== Courses =====

func (s *ResourceService) GetCourses() ([]models.Course, error) {
	var courses []models.Course
	result := s.db().Find(&courses)
	return courses, result.Error
}

func (s *ResourceService) CreateCourse(c models.Course) error {
	return s.db().Create(&c).Error
}

func (s *ResourceService) UpdateCourse(c models.Course) error {
	return s.db().Save(&c).Error
}

func (s *ResourceService) DeleteCourse(id uint) error {
	return s.db().Delete(&models.Course{}, id).Error
}

// ===== Classrooms =====

func (s *ResourceService) GetClassrooms() ([]models.Classroom, error) {
	var classrooms []models.Classroom
	result := s.db().Find(&classrooms)
	return classrooms, result.Error
}

func (s *ResourceService) CreateClassroom(c models.Classroom) error {
	return s.db().Create(&c).Error
}

func (s *ResourceService) UpdateClassroom(c models.Classroom) error {
	return s.db().Save(&c).Error
}

func (s *ResourceService) DeleteClassroom(id uint) error {
	return s.db().Delete(&models.Classroom{}, id).Error
}

// ===== Class Groups =====

func (s *ResourceService) GetClassGroups() ([]models.ClassGroup, error) {
	var groups []models.ClassGroup
	result := s.db().Find(&groups)
	return groups, result.Error
}

func (s *ResourceService) CreateClassGroup(c models.ClassGroup) error {
	return s.db().Create(&c).Error
}

func (s *ResourceService) UpdateClassGroup(c models.ClassGroup) error {
	return s.db().Save(&c).Error
}

func (s *ResourceService) DeleteClassGroup(id uint) error {
	return s.db().Delete(&models.ClassGroup{}, id).Error
}

// ===== Schedule =====

func (s *ResourceService) GetScheduleEntries(semester string) ([]models.ScheduleEntry, error) {
	var entries []models.ScheduleEntry
	query := s.db().Preload("Course").Preload("Teacher").Preload("Classroom")
	if semester != "" {
		query = query.Where("semester = ?", semester)
	}
	result := query.Find(&entries)
	return entries, result.Error
}
