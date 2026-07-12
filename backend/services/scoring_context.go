package services

import (
	"encoding/json"
	"scheduling-system/backend/models"
)

// ScoringContext bundles all data needed for schedule quality evaluation.
// It is the single source of truth for scoring — no call site should derive
// constraints or sports course IDs independently.
//
// Version history:
//
//	v1 — initial version with 7 soft constraints (teacher_preference,
//	      course_dispersed, teacher_days_limit, low_floor_preference,
//	      avoid_saturday/avoid_sunday, pe_preferred_periods, student_fatigue)
//	v2 — v0.5.2: added ExpectedTotalSessions for placement-completeness scaling
//	      of FinalTotal. Total field semantics unchanged (Stable Core compat).
type ScoringContext struct {
	Version            int                     `json:"version"`
	EnabledConstraints []string                `json:"enabledConstraints"`
	SportsCourseIDs    map[uint]bool           `json:"-"` // runtime only
	TeachingTasks      []models.TeachingTask   `json:"-"` // runtime only

	// v0.5.2: expected weekly session count used by ScoreSchedule to compute
	// FinalTotal via completeness scaling. 0 = legacy path, no scaling.
	ExpectedTotalSessions int `json:"-"`
}

// NewScoringContext creates a scoring context for the current scheduling run.
func NewScoringContext(
	constraints []string,
	sportsIDs map[uint]bool,
	tasks []models.TeachingTask,
) ScoringContext {
	return NewScoringContextWithExpected(constraints, sportsIDs, tasks, 0)
}

// NewScoringContextWithExpected creates a v0.5.2 scoring context that also
// carries the total number of sessions expected to be placed. This drives
// FinalTotal via placement-completeness scaling. Passing 0 is legacy behavior
// (FinalTotal == Total).
func NewScoringContextWithExpected(
	constraints []string,
	sportsIDs map[uint]bool,
	tasks []models.TeachingTask,
	expectedTotalSessions int,
) ScoringContext {
	if constraints == nil {
		constraints = []string{}
	}
	if sportsIDs == nil {
		sportsIDs = make(map[uint]bool)
	}
	if tasks == nil {
		tasks = []models.TeachingTask{}
	}
	if expectedTotalSessions < 0 {
		expectedTotalSessions = 0
	}
	return ScoringContext{
		Version:               2,
		EnabledConstraints:    constraints,
		SportsCourseIDs:       sportsIDs,
		TeachingTasks:         tasks,
		ExpectedTotalSessions: expectedTotalSessions,
	}
}

// StoredConfig is the persisted subset of ScoringContext stored in a snapshot.
// Only EnabledConstraints and Version are persisted;
// SportsCourseIDs and TeachingTasks are rebuilt from entries when re-scoring.
type StoredConfig struct {
	Version            int      `json:"version"`
	EnabledConstraints []string `json:"enabledConstraints"`
}

// ToStoredConfig extracts the persistable subset.
func (ctx ScoringContext) ToStoredConfig() StoredConfig {
	return StoredConfig{
		Version:            ctx.Version,
		EnabledConstraints: ctx.EnabledConstraints,
	}
}

// MarshalStored serializes the persistable config to JSON.
func (ctx ScoringContext) MarshalStored() string {
	b, _ := json.Marshal(ctx.ToStoredConfig())
	return string(b)
}

// UnmarshalStoredConfig deserializes a StoredConfig from a JSON string.
func UnmarshalStoredConfig(raw string) (StoredConfig, error) {
	var cfg StoredConfig
	if raw == "" {
		return cfg, nil
	}
	err := json.Unmarshal([]byte(raw), &cfg)
	return cfg, err
}

// ScoringContextFromStored rebuilds a runtime ScoringContext from stored config
// plus rebuilt SportsCourseIDs and TeachingTasks.
func ScoringContextFromStored(
	stored StoredConfig,
	sportsIDs map[uint]bool,
	tasks []models.TeachingTask,
) ScoringContext {
	if stored.Version == 0 {
		stored.Version = 1
	}
	return ScoringContext{
		Version:            stored.Version,
		EnabledConstraints: stored.EnabledConstraints,
		SportsCourseIDs:    sportsIDs,
		TeachingTasks:      tasks,
	}
}

// FullDefaultConstraints returns all known constraint keys.
func FullDefaultConstraints() []string {
	return []string{
		"teacher_preference",
		"course_dispersed",
		"teacher_days_limit",
		"low_floor_preference",
		"pe_preferred_periods",
		"avoid_saturday",
		"avoid_sunday",
		"student_fatigue",
	}
}
