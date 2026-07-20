package models

import "gorm.io/gorm"

// ScheduleEntry 是 v0.6.0 模型拆分后的资源分配实体。
// 一条 ScheduleEntry = 一个 TimeAssignment 分配到一间 Classroom。
// TIME_ONLY 模式下本表为零行 (INV-E1)。
type ScheduleEntry struct {
	gorm.Model
	SemesterID        uint `gorm:"index;not null" json:"semesterId"`
	ScheduleVersionID uint `gorm:"index;not null" json:"scheduleVersionId"`
	TimeAssignmentID  uint `gorm:"uniqueIndex;not null" json:"timeAssignmentId"`
	ClassroomID       uint `gorm:"index;not null" json:"classroomId"`

	// Associations (read-only preloads)
	TimeAssignment *TimeAssignment `gorm:"foreignKey:TimeAssignmentID" json:"timeAssignment,omitempty"`
	Classroom      Classroom       `gorm:"foreignKey:ClassroomID" json:"classroom,omitempty"`
}
