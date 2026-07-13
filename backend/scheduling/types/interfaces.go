package types

import "context"

// ProgressReporter is passed as an explicit parameter to every long-
// running Solver call (INV: no context.Value smuggling). Implementations
// route Stage/Iteration/Log to whatever surface the caller cares about
// (Wails logs, stdout, tests). Callers pass NoopReporter{} rather than
// nil when they do not care; implementations still defensively replace
// nil with NoopReporter{} at their entrypoints.
type ProgressReporter interface {
	// Stage announces a coarse pipeline stage transition. `percent` is
	// a rough 0..100 progress hint for UI.
	Stage(name string, percent int)

	// Iteration reports a fine-grained algorithmic step. Implementations
	// are expected to sample (e.g., every N iterations) to bound log
	// volume; that decision belongs to the impl, not to callers.
	Iteration(current, total int, currentScore, bestScore, temperature float64)

	// Log emits a free-form human-readable line.
	Log(message string)
}

// NoopReporter satisfies ProgressReporter with no side effects. It is
// safe to use as a zero-value.
type NoopReporter struct{}

func (NoopReporter) Stage(string, int)                             {}
func (NoopReporter) Iteration(int, int, float64, float64, float64) {}
func (NoopReporter) Log(string)                                    {}

// ITimeScheduler is Stage 1 of the two-stage pipeline. Implementations
// must be pure functions of their inputs (INV-P9): no DB, no
// filesystem, no ambient state.
type ITimeScheduler interface {
	Solve(ctx context.Context, input TimeSchedulingInput, progress ProgressReporter) (TimeSchedulingOutput, error)
}

// IRoomScheduler is Stage 2. Same purity requirement as ITimeScheduler.
type IRoomScheduler interface {
	Assign(ctx context.Context, input RoomSchedulingInput, progress ProgressReporter) (RoomSchedulingOutput, error)
}

// IScorer computes a ScoreBreakdown from a proposed schedule. It is the
// sole authority mapping `dims` to bucket nil-ness (INV-S2).
// allocations and classrooms may be nil in TIME_ONLY mode.
type IScorer interface {
	Score(
		assignments []TimeAssignmentDraft,
		allocations []RoomAllocationDraft,
		teachers []TeacherView,
		classrooms []ClassroomView,
		tasks []TeachingTaskView,
		dims []string,
	) ScoreBreakdown
}

// ISchedulingOrchestrator is the composition point over ITimeScheduler,
// IRoomScheduler, and IScorer. The Service layer holds one of these and
// delegates all algorithmic decisions to it. Run must be idempotent for
// the same input + seed (INV-P7).
type ISchedulingOrchestrator interface {
	Run(ctx context.Context, req OrchestratorRequest, progress ProgressReporter) (OrchestratorResult, error)
}
