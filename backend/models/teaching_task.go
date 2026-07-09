package models

import "gorm.io/gorm"

// TeachingTask represents a teaching assignment: one course taught by one teacher
// for one or more class groups in a specific semester.
// This replaces the implicit department-based matching with explicit relationships.
type TeachingTask struct {
	gorm.Model
	CourseID   uint   `gorm:"index;not null" json:"courseId"`
	TeacherID  uint   `gorm:"index;not null" json:"teacherId"`
	SemesterID uint   `gorm:"index;not null" json:"semesterId"`
	Status     string `gorm:"size:20;default:active" json:"status"` // active, inactive

	// Associations
	Course   Course              `gorm:"foreignKey:CourseID" json:"course,omitempty"`
	Teacher  Teacher             `gorm:"foreignKey:TeacherID" json:"teacher,omitempty"`
	Semester Semester            `gorm:"foreignKey:SemesterID" json:"semester,omitempty"`
	Classes  []TeachingTaskClass `gorm:"foreignKey:TeachingTaskID" json:"classes,omitempty"`
}
