package room

import (
	"context"

	"scheduling-system/backend/models"
	"scheduling-system/backend/scheduling/types"
)

type Greedy struct {
	m Matcher
}

func NewGreedy(m Matcher) *Greedy { return &Greedy{m: m} }

func (g *Greedy) Assign(ctx context.Context, in types.RoomSchedulingInput, p types.ProgressReporter) (types.RoomSchedulingOutput, error) {
	occupancy := map[slotKey]bool{}
	taskByID := indexTasksByID(in.Tasks)

	var allocations []types.RoomAllocationDraft
	var hints []types.ResourceConflictHint

	for _, a := range in.Assignments {
		task := taskByID[a.TeachingTaskID]

		var candidates []types.ClassroomView
		for _, c := range in.Classrooms {
			if c.Capacity >= a.TotalStudents {
				candidates = append(candidates, c)
			}
		}
		if len(candidates) == 0 {
			hints = append(hints, hintFor(a, types.ReasonNoCapacity))
			continue
		}

		var matched []types.ClassroomView
		for _, c := range candidates {
			mr := g.m.Match(taskToModel(task), models.Course{}, roomViewToModel(c))
			if mr.OK {
				matched = append(matched, c)
			}
		}
		if len(matched) == 0 {
			hints = append(hints, hintFor(a, types.ReasonNoMatchingType))
			continue
		}

		var picked *types.ClassroomView
		for i := range matched {
			c := matched[i]
			if !isOccupied(occupancy, int(a.DayOfWeek), int(a.StartPeriod), a.Span, c.ID) {
				picked = &c
				break
			}
		}
		if picked == nil {
			hints = append(hints, hintFor(a, types.ReasonAllOccupied))
			continue
		}

		markOccupied(occupancy, int(a.DayOfWeek), int(a.StartPeriod), a.Span, picked.ID)
		allocations = append(allocations, types.RoomAllocationDraft{
			LocalRef:    a.LocalRef,
			ClassroomID: picked.ID,
		})
	}

	return types.RoomSchedulingOutput{
		Allocations: allocations,
		Hints:       hints,
	}, nil
}

type slotKey struct {
	Day, Period int
	RoomID      uint
}

func isOccupied(m map[slotKey]bool, day, startPeriod, span int, roomID uint) bool {
	for p := startPeriod; p < startPeriod+span; p++ {
		if m[slotKey{Day: day, Period: p, RoomID: roomID}] {
			return true
		}
	}
	return false
}

func markOccupied(m map[slotKey]bool, day, startPeriod, span int, roomID uint) {
	for p := startPeriod; p < startPeriod+span; p++ {
		m[slotKey{Day: day, Period: p, RoomID: roomID}] = true
	}
}

func hintFor(a types.TimeAssignmentPlaced, reason types.HintReason) types.ResourceConflictHint {
	return types.ResourceConflictHint{
		TeachingTaskID: a.TeachingTaskID,
		DayOfWeek:      a.DayOfWeek,
		StartPeriod:    a.StartPeriod,
		Span:           a.Span,
		Reason:         reason,
	}
}

func indexTasksByID(tasks []types.TeachingTaskView) map[uint]types.TeachingTaskView {
	m := make(map[uint]types.TeachingTaskView, len(tasks))
	for _, t := range tasks {
		m[t.ID] = t
	}
	return m
}

func taskToModel(t types.TeachingTaskView) models.TeachingTask {
	return models.TeachingTask{
		TeacherID:        t.TeacherID,
		CourseID:         t.CourseID,
		RequiredRoomType: t.RequiredRoomType,
	}
}

func roomViewToModel(v types.ClassroomView) models.Classroom {
	cls := models.Classroom{
		RoomType:  v.Type,
		Floor:     v.Floor,
		Capacity:  v.Capacity,
		Equipment: v.Equipment,
	}
	cls.ID = v.ID
	return cls
}