package types

import (
	"context"
	"testing"
)

// TestNoopReporter_SatisfiesInterface is a compile-time check via
// interface assignment. If NoopReporter fails to implement
// ProgressReporter, this file does not compile.
func TestNoopReporter_SatisfiesInterface(t *testing.T) {
	var p ProgressReporter = NoopReporter{}
	// call each method to ensure they don't panic on zero-value receiver
	p.Stage("init", 0)
	p.Iteration(1, 100, 0.5, 0.6, 10.0)
	p.Log("hello")
}

// Fake implementations used only to prove the interfaces are shaped
// correctly. These are internal-only, no goroutines, no state.

type fakeTimeScheduler struct{}

func (fakeTimeScheduler) Solve(ctx context.Context, in TimeSchedulingInput, p ProgressReporter) (TimeSchedulingOutput, error) {
	return TimeSchedulingOutput{}, nil
}

type fakeRoomScheduler struct{}

func (fakeRoomScheduler) Assign(ctx context.Context, in RoomSchedulingInput, p ProgressReporter) (RoomSchedulingOutput, error) {
	return RoomSchedulingOutput{}, nil
}

type fakeScorer struct{}

func (fakeScorer) Score(
	assignments []TimeAssignmentDraft,
	allocations []RoomAllocationDraft,
	teachers []TeacherView,
	classrooms []ClassroomView,
	tasks []TeachingTaskView,
	dims []string,
) ScoreBreakdown {
	return ScoreBreakdown{}
}

type fakeOrchestrator struct{}

func (fakeOrchestrator) Run(ctx context.Context, req OrchestratorRequest, p ProgressReporter) (OrchestratorResult, error) {
	return OrchestratorResult{}, nil
}

func TestInterfaces_AreImplementable(t *testing.T) {
	// Compile-time assertions via interface assignment.
	var _ ITimeScheduler = fakeTimeScheduler{}
	var _ IRoomScheduler = fakeRoomScheduler{}
	var _ IScorer = fakeScorer{}
	var _ ISchedulingOrchestrator = fakeOrchestrator{}
	// If the test compiles and runs, all four interfaces have shapes
	// matching the fake implementations above.
}
