package room_test

import (
	"context"
	"testing"

	"scheduling-system/backend/scheduling/room"
	"scheduling-system/backend/scheduling/types"
)

func TestGreedy_AssignsAllWhenRoomsSuffice(t *testing.T) {
	g := room.NewScheduler()
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
	g := room.NewScheduler()
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
	if len(out.Hints) != 1 || out.Hints[0].Reason != types.ReasonAllOccupied {
		t.Errorf("expected 1 AllOccupied hint, got %+v", out.Hints)
	}
}

func TestGreedy_TypeMismatchFromMatcher(t *testing.T) {
	g := room.NewScheduler()
	in := types.RoomSchedulingInput{
		Assignments: []types.TimeAssignmentPlaced{
			{LocalRef: 0, TeachingTaskID: 1, DayOfWeek: 0, StartPeriod: 0, Span: 2, TotalStudents: 30},
		},
		Classrooms: []types.ClassroomView{
			{ID: 100, Capacity: 60, Type: "STANDARD"},
		},
		// Task requires LAB room, but only STANDARD is available → matcher rejects
		Tasks: []types.TeachingTaskView{{ID: 1, TotalStudents: 30, RequiredRoomType: "LAB"}},
	}
	out, _ := g.Assign(context.Background(), in, types.NoopReporter{})
	if len(out.Allocations) != 0 {
		t.Errorf("expected no allocation when room type mismatches")
	}
	if len(out.Hints) != 1 || out.Hints[0].Reason != types.ReasonAllOccupied {
		t.Errorf("expected 1 AllOccupied hint, got %+v", out.Hints)
	}
}

func TestGreedy_ConflictOnSameSlotSameRoom(t *testing.T) {
	g := room.NewScheduler()
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
		t.Errorf("expected 1 successful allocation, got %d", len(out.Allocations))
	}
	if len(out.Hints) != 1 || out.Hints[0].Reason != types.ReasonAllOccupied {
		t.Errorf("expected 1 AllOccupied hint for second task, got %+v", out.Hints)
	}
}
