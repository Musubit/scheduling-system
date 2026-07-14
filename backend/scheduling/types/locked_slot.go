package types

// LockedTimeSlot is a time region that is forbidden for scheduling.
// It represents administrator-configured hard constraints (e.g., a
// campus-wide reserved period). This is a value-type copy inside the
// types package; the services layer holds its own LockedTimeSlot type
// with the same shape but distinct identity, and callers must copy
// across the boundary rather than share the underlying struct.
//
// Do not merge this with ResourceConflictHint (INV-H2). They differ in
// meaning: LockedTimeSlot is a persistent, admin-authored hard rule;
// ResourceConflictHint is a transient, solver-generated soft signal.
type LockedTimeSlot struct {
	DayOfWeek   DayOfWeek `json:"dayOfWeek"`
	StartPeriod Period    `json:"startPeriod"`
	Span        int       `json:"span"`
}
