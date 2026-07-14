package room_test

import (
	"context"
	"testing"

	"scheduling-system/backend/models"
	"scheduling-system/backend/scheduling/matcher"
	"scheduling-system/backend/scheduling/room"
	"scheduling-system/backend/scheduling/types"
)

type fakeMatcher struct {
	deny map[uint]bool
}

func (f *fakeMatcher) Match(task models.TeachingTask, course models.Course, cls models.Classroom) matcher.MatchResult {
	if f.deny[cls.ID] {
		return matcher.MatchResult{OK: false, Code: matcher.CodeRoomTypeMismatch, Reason: "fake deny"}
	}
	return matcher.MatchResult{OK: true, Code: matcher.MatchOK}
}

func TestGreedy_AssignsAllWhenRoomsSuffice(t *testing.T) {
	g := room.NewGreedy(&fakeMatcher{})
	in := types.RoomSchedulingInput{
		Assignments: []types.TimeAssignmentPlaced{
			{LocalRef: 0, TeachingTaskID: 1, DayOfWeek: 0, StartPeriod: 0, Span: 2, TotalStudents: 30},
			{LocalRef: 1, TeachingTaskID: 2, DayOfWeek: 1, StartPeriod: 0, Span: 2, TotalStudents: 30},
		},
		Classrooms: []types.ClassroomView{
			{ID: 100, Capacity: 60, Type: "STANDARD"},
			{ID: 101, Capacity: 60, Type: "STANDARD"},
		},
		Tasks: []types.TeachingTaskView{
			{ID: 1, TotalStudents: 30},
			{ID: 2, TotalStudents: 30},
		},
	}
	out, err := g.Assign(context.Background(), in, types.NoopReporter{})
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if len(out.Allocations) != 2 {
		t.Errorf("want 2 allocations, got %d", len(out.Allocations))
	}
	if len(out.Hints) != 0 {
		t.Errorf("want 0 hints, got %d: %+v", len(out.Hints), out.Hints)
	}
}

func TestGreedy_CapacityOverflow(t *testing.T) {
	g := room.NewGreedy(&fakeMatcher{})
	in := types.RoomSchedulingInput{
		Assignments: []types.TimeAssignmentPlaced{
			{LocalRef: 0, TeachingTaskID: 1, DayOfWeek: 0, StartPeriod: 0, Span: 2, TotalStudents: 500},
		},
		Classrooms: []types.ClassroomView{
			{ID: 100, Capacity: 60, Type: "STANDARD"},
		},
		Tasks: []types.TeachingTaskView{{ID: 1, TotalStudents: 500}},
	}
	out, _ := g.Assign(context.Background(), in, types.NoopReporter{})
	if len(out.Allocations) != 0 {
		t.Errorf("expected no allocation for over-capacity task")
	}
	if len(out.Hints) != 1 || out.Hints[0].Reason != types.ReasonNoCapacity {
		t.Errorf("expected 1 NoCapacity hint, got %+v", out.Hints)
	}
}

func TestGreedy_TypeMismatchFromMatcher(t *testing.T) {
	m := &fakeMatcher{deny: map[uint]bool{100: true}}
	g := room.NewGreedy(m)
	in := types.RoomSchedulingInput{
		Assignments: []types.TimeAssignmentPlaced{
			{LocalRef: 0, TeachingTaskID: 1, DayOfWeek: 0, StartPeriod: 0, Span: 2, TotalStudents: 30},
		},
		Classrooms: []types.ClassroomView{
			{ID: 100, Capacity: 60, Type: "STANDARD"},
		},
		Tasks: []types.TeachingTaskView{{ID: 1, TotalStudents: 30}},
	}
	out, _ := g.Assign(context.Background(), in, types.NoopReporter{})
	if len(out.Allocations) != 0 {
		t.Errorf("expected no allocation when matcher denies all rooms")
	}
	if len(out.Hints) != 1 || out.Hints[0].Reason != types.ReasonNoMatchingType {
		t.Errorf("expected 1 NoMatchingType hint, got %+v", out.Hints)
	}
}

func TestGreedy_ConflictOnSameSlotSameRoom(t *testing.T) {
	g := room.NewGreedy(&fakeMatcher{})
	in := types.RoomSchedulingInput{
		Assignments: []types.TimeAssignmentPlaced{
			{LocalRef: 0, TeachingTaskID: 1, DayOfWeek: 0, StartPeriod: 0, Span: 2, TotalStudents: 30},
			{LocalRef: 1, TeachingTaskID: 2, DayOfWeek: 0, StartPeriod: 0, Span: 2, TotalStudents: 30},
		},
		Classrooms: []types.ClassroomView{
			{ID: 100, Capacity: 60, Type: "STANDARD"},
		},
		Tasks: []types.TeachingTaskView{{ID: 1, TotalStudents: 30}, {ID: 2, TotalStudents: 30}},
	}
	out, _ := g.Assign(context.Background(), in, types.NoopReporter{})
	if len(out.Allocations) != 1 {
		t.Errorf("expected 1 successful allocation (first-come-first-served), got %d", len(out.Allocations))
	}
	if len(out.Hints) != 1 || out.Hints[0].Reason != types.ReasonAllOccupied {
		t.Errorf("expected 1 AllOccupied hint for second task, got %+v", out.Hints)
	}
}