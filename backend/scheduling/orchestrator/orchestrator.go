// Package orchestrator 组合 time / room / scorer 三个 stage,
// 按 SchedulingMode 装配调度流程。
package orchestrator

import (
	"context"
	"errors"
	"time"

	"scheduling-system/backend/scheduling/types"
)

// Orchestrator 是主编排器。
type Orchestrator struct {
	timeSched types.ITimeScheduler
	roomSched types.IRoomScheduler
	scorer    types.IScorer
}

// New 构造 Orchestrator。
func New(time types.ITimeScheduler, room types.IRoomScheduler, scorer types.IScorer) *Orchestrator {
	return &Orchestrator{timeSched: time, roomSched: room, scorer: scorer}
}

// stageSuppressReporter 包装 ProgressReporter，静默子组件的 Stage 调用。
// 子组件（TimeScheduler/RoomScheduler）的内部阶段切换不应覆盖
// Orchestrator 层设定的进度百分比边界。
type stageSuppressReporter struct {
	inner types.ProgressReporter
}

func (r stageSuppressReporter) Stage(name string, percent int) {
	// 静默: 子组件的 Stage 不向上传播
}

func (r stageSuppressReporter) Iteration(cur, total int, cs, bs, temp float64) {
	r.inner.Iteration(cur, total, cs, bs, temp)
}

func (r stageSuppressReporter) Log(msg string) {
	r.inner.Log(msg)
}

// Run 实现 types.ISchedulingOrchestrator。
func (o *Orchestrator) Run(ctx context.Context, req types.OrchestratorRequest, p types.ProgressReporter) (types.OrchestratorResult, error) {
	if !req.Mode.IsValid() {
		return types.OrchestratorResult{}, errors.New("orchestrator: invalid mode " + string(req.Mode))
	}
	start := time.Now()

	// Stage 1: time scheduling — Orchestrator 掌握阶段切换，
	// 传给 TimeScheduler 的 reporter 静默 Stage（避免 "done"/100 覆盖进度）。
	p.Stage("时间排课", 0)
	timeIn := types.TimeSchedulingInput{
		Tasks:             req.Tasks,
		Teachers:          req.Teachers,
		ClassGroups:       req.ClassGroups,
		LockedSlots:       req.LockedSlots,
		Constraints:       req.Constraints,
		ConstraintWeights: req.ConstraintWeights,
		Seed:              req.Seed,
		Deadline:          req.Deadline,
		SemesterID:        req.SemesterID,
	}
	timeOut, err := o.timeSched.Solve(ctx, timeIn, stageSuppressReporter{p})
	if err != nil {
		return types.OrchestratorResult{}, err
	}

	// Stage 2: room scheduling（仅 FULL 模式）
	var allocations []types.RoomAllocationDraft
	if req.Mode.RequiresRoomAssignment() {
		p.Stage("教室分配", 50)
		roomIn := types.RoomSchedulingInput{
			Assignments: draftsToPlaced(timeOut.Assignments, req.Tasks),
			Classrooms:  req.Classrooms,
			Tasks:       req.Tasks,
			Deadline:    req.Deadline,
		}
		roomOut, err := o.roomSched.Assign(ctx, roomIn, stageSuppressReporter{p})
		if err != nil {
			return types.OrchestratorResult{}, err
		}
		allocations = roomOut.Allocations
	}

	// Stage 3: score
	p.Stage("评分计算", 90)
	dims := req.Mode.EnabledScoreDimensions()
	scoreBd := o.scorer.Score(timeOut.Assignments, allocations, req.Teachers, req.Classrooms, req.Tasks, dims)

	return types.OrchestratorResult{
		Assignments: timeOut.Assignments,
		Allocations: allocations,
		Score:       scoreBd,
		ElapsedMs:   time.Since(start).Milliseconds(),
	}, nil
}

// draftsToPlaced 把 TimeAssignmentDraft 转成含更多上下文的 TimeAssignmentPlaced。
func draftsToPlaced(drafts []types.TimeAssignmentDraft, tasks []types.TeachingTaskView) []types.TimeAssignmentPlaced {
	taskByID := make(map[uint]types.TeachingTaskView, len(tasks))
	for _, t := range tasks {
		taskByID[t.ID] = t
	}
	out := make([]types.TimeAssignmentPlaced, 0, len(drafts))
	for i, d := range drafts {
		t := taskByID[d.TeachingTaskID]
		out = append(out, types.TimeAssignmentPlaced{
			LocalRef:         i,
			TeachingTaskID:   d.TeachingTaskID,
			DayOfWeek:        d.DayOfWeek,
			StartPeriod:      d.StartPeriod,
			Span:             d.Span,
			TotalStudents:    t.TotalStudents,
			RequiredRoomType: t.RequiredRoomType,
			AllowedRoomIDs:   t.AllowedRoomIDs,
		})
	}
	return out
}
