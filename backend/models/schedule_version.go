package models

import (
	"gorm.io/gorm"
)

// ScheduleVersion represents a user-facing historical schedule snapshot
// containing the actual schedule entries at a point in time.
// Unlike ScheduleSnapshot (which stores only aggregated statistics),
// ScheduleVersion stores the full entry set so versions can be viewed,
// restored, and compared independently of current schedule data.
type ScheduleVersion struct {
	gorm.Model
	SemesterID uint    `gorm:"index;not null" json:"semesterId"`
	Name       string  `gorm:"size:100;not null" json:"name"`
	Source     string  `gorm:"size:20;not null" json:"source"` // AutoGenerate | ManualAdjust | Import | Restore | Copy
	Score      float64 `json:"score"`
	EntryCount int     `json:"entryCount"`
	Solver     string  `gorm:"size:30" json:"solver"` // optional: which solver produced it

	// v0.5.5: 排课模式(FULL_SCHEDULING / TIME_ONLY_SCHEDULING)。
	// 前端根据这个字段决定是否展示"教室"列 / 资源评分,避免 TIME_ONLY
	// 历史版本被误解为"排课失败 - 教室分配 0"。
	Mode string `gorm:"size:32;default:FULL_SCHEDULING" json:"mode,omitempty"`

	Entries []ScheduleVersionEntry `gorm:"foreignKey:VersionID" json:"entries,omitempty"`
}

// TableName overrides the default table name for ScheduleVersion.
func (ScheduleVersion) TableName() string { return "schedule_versions" }

// ScheduleVersionEntry stores a single course entry within a historical
// schedule version. Fields mirror the main ScheduleEntry but are stored
// independently so historical versions remain valid even if current
// resource data (teachers, classrooms, etc.) is later modified or deleted.
type ScheduleVersionEntry struct {
	gorm.Model
	VersionID      uint   `gorm:"index;not null" json:"versionId"`
	OriginalEntry  *uint  `gorm:"index" json:"originalEntryId,omitempty"` // optional: links back to the schedule_entry row at time of capture
	TeachingTaskID *uint  `gorm:"index" json:"teachingTaskId,omitempty"`
	CourseID       uint   `gorm:"not null" json:"courseId"`
	TeacherID      uint   `gorm:"not null" json:"teacherId"`
	ClassroomID    uint   `gorm:"not null" json:"classroomId"`
	DayOfWeek      int    `gorm:"not null" json:"dayOfWeek"`
	StartPeriod    int    `gorm:"not null" json:"startPeriod"`
	Span           int    `gorm:"default:2" json:"span"`
	Weeks          string `gorm:"size:50;default:1-16" json:"weeks"`
}

// TableName overrides the default table name for ScheduleVersionEntry.
func (ScheduleVersionEntry) TableName() string { return "schedule_version_entries" }

// Source constants for ScheduleVersion.Source.
const (
	VersionSourceAutoGenerate = "AutoGenerate"
	VersionSourceManualAdjust = "ManualAdjust"
	VersionSourceImport       = "Import"
	VersionSourceRestore      = "Restore"
	VersionSourceCopy         = "Copy"
)
