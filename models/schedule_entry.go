package models

import "gorm.io/gorm"

// ScheduleEntry represents a single scheduled course slot.
type ScheduleEntry struct {
	gorm.Model
	CourseID    uint   `gorm:"index;not null" json:"courseId"`
	TeacherID   uint   `gorm:"index;not null" json:"teacherId"`
	ClassroomID uint   `gorm:"uniqueIndex:idx_schedule_room;not null" json:"classroomId"`
	Semester    string `gorm:"size:50;uniqueIndex:idx_schedule_room" json:"semester"`
	DayOfWeek   int    `gorm:"uniqueIndex:idx_schedule_room;not null" json:"dayOfWeek"`
	StartPeriod int    `gorm:"uniqueIndex:idx_schedule_room;not null" json:"startPeriod"` // 0-10 (对应第1-11节)
	Span        int    `gorm:"default:2" json:"span"`       // consecutive periods
	Weeks       string `gorm:"size:50;default:1-16" json:"weeks"`

	// Associations
	Course    Course    `gorm:"foreignKey:CourseID" json:"course,omitempty"`
	Teacher   Teacher   `gorm:"foreignKey:TeacherID" json:"teacher,omitempty"`
	Classroom Classroom `gorm:"foreignKey:ClassroomID" json:"classroom,omitempty"`
}
