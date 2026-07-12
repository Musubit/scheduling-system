package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ScheduleSnapshot captures a point-in-time evaluation of a schedule.
// Generated automatically after scheduling or manually by user request.
type ScheduleSnapshot struct {
	gorm.Model
	Name     string `gorm:"size:100" json:"name"`     // 快照名称，创建时自动生成
	Semester string `gorm:"index;size:50" json:"semester"`
	Dept     string `gorm:"size:50" json:"dept"`     // 院系范围，空=全校
	Trigger  string `gorm:"size:20" json:"trigger"`   // "auto" | "manual"

	// Hard constraint pass/fail
	HardPassed       bool `json:"hardPassed"`
	TeacherConflicts int  `json:"teacherConflicts"`
	RoomConflicts    int  `json:"roomConflicts"`
	ClassConflicts   int  `json:"classConflicts"`
	LockedViolations int  `json:"lockedViolations"`

	// Soft constraint scores (0-100 scale, weighted)
	TotalScore     float64 `json:"totalScore"`
	TeacherPref    float64 `json:"teacherPref"`
	CourseSpacing  float64 `json:"courseSpacing"`
	TeacherDays    float64 `json:"teacherDays"`
	LowFloorPref   float64 `json:"lowFloorPref"`
	WeekendAvoid   float64 `json:"weekendAvoid"`
	PePeriodPref   float64 `json:"pePeriodPref"`   // 体育课时段偏好
	StudentFatigue float64 `json:"studentFatigue"` // 学生连续疲劳度
	CapacityWarn   int     `json:"capacityWarn"`   // 容量不足警告数

	// Scoring configuration stored at snapshot time for reproducible re-scoring
	EnabledConstraints string `gorm:"size:500" json:"enabledConstraints"` // JSON array
	ScoreVersion       int    `json:"scoreVersion"`

	// Statistics
	TotalEntries         int     `json:"totalEntries"`
	SolveTimeMs          int64   `json:"solveTimeMs"`
	Solver               string  `gorm:"size:30" json:"solver"`
	PerCategoryMax       float64 `json:"perCategoryMax"`
	EnabledCategoryCount int     `json:"enabledCategoryCount"`

	// Linked details
	Details []SnapshotDetail `gorm:"foreignKey:SnapshotID" json:"details,omitempty"`
}

// SnapshotDetail stores per-teacher/course score contributions for the report.
type SnapshotDetail struct {
	gorm.Model
	SnapshotID uint   `gorm:"index" json:"snapshotId"`
	EntityType string `gorm:"size:20" json:"entityType"` // "teacher" | "course"
	EntityCode string `gorm:"size:20" json:"entityCode"`
	EntityName string `gorm:"size:50" json:"entityName"`

	// Per-constraint breakdown
	EarlyPenalty    float64 `json:"earlyPenalty"`
	LatePenalty     float64 `json:"latePenalty"`
	DaysActual      int     `json:"daysActual"`
	DaysTarget      int     `json:"daysTarget"`
	AvgFloor        float64 `json:"avgFloor"`
	CapacityWarning bool    `json:"capacityWarning"`

	// Schedule summary
	EntryCount int    `json:"entryCount"`
	DaysCount  int    `json:"daysCount"`
	Summary    string `gorm:"size:500" json:"summary"` // e.g. "周一1-2节,周三3-4节"
}

// TableName overrides the default table name for ScheduleSnapshot.
func (ScheduleSnapshot) TableName() string {
	return "schedule_snapshots"
}

// TableName overrides the default table name for SnapshotDetail.
func (SnapshotDetail) TableName() string {
	return "snapshot_details"
}

// BeforeCreate sets the creation time.
func (s *ScheduleSnapshot) BeforeCreate(tx *gorm.DB) error {
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	return nil
}

// Trigger constants — centralized single source of truth.
// Extend here when adding new trigger types.
const (
	TriggerAuto    = "auto"
	TriggerManual  = "manual"
	TriggerImport  = "import"
	TriggerRestore = "restore"
	TriggerCopy    = "copy"
)

// TriggerLabel maps a trigger code to its human-readable label.
// Extensible for future trigger types — add new cases here.
func TriggerLabel(trigger string) string {
	switch trigger {
	case TriggerManual:
		return "手动生成"
	case TriggerAuto:
		return "自动排课"
	case TriggerImport:
		return "导入生成"
	case TriggerRestore:
		return "恢复生成"
	case TriggerCopy:
		return "复制生成"
	default:
		return trigger
	}
}

// DefaultSnapshotName returns the auto-generated name for a snapshot.
// Format: "{TriggerLabel} · yyyy-MM-dd HH:mm:ss"
func DefaultSnapshotName(trigger string, t time.Time) string {
	return fmt.Sprintf("%s · %s", TriggerLabel(trigger), t.Format("2006-01-02 15:04:05"))
}

// DisplayName returns the snapshot's display name, falling back to auto-generated
// default if Name is empty. Pure function — does not mutate.
func (s *ScheduleSnapshot) DisplayName() string {
	if s.Name != "" {
		return s.Name
	}
	return DefaultSnapshotName(s.Trigger, s.CreatedAt)
}
