// Package room 提供教室分配的实现。
//
// Greedy 实现 types.IRoomScheduler，使用贪心 first-fit 策略：
// 按约束最多优先排序（AllowedRoomIDs 最少优先，学生数多的优先），
// 对每个时间分配依次尝试容量足够且无时间冲突的教室。
package room

import (
	"context"

	"scheduling-system/backend/models"
	"scheduling-system/backend/scheduling/matcher"
	schedtypes "scheduling-system/backend/scheduling/types"
)

// Scheduler 是贪心教室分配器，实现 IRoomScheduler。
type Scheduler struct{}

// NewScheduler 创建一个 Scheduler。
func NewScheduler() *Scheduler {
	return &Scheduler{}
}

// 编译期接口检查。
var _ schedtypes.IRoomScheduler = (*Scheduler)(nil)

// Assign 实现 types.IRoomScheduler。
func (s *Scheduler) Assign(
	ctx context.Context,
	input schedtypes.RoomSchedulingInput,
	progress schedtypes.ProgressReporter,
) (schedtypes.RoomSchedulingOutput, error) {
	if progress == nil {
		progress = schedtypes.NoopReporter{}
	}
	return assignGreedy(ctx, input, progress)
}

// taskToModel 将 TeachingTaskView 转为 matcher 所需的 models.TeachingTask。
func taskToModel(t schedtypes.TeachingTaskView) models.TeachingTask {
	return models.TeachingTask{
		TeacherID:        t.TeacherID,
		CourseID:         t.CourseID,
		RequiredRoomType: t.RequiredRoomType,
	}
}

// roomViewToModel 将 ClassroomView 转为 matcher 所需的 models.Classroom。
func roomViewToModel(v schedtypes.ClassroomView) models.Classroom {
	return models.Classroom{
		RoomType:  v.Type,
		Floor:     v.Floor,
		Capacity:  v.Capacity,
		Equipment: v.Equipment,
	}
}

// roomMatcher 是 matcher.Match 的薄封装。
type roomMatcher struct{}

func (roomMatcher) Match(task models.TeachingTask, course models.Course, cls models.Classroom) matcher.MatchResult {
	return matcher.Match(task, course, cls)
}
