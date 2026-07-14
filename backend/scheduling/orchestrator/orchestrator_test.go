package orchestrator_test

import (
	"context"
	"errors"
	"testing"

	"scheduling-system/backend/scheduling/orchestrator"
	"scheduling-system/backend/scheduling/types"
)

// --- Fakes ---

type fakeTime struct {
	out    types.TimeSchedulingOutput
	err    error
	called bool
}

func (f *fakeTime) Solve(ctx context.Context, in types.TimeSchedulingInput, p types.ProgressReporter) (types.TimeSchedulingOutput, error) {
	f.called = true
	return f.out, f.err
}

type fakeRoom struct {
	out    types.RoomSchedulingOutput
	err    error
	called bool
}

func (f *fakeRoom) Assign(ctx context.Context, in types.RoomSchedulingInput, p types.ProgressReporter) (types.RoomSchedulingOutput, error) {
	f.called = true
	return f.out, f.err
}

type fakeScorer struct {
	out    types.ScoreBreakdown
	called bool
}

func (f *fakeScorer) Score(assignments []types.TimeAssignmentDraft, allocations []types.RoomAllocationDraft, teachers []types.TeacherView, classrooms []types.ClassroomView, tasks []types.TeachingTaskView, dims []string) types.ScoreBreakdown {
	f.called = true
	return f.out
}

// --- Tests ---

type capture struct {
	dims []string
}

// TestOrchestrator_FullMode_HappyPath
func TestOrchestrator_FullMode_HappyPath(t *testing.T) {
	ft := &fakeTime{out: types.TimeSchedulingOutput{
		Assignments: []types.TimeAssignmentDraft{{TeachingTaskID: 1, DayOfWeek: 0, StartPeriod: 0, Span: 2}},
	}}
	fr := &fakeRoom{out: types.RoomSchedulingOutput{
		Allocations: []types.RoomAllocationDraft{{LocalRef: 0, ClassroomID: 100}},
	}}
	fs := &fakeScorer{out: types.ScoreBreakdown{Total: 80, FinalTotal: 80}}

	orc := orchestrator.New(ft, fr, fs)
	res, err := orc.Run(context.Background(), types.OrchestratorRequest{Mode: types.ModeFullScheduling}, types.NoopReporter{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !ft.called {
		t.Error("time scheduler not called")
	}
	if !fr.called {
		t.Error("room scheduler not called in FULL mode")
	}
	if !fs.called {
		t.Error("scorer not called")
	}
	if len(res.Assignments) != 1 || len(res.Allocations) != 1 {
		t.Errorf("want 1 assignment + 1 allocation, got %d + %d", len(res.Assignments), len(res.Allocations))
	}
}

// TestOrchestrator_TimeOnlyMode_SkipsRoom
func TestOrchestrator_TimeOnlyMode_SkipsRoom(t *testing.T) {
	ft := &fakeTime{out: types.TimeSchedulingOutput{
		Assignments: []types.TimeAssignmentDraft{{TeachingTaskID: 1}},
	}}
	fr := &fakeRoom{}
	fs := &fakeScorer{out: types.ScoreBreakdown{Total: 60}}

	orc := orchestrator.New(ft, fr, fs)
	res, err := orc.Run(context.Background(), types.OrchestratorRequest{Mode: types.ModeTimeOnlyScheduling}, types.NoopReporter{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !ft.called {
		t.Error("time not called")
	}
	if fr.called {
		t.Error("room scheduler SHOULD NOT be called in TIME_ONLY mode")
	}
	if len(res.Allocations) != 0 {
		t.Errorf("TIME_ONLY should produce zero allocations, got %d", len(res.Allocations))
	}
}

// TestOrchestrator_TimeFailure_PropagatesError
func TestOrchestrator_TimeFailure_PropagatesError(t *testing.T) {
	ft := &fakeTime{err: errors.New("time solver crashed")}
	fr := &fakeRoom{}
	fs := &fakeScorer{}

	orc := orchestrator.New(ft, fr, fs)
	_, err := orc.Run(context.Background(), types.OrchestratorRequest{Mode: types.ModeFullScheduling}, types.NoopReporter{})
	if err == nil {
		t.Fatal("want error from time solver")
	}
	if fr.called {
		t.Error("room should not be called when time fails")
	}
}

// TestOrchestrator_RoomFailureInFullMode_PropagatesError
func TestOrchestrator_RoomFailureInFullMode_PropagatesError(t *testing.T) {
	ft := &fakeTime{out: types.TimeSchedulingOutput{Assignments: []types.TimeAssignmentDraft{{TeachingTaskID: 1}}}}
	fr := &fakeRoom{err: errors.New("room solver crashed")}
	fs := &fakeScorer{}

	orc := orchestrator.New(ft, fr, fs)
	_, err := orc.Run(context.Background(), types.OrchestratorRequest{Mode: types.ModeFullScheduling}, types.NoopReporter{})
	if err == nil {
		t.Fatal("want error from room solver in FULL mode")
	}
}

// TestOrchestrator_InvalidMode_Errors
func TestOrchestrator_InvalidMode_Errors(t *testing.T) {
	orc := orchestrator.New(&fakeTime{}, &fakeRoom{}, &fakeScorer{})
	_, err := orc.Run(context.Background(), types.OrchestratorRequest{Mode: "UNKNOWN_MODE"}, types.NoopReporter{})
	if err == nil {
		t.Fatal("want error for invalid mode")
	}
}

// TestOrchestrator_ScorerDimensionsByMode
func TestOrchestrator_ScorerDimensionsByMode(t *testing.T) {
	makeScorer := func(cap *capture) types.IScorer {
		return &captureScorer{cap: cap}
	}
	// FULL
	capFull := &capture{}
	orc := orchestrator.New(&fakeTime{}, &fakeRoom{}, makeScorer(capFull))
	orc.Run(context.Background(), types.OrchestratorRequest{Mode: types.ModeFullScheduling}, types.NoopReporter{})
	if !containsAll(capFull.dims, []string{"time", "teacher", "student", "resource"}) {
		t.Errorf("FULL should enable all 4 dims, got %v", capFull.dims)
	}
	// TIME_ONLY
	capTime := &capture{}
	orc2 := orchestrator.New(&fakeTime{}, &fakeRoom{}, makeScorer(capTime))
	orc2.Run(context.Background(), types.OrchestratorRequest{Mode: types.ModeTimeOnlyScheduling}, types.NoopReporter{})
	if containsAny(capTime.dims, []string{"resource"}) {
		t.Errorf("TIME_ONLY should NOT include 'resource' dim, got %v", capTime.dims)
	}
}

type captureScorer struct{ cap *capture }

func (c *captureScorer) Score(a []types.TimeAssignmentDraft, al []types.RoomAllocationDraft, tr []types.TeacherView, cl []types.ClassroomView, ts []types.TeachingTaskView, dims []string) types.ScoreBreakdown {
	c.cap.dims = dims
	return types.ScoreBreakdown{}
}

func containsAll(hay, needles []string) bool {
	for _, n := range needles {
		found := false
		for _, h := range hay {
			if h == n {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func containsAny(hay, needles []string) bool {
	for _, n := range needles {
		for _, h := range hay {
			if h == n {
				return true
			}
		}
	}
	return false
}
