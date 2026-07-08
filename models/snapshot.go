package models

import (
	"time"

	"gorm.io/gorm"
)

// ScheduleSnapshot captures a point-in-time evaluation of a schedule.
// Generated automatically after scheduling or manually by user request.
type ScheduleSnapshot struct {
	gorm.Model
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
	CapacityWarn   int     `json:"capacityWarn"` // 容量不足警告数

	// Statistics
	TotalEntries int    `json:"totalEntries"`
	SolveTimeMs  int64  `json:"solveTimeMs"`
	Solver       string `gorm:"size:30" json:"solver"` // "simulated_annealing"

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
