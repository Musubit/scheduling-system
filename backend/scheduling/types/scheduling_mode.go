package types

// SchedulingMode is the sole source of mode state (I1, INV-M1). Two values
// are allowed; all other code must go through the methods on this type
// rather than switching on the raw string.
type SchedulingMode string

const (
	ModeFullScheduling     SchedulingMode = "FULL_SCHEDULING"
	ModeTimeOnlyScheduling SchedulingMode = "TIME_ONLY_SCHEDULING"
)

// IsValid reports whether m is one of the two allowed modes.
func (m SchedulingMode) IsValid() bool {
	switch m {
	case ModeFullScheduling, ModeTimeOnlyScheduling:
		return true
	}
	return false
}

// IsTimeOnly reports whether m suppresses the room-assignment stage.
func (m SchedulingMode) IsTimeOnly() bool {
	return m == ModeTimeOnlyScheduling
}

// RequiresRoomAssignment reports whether the pipeline must run the room
// scheduler. Orchestrator assembly decision point (INV-P4).
func (m SchedulingMode) RequiresRoomAssignment() bool {
	return m == ModeFullScheduling
}

// EnabledScoreDimensions returns the ordered list of score bucket keys
// active for this mode. The returned slice must be treated as read-only.
// TIME_ONLY excludes "resource"; FULL includes all four (INV-S2).
func (m SchedulingMode) EnabledScoreDimensions() []string {
	if m == ModeTimeOnlyScheduling {
		return []string{"time", "teacher", "student"}
	}
	return []string{"time", "teacher", "student", "resource"}
}

func SchedulingModeFromString(s string) SchedulingMode {
	switch s {
	case "FULL_SCHEDULING":
		return ModeFullScheduling
	case "TIME_ONLY_SCHEDULING":
		return ModeTimeOnlyScheduling
	default:
		return ModeFullScheduling
	}
}
