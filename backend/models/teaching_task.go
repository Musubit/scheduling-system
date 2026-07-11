package models

import "gorm.io/gorm"

// TeachingTask represents a teaching assignment: one course taught by one teacher
// for one or more class groups in a specific semester.
// This replaces the implicit department-based matching with explicit relationships.
type TeachingTask struct {
	gorm.Model
	CourseID         uint   `gorm:"index;not null" json:"courseId"`
	TeacherID        uint   `gorm:"index;not null" json:"teacherId"`
	SemesterID       uint   `gorm:"index;not null" json:"semesterId"`
	Status           string `gorm:"size:20;default:active" json:"status"` // active, inactive

	// Time parameters for scheduling
	TotalHours       int `gorm:"default:0" json:"totalHours"`       // 总学时（必填）
	StartWeek        int `gorm:"default:1" json:"startWeek"`        // 起始周（默认1）
	EndWeek          int `gorm:"default:16" json:"endWeek"`         // 结束周（默认16）
	MaxHoursPerWeek  int `gorm:"default:0" json:"maxHoursPerWeek"`  // 周最大学时（0=不限）

	// Associations
	Course   Course              `gorm:"foreignKey:CourseID" json:"course,omitempty"`
	Teacher  Teacher             `gorm:"foreignKey:TeacherID" json:"teacher,omitempty"`
	Semester Semester            `gorm:"foreignKey:SemesterID" json:"semester,omitempty"`
	Classes  []TeachingTaskClass `gorm:"foreignKey:TeachingTaskID" json:"classes,omitempty"`
}
