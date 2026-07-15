package services

import (
	"encoding/json"

	"scheduling-system/backend/models"
	schedtypes "scheduling-system/backend/scheduling/types"
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
//	v3 — v0.5.5: added Mode so downstream scoring / snapshot / version writers
//	      can distinguish TIME_ONLY from FULL. Absent (v1/v2 stored configs)
//	      → treated as FULL_SCHEDULING for backward compatibility.
type ScoringContext struct {
	Version            int                     `json:"version"`
	EnabledConstraints []string                `json:"enabledConstraints"`
	Mode               schedtypes.SchedulingMode `json:"mode,omitempty"`
	SportsCourseIDs    map[uint]bool           `json:"-"` // runtime only
	TeachingTasks      []models.TeachingTask   `json:"-"` // runtime only

	// v0.5.2: expected weekly session count used by ScoreSchedule to compute
	// FinalTotal via completeness scaling. 0 = legacy path, no scaling.
	ExpectedTotalSessions int `json:"-"`

	// v0.5.6: per-constraint weights (0-100). When nil or empty, ScoreSchedule
	// falls back to equal weighting (100 / enabledCount per category).
	// Weights are persisted so snapshots/versions remember the configuration.
	ConstraintWeights map[string]int `json:"constraintWeights,omitempty"`
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
		Version:               3,
		EnabledConstraints:    constraints,
		Mode:                  schedtypes.ModeFullScheduling,
		SportsCourseIDs:       sportsIDs,
		TeachingTasks:         tasks,
		ExpectedTotalSessions: expectedTotalSessions,
	}
}

// WithMode sets Mode on the context and returns a copy — chainable at call sites
// (e.g., in RunScheduling after resolving mode from SchedulingConfig).
func (ctx ScoringContext) WithMode(mode schedtypes.SchedulingMode) ScoringContext {
	if !mode.IsValid() {
		mode = schedtypes.ModeFullScheduling
	}
	ctx.Mode = mode
	return ctx
}

// WithConstraintWeights sets per-constraint weights on the context and returns a copy.
// Chainable: ctx := NewScoringContext(...).WithConstraintWeights(weights).
// When weights is nil or empty, ScoreSchedule uses equal weighting.
func (ctx ScoringContext) WithConstraintWeights(weights map[string]int) ScoringContext {
	ctx.ConstraintWeights = weights
	return ctx
}

// EffectiveMode returns the mode with backward-compat default: empty → FULL.
// All read paths on scoring should use this rather than raw Mode.
func (ctx ScoringContext) EffectiveMode() schedtypes.SchedulingMode {
	if ctx.Mode.IsValid() {
		return ctx.Mode
	}
	return schedtypes.ModeFullScheduling
}

// StoredConfig is the persisted subset of ScoringContext stored in a snapshot.
// v3 adds Mode; v1/v2 rows omit it and read back as FULL_SCHEDULING.
// v4 adds ConstraintWeights; v1/v2/v3 rows omit it and read back as nil (equal weighting).
type StoredConfig struct {
	Version            int                       `json:"version"`
	EnabledConstraints []string                  `json:"enabledConstraints"`
	Mode               schedtypes.SchedulingMode `json:"mode,omitempty"`
	ConstraintWeights  map[string]int            `json:"constraintWeights,omitempty"`
}

// ToStoredConfig extracts the persistable subset.
func (ctx ScoringContext) ToStoredConfig() StoredConfig {
	return StoredConfig{
		Version:            ctx.Version,
		EnabledConstraints: ctx.EnabledConstraints,
		Mode:               ctx.EffectiveMode(),
		ConstraintWeights:  ctx.ConstraintWeights,
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
// plus rebuilt SportsCourseIDs and TeachingTasks. v1/v2 stored configs have
// no Mode field — treated as FULL_SCHEDULING.
func ScoringContextFromStored(
	stored StoredConfig,
	sportsIDs map[uint]bool,
	tasks []models.TeachingTask,
) ScoringContext {
	if stored.Version == 0 {
		stored.Version = 1
	}
	mode := stored.Mode
	if !mode.IsValid() {
		mode = schedtypes.ModeFullScheduling
	}
	return ScoringContext{
		Version:            stored.Version,
		EnabledConstraints: stored.EnabledConstraints,
		Mode:               mode,
		SportsCourseIDs:    sportsIDs,
		TeachingTasks:      tasks,
		ConstraintWeights:  stored.ConstraintWeights,
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
