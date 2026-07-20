package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ScheduleVersion is the single unified entity representing a saved schedule state.
// It combines the former ScheduleSnapshot (scoring details) and ScheduleVersion
// (actual entries) into one record. Every version has both the full entry set and
// the complete scoring breakdown, eliminating the previous split-brain design.
type ScheduleVersion struct {
	gorm.Model
	SemesterID uint   `gorm:"index;not null" json:"semesterId"`
	Name       string `gorm:"size:100;not null" json:"name"`
	Source     string `gorm:"size:20;not null" json:"source"` // AutoGenerate | ManualAdjust | Import | Restore | Copy
	Solver     string `gorm:"size:30" json:"solver"`
	Mode       string `gorm:"size:32;default:FULL_SCHEDULING" json:"mode,omitempty"`

	// Hard constraint status (merged from ScheduleSnapshot)
	HardPassed       bool `json:"hardPassed"`
	TeacherConflicts int  `json:"teacherConflicts"`
	RoomConflicts    int  `json:"roomConflicts"`
	ClassConflicts   int  `json:"classConflicts"`

	// Soft constraint scores (merged from ScheduleSnapshot)
	TotalScore     float64 `json:"totalScore"`
	FinalScore     float64 `json:"finalScore"`
	TeacherPref    float64 `json:"teacherPref"`
	CourseSpacing  float64 `json:"courseSpacing"`
	TeacherDays    float64 `json:"teacherDays"`
	LowFloorPref   float64 `json:"lowFloorPref"`
	WeekendAvoid   float64 `json:"weekendAvoid"`
	PePeriodPref   float64 `json:"pePeriodPref"`
	StudentFatigue float64 `json:"studentFatigue"`
	CapacityWarn   int     `json:"capacityWarn"`

	// Scoring configuration (merged from ScheduleSnapshot)
	EnabledConstraints   string  `gorm:"size:500" json:"enabledConstraints"`
	ScoreVersion         int     `json:"scoreVersion"`
	PerCategoryMax       float64 `json:"perCategoryMax"`
	EnabledCategoryCount int     `json:"enabledCategoryCount"`
	CategoryMaxes        string  `gorm:"type:text" json:"categoryMaxes,omitempty"`

	// Placement completeness (merged from ScheduleSnapshot)
	PlacedSessions   int     `gorm:"default:0" json:"placedSessions"`
	ExpectedSessions int     `gorm:"default:0" json:"expectedSessions"`
	Completeness     float64 `gorm:"default:0" json:"completeness"`

	// Statistics
	EntryCount  int   `json:"entryCount"`
	SolveTimeMs int64 `json:"solveTimeMs"`

	// Linked data
	Entries []ScheduleVersionEntry `gorm:"foreignKey:VersionID" json:"entries,omitempty"`
	Details []VersionDetail        `gorm:"foreignKey:VersionID" json:"details,omitempty"`

	// Deprecated: Score is kept for backward compatibility; use FinalScore instead.
	// New code should always read/write FinalScore.
	Score float64 `json:"score"`
}

// TableName overrides the default table name for ScheduleVersion.
func (ScheduleVersion) TableName() string { return "schedule_versions" }

// DisplayName returns the version's display name, falling back to auto-generated
// default if Name is empty. Pure function — does not mutate.
func (v *ScheduleVersion) DisplayName() string {
	if v.Name != "" {
		return v.Name
	}
	return DefaultVersionName(v.Source, v.CreatedAt)
}

// DefaultVersionName returns the auto-generated name for a version.
func DefaultVersionName(source string, t time.Time) string {
	return fmt.Sprintf("%s · %s", SourceLabel(source), t.Format("2006-01-02 15:04:05"))
}

// SourceLabel maps a version source code to its human-readable label.
func SourceLabel(source string) string {
	switch source {
	case VersionSourceAutoGenerate:
		return "自动排课"
	case VersionSourceManualAdjust:
		return "手动调整"
	case VersionSourceImport:
		return "导入"
	case VersionSourceRestore:
		return "恢复"
	case VersionSourceCopy:
		return "复制"
	default:
		return source
	}
}

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
	ClassroomID    *uint  `json:"classroomId,omitempty"`
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
