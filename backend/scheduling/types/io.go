package types

import "time"

// TimeSchedulingInput is the entire input contract for a TimeScheduler
// run. It is a pure value; TimeScheduler must not read any state outside
// this struct (INV-P9).
type TimeSchedulingInput struct {
	Tasks       []TeachingTaskView
	Teachers    []TeacherView
	ClassGroups []ClassGroupView

	LockedSlots    []LockedTimeSlot
	AvoidanceHints []ResourceConflictHint

	Deadline          time.Time
	Seed              int64
	Constraints       []string
	ConstraintWeights map[string]int
	SportsCourseIDs   map[uint]bool
	SemesterID        uint // filled onto Drafts by the Service layer, not by TimeScheduler
}

// TimeSchedulingOutput is TimeScheduler's return contract. Assignments
// carry no persistence identifiers (INV-P5). ScoreDetail covers three
// time-family buckets; Resource is computed by RoomScheduler when
// applicable.
type TimeSchedulingOutput struct {
	Assignments []TimeAssignmentDraft
	ScoreDetail TimeScoreDetail
	Diagnostics []string
	Iterations  int
	ElapsedMs   int64
}

// RoomSchedulingInput is the entire input contract for a RoomScheduler
// run. LocalRef inside each TimeAssignmentPlaced correlates output
// allocations and hints back to specific unpersisted TAs.
type RoomSchedulingInput struct {
	Assignments []TimeAssignmentPlaced
	Classrooms  []ClassroomView
	Tasks       []TeachingTaskView
	Deadline    time.Time
}

// RoomSchedulingOutput is RoomScheduler's return contract. Successful
// allocations reference LocalRef; failures are surfaced as
// ResourceConflictHint entries which the Orchestrator feeds back into
// the next TimeScheduler pass (up to MaxRetries).
type RoomSchedulingOutput struct {
	Allocations []RoomAllocationDraft
	Hints       []ResourceConflictHint
	ScoreDetail ResourceScoreDetail
	ElapsedMs   int64
}

// OrchestratorRequest is the full input to a scheduling run. The
// SchedulingService constructs one of these per RunScheduling call;
// downstream Solver components see only projections of it.
//
// Mode has exactly two consumption points inside Orchestrator.Run
// (INV-P4): RequiresRoomAssignment (assembly) and
// EnabledScoreDimensions (scoring).
type OrchestratorRequest struct {
	Mode SchedulingMode

	Tasks       []TeachingTaskView
	Teachers    []TeacherView
	ClassGroups []ClassGroupView
	Classrooms  []ClassroomView // may be empty/nil in TIME_ONLY

	LockedSlots       []LockedTimeSlot
	Constraints       []string
	ConstraintWeights map[string]int
	Deadline          time.Time
	Seed              int64
	MaxRetries        int
	SemesterID        uint
}

// OrchestratorResult is the summary returned to the Service layer. It
// does not carry ScheduleVersionID: version creation is the Service's
// responsibility inside the persistence transaction.
//
// Score reflects the state of the FINAL retry attempt only (INV-P12).
// Intermediate attempt scores are surfaced through ProgressReporter,
// not returned here.
type OrchestratorResult struct {
	Assignments []TimeAssignmentDraft
	Allocations []RoomAllocationDraft // nil in TIME_ONLY
	Score       ScoreBreakdown
	Logs        []string
	Diagnostics []string
	Attempts    int
	ElapsedMs   int64
}
