package services

import (
	"context"
	"fmt"
	"sort"

	schedtypes "scheduling-system/backend/scheduling/types"
)

// LegacyRoomAllocatorAdapter implements schedtypes.IRoomScheduler with a simple
// greedy room allocator. This is a v0.6.0 temporary adapter — it will be
// deleted in v0.6.1 when the pure scheduling/room/ implementation replaces it.
type LegacyRoomAllocatorAdapter struct{}

// NewLegacyRoomAllocatorAdapter creates a new adapter instance.
func NewLegacyRoomAllocatorAdapter() *LegacyRoomAllocatorAdapter {
	return &LegacyRoomAllocatorAdapter{}
}

// Compile-time interface check.
var _ schedtypes.IRoomScheduler = (*LegacyRoomAllocatorAdapter)(nil)

// Assign implements schedtypes.IRoomScheduler using a greedy first-fit
// strategy. Assignments are sorted most-constrained-first; each is placed in
// the first allowed room that has sufficient capacity and no time conflict.
func (a *LegacyRoomAllocatorAdapter) Assign(
	ctx context.Context,
	input schedtypes.RoomSchedulingInput,
	progress schedtypes.ProgressReporter,
) (schedtypes.RoomSchedulingOutput, error) {
	// Defensively replace nil reporter with no-op.
	if progress == nil {
		progress = schedtypes.NoopReporter{}
	}

	progress.Stage("Greedy Room Allocation (legacy adapter)", 0)

	// 1. Sort assignments: most constrained first (fewest allowed rooms),
	//    tiebreak by TotalStudents descending.
	sorted := make([]schedtypes.TimeAssignmentPlaced, len(input.Assignments))
	copy(sorted, input.Assignments)
	sort.Slice(sorted, func(i, j int) bool {
		li, lj := len(sorted[i].AllowedRoomIDs), len(sorted[j].AllowedRoomIDs)
		if li != lj {
			return li < lj
		}
		return sorted[i].TotalStudents > sorted[j].TotalStudents
	})

	// 2. Build room-by-ID map.
	roomMap := make(map[uint]schedtypes.ClassroomView, len(input.Classrooms))
	for _, r := range input.Classrooms {
		roomMap[r.ID] = r
	}

	// 3. Build task-by-ID map (for hint detail generation).
	taskMap := make(map[uint]schedtypes.TeachingTaskView, len(input.Tasks))
	for _, t := range input.Tasks {
		taskMap[t.ID] = t
	}

	// 4. Room occupancy map: slotKey{RoomID, Day, Period} → occupied.
	type slotKey struct {
		RoomID uint
		Day    schedtypes.DayOfWeek
		Period schedtypes.Period
	}
	occupancy := make(map[slotKey]bool)

	allocations := make([]schedtypes.RoomAllocationDraft, 0, len(sorted))
	hints := make([]schedtypes.ResourceConflictHint, 0)

	// 5. Greedy assignment loop.
	for _, asgn := range sorted {
		placed := false
		for _, roomID := range asgn.AllowedRoomIDs {
			room, ok := roomMap[roomID]
			if !ok {
				continue
			}

			// Capacity check (skip for GYM — shared venue never conflicts on capacity).
			if room.Type != "GYM" && room.Capacity < asgn.TotalStudents {
				continue
			}

			// Time conflict check: any overlapping period already occupied?
			conflict := false
			for p := schedtypes.Period(0); p < schedtypes.Period(asgn.Span); p++ {
				key := slotKey{RoomID: roomID, Day: asgn.DayOfWeek, Period: asgn.StartPeriod + p}
				if occupancy[key] {
					conflict = true
					break
				}
			}
			if conflict {
				continue
			}

			// Found a suitable room — mark occupancy.
			for p := schedtypes.Period(0); p < schedtypes.Period(asgn.Span); p++ {
				key := slotKey{RoomID: roomID, Day: asgn.DayOfWeek, Period: asgn.StartPeriod + p}
				occupancy[key] = true
			}

			allocations = append(allocations, schedtypes.RoomAllocationDraft{
				LocalRef:    asgn.LocalRef,
				ClassroomID: roomID,
			})
			placed = true
			break
		}

		// If not placed, record a conflict hint.
		if !placed {
			task, _ := taskMap[asgn.TeachingTaskID]
			detail := fmt.Sprintf("Task #%d (%s): %d students, need %q room, %d allowed rooms — all occupied or insufficient capacity",
				asgn.TeachingTaskID, task.CourseName, asgn.TotalStudents, asgn.RequiredRoomType, len(asgn.AllowedRoomIDs))
			hints = append(hints, schedtypes.ResourceConflictHint{
				TeachingTaskID: asgn.TeachingTaskID,
				DayOfWeek:      asgn.DayOfWeek,
				StartPeriod:    asgn.StartPeriod,
				Span:           asgn.Span,
				Reason:         schedtypes.ReasonAllOccupied,
				Detail:         detail,
			})
		}
	}

	progress.Stage("Greedy Room Allocation complete", 100)

	return schedtypes.RoomSchedulingOutput{
		Allocations: allocations,
		Hints:       hints,
	}, nil
}
