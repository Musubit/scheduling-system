package room

import (
	"context"
	"sort"

	"scheduling-system/backend/models"
	schedtypes "scheduling-system/backend/scheduling/types"
)

// assignGreedy 执行贪心 first-fit 教室分配。
// 按约束最多优先排序，每个分配尝试第一个容量足够且无时间冲突的教室。
func assignGreedy(
	_ context.Context,
	input schedtypes.RoomSchedulingInput,
	_ schedtypes.ProgressReporter,
) (schedtypes.RoomSchedulingOutput, error) {
	// 1. 排序: AllowedRoomIDs 最少优先，同数量时学生多的优先
	sorted := make([]schedtypes.TimeAssignmentPlaced, len(input.Assignments))
	copy(sorted, input.Assignments)
	sort.Slice(sorted, func(i, j int) bool {
		li, lj := len(sorted[i].AllowedRoomIDs), len(sorted[j].AllowedRoomIDs)
		if li != lj {
			return li < lj
		}
		return sorted[i].TotalStudents > sorted[j].TotalStudents
	})

	// 2. 构建查找表
	roomMap := make(map[uint]schedtypes.ClassroomView, len(input.Classrooms))
	for _, r := range input.Classrooms {
		roomMap[r.ID] = r
	}
	taskMap := indexTasksByID(input.Tasks)

	// 3. 占用图
	type slotKey struct {
		RoomID uint
		Day    schedtypes.DayOfWeek
		Period schedtypes.Period
	}
	occupancy := make(map[slotKey]bool)

	allocations := make([]schedtypes.RoomAllocationDraft, 0, len(sorted))
	hints := make([]schedtypes.ResourceConflictHint, 0)

	// 4. 贪心分配
	m := roomMatcher{}
	for _, asgn := range sorted {
		// 确定候选教室: AllowedRoomIDs 优先，空时 fallback 到所有教室
		candidateIDs := asgn.AllowedRoomIDs
		if len(candidateIDs) == 0 {
			candidateIDs = make([]uint, 0, len(input.Classrooms))
			for _, r := range input.Classrooms {
				candidateIDs = append(candidateIDs, r.ID)
			}
		}

		placed := false
		for _, roomID := range candidateIDs {
			room, ok := roomMap[roomID]
			if !ok {
				continue
			}

			// 容量检查（GYM 跳过）
			if room.Type != "GYM" && room.Capacity < asgn.TotalStudents {
				continue
			}

			// 教室类型匹配（使用 ResourceMatcher）
			task := taskToModel(taskMap[asgn.TeachingTaskID])
			rm := roomViewToModel(room)
			mr := m.Match(task, models.Course{}, rm)
			if !mr.OK {
				continue
			}

			// 时间冲突检查
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

			// 分配成功
			for p := schedtypes.Period(0); p < schedtypes.Period(asgn.Span); p++ {
				occupancy[slotKey{RoomID: roomID, Day: asgn.DayOfWeek, Period: asgn.StartPeriod + p}] = true
			}
			allocations = append(allocations, schedtypes.RoomAllocationDraft{
				LocalRef:    asgn.LocalRef,
				ClassroomID: roomID,
			})
			placed = true
			break
		}

		if !placed {
			hints = append(hints, schedtypes.ResourceConflictHint{
				TeachingTaskID: asgn.TeachingTaskID,
				DayOfWeek:      asgn.DayOfWeek,
				StartPeriod:    asgn.StartPeriod,
				Span:           asgn.Span,
				Reason:         schedtypes.ReasonAllOccupied,
				Detail:         "all allowed rooms occupied or insufficient capacity",
			})
		}
	}

	return schedtypes.RoomSchedulingOutput{
		Allocations: allocations,
		Hints:       hints,
	}, nil
}

func indexTasksByID(tasks []schedtypes.TeachingTaskView) map[uint]schedtypes.TeachingTaskView {
	m := make(map[uint]schedtypes.TeachingTaskView, len(tasks))
	for _, t := range tasks {
		m[t.ID] = t
	}
	return m
}
