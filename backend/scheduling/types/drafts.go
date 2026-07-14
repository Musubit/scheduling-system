package types

// TimeAssignmentDraft is the Stage 1 output row — a scheduled weekly
// session that has NOT yet been persisted. Deliberately missing ID,
// ScheduleVersionID, and SemesterID (INV-P5): those are filled by the
// SchedulingService transaction, not by any Solver component.
type TimeAssignmentDraft struct {
	TeachingTaskID uint
	DayOfWeek      DayOfWeek
	StartPeriod    Period
	Span           int
}

// RoomAllocationDraft is the Stage 2 output — a room allocation for a
// particular TimeAssignment, referenced by LocalRef because the TA has
// not been persisted yet and has no real ID. The Orchestrator resolves
// LocalRef → real TA ID inside the persistence transaction.
type RoomAllocationDraft struct {
	LocalRef    int
	ClassroomID uint
}

// TimeAssignmentPlaced is the internal Stage 1 → Stage 2 transfer type.
// It repackages a TimeAssignmentDraft with the extra fields RoomScheduler
// needs (student count, required room type, allowed room IDs) so that
// RoomScheduler does not have to re-look-up the source TeachingTaskView.
// The LocalRef field lets RoomScheduler cite unpersisted TAs in its
// RoomAllocationDraft output and ResourceConflictHint output.
type TimeAssignmentPlaced struct {
	LocalRef         int
	TeachingTaskID   uint
	DayOfWeek        DayOfWeek
	StartPeriod      Period
	Span             int
	TotalStudents    int
	RequiredRoomType string
	AllowedRoomIDs   []uint
}
