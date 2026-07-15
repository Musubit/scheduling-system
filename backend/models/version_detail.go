package models

import "gorm.io/gorm"

// VersionDetail stores per-teacher score contributions for a schedule version.
// This is the unified replacement for SnapshotDetail, linked to ScheduleVersion
// instead of ScheduleSnapshot.
type VersionDetail struct {
	gorm.Model
	VersionID  uint   `gorm:"index;not null" json:"versionId"`
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

// TableName overrides the default table name for VersionDetail.
func (VersionDetail) TableName() string { return "version_details" }
