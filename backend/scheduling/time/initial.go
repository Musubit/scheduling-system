package time

import (
	"sort"
)

// buildInitial 用贪心算法为所有教学任务构造初始时间分配。
// 按"难度"排序（班级多、学时多的优先），然后逐 session 尝试放置。
func (sctx *timeContext) buildInitial() {
	sctx.entries = make([]timeEntry, 0, sctx.expectedTotalSessions)
	sctx.teacherOcc = make(map[uint64]bool)
	sctx.classOcc = make(map[uint64]bool)

	// 收集所有任务，按难度降序
	type taskWithSessions struct {
		taskID   uint
		sessions int // 需要放置的 session 数
	}
	taskList := make([]taskWithSessions, 0, len(sctx.taskByID))
	for tid, ti := range sctx.taskByID {
		plan := resolveSessionPlan(ti.CourseHours, ti.StartWeek, ti.EndWeek, ti.MaxPerWeek, ti.PreferredSpan)
		sessions := len(plan)
		if sessions > 0 {
			taskList = append(taskList, taskWithSessions{taskID: tid, sessions: sessions})
		}
	}
	sort.Slice(taskList, func(i, j int) bool {
		ti := sctx.taskByID[taskList[i].taskID]
		tj := sctx.taskByID[taskList[j].taskID]
		// 学生多优先
		if ti.TotalStudents != tj.TotalStudents {
			return ti.TotalStudents > tj.TotalStudents
		}
		return taskList[i].sessions > taskList[j].sessions
	})

	// 逐任务逐 session 放置
	for _, ts := range taskList {
		ti := sctx.taskByID[ts.taskID]
		span := ti.PreferredSpan
		if span < 2 {
			span = 2
		}

		for s := 0; s < ts.sessions; s++ {
			sctx.placeOneSession(ts.taskID, ti, span)
		}
	}
}

// placeOneSession 为教学任务的一个 session 寻找合法时间位置。
// 优先选: 非周末 > 教师偏好 > 分散课程日 > 低冲突。
func (sctx *timeContext) placeOneSession(taskID uint, ti *taskInfo, span int) {
	validStarts := validStartsForSpan(span)
	if len(validStarts) == 0 {
		return
	}

	// 生成所有可行 (day, start) 候选
	type candidate struct {
		day   int
		start int
		score float64 // 启发式评分, 越高越好
	}
	var candidates []candidate

	for day := 0; day < 7; day++ {
		for _, start := range validStarts {
			if sctx.isTimeConflict(taskID, ti, day, start, span) {
				continue
			}
			score := sctx.heuristicScore(taskID, ti, day, start, span)
			candidates = append(candidates, candidate{day, start, score})
		}
	}

	if len(candidates) == 0 {
		return // 无法放置此 session
	}

	// 选最高分候选
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.score > best.score {
			best = c
		}
	}

	// 创建 entry
	entry := timeEntry{
		TeachingTaskID: taskID,
		CourseID:       ti.CourseID,
		TeacherID:      ti.TeacherID,
		ClassGroupIDs:  ti.ClassGroupIDs,
		DayOfWeek:      best.day,
		StartPeriod:    best.start,
		Span:           span,
	}

	sctx.entries = append(sctx.entries, entry)
	sctx.addTimeOccupancy(entry)
}

// isTimeConflict 检查在 (day, start, span) 放置是否会产生硬冲突。
func (sctx *timeContext) isTimeConflict(_ uint, ti *taskInfo, day, start, span int) bool {
	// 检查锁定槽位
	for _, ls := range sctx.lockedSlots {
		if int(ls.DayOfWeek) == day && periodsOverlap(start, span, int(ls.StartPeriod), ls.Span) {
			return true
		}
	}

	// 检查教师不可用
	if sctx.isTeacherUnavailable(ti.TeacherID, day, start, span) {
		return true
	}

	// 检查教师占用
	for p := start; p < start+span; p++ {
		if sctx.teacherOcc[occKey(day, p, ti.TeacherID)] {
			return true
		}
	}

	// 检查班级占用
	for _, cgID := range ti.ClassGroupIDs {
		for p := start; p < start+span; p++ {
			if sctx.classOcc[occKey(day, p, cgID)] {
				return true
			}
		}
	}

	return false
}

// heuristicScore 计算候选位置的启发式评分（用于初始构造，非最终评分）。
func (sctx *timeContext) heuristicScore(taskID uint, ti *taskInfo, day, start, _ int) float64 {
	score := 1.0

	// 偏好非周末
	if day >= 5 {
		score -= 0.5
	}

	// 偏好非早晚（体育课除外）
	if ti.IsSports {
		if start == 2 || start == 6 {
			score += 0.3 // PE 偏好时段
		}
	} else {
		if ti.TeacherID > 0 {
			tv := sctx.teacherByID[ti.TeacherID]
			if tv != nil {
				if tv.PreferNoEarly && start <= 1 {
					score -= 0.2
				}
				if tv.PreferNoLate && start >= 6 {
					score -= 0.2
				}
			}
		}
	}

	// 偏好分散：同一 task 已有排课的 day 降权
	usedDays := make(map[int]bool)
	for _, e := range sctx.entries {
		if e.TeachingTaskID == taskID {
			usedDays[e.DayOfWeek] = true
		}
	}
	if usedDays[day] {
		score -= 0.3 // 同天聚集降权
	}

	return score
}

// addTimeOccupancy 标记 entry 的师生时间占用。
func (sctx *timeContext) addTimeOccupancy(e timeEntry) {
	for p := e.StartPeriod; p < e.StartPeriod+e.Span; p++ {
		sctx.teacherOcc[occKey(e.DayOfWeek, p, e.TeacherID)] = true
		for _, cgID := range e.ClassGroupIDs {
			sctx.classOcc[occKey(e.DayOfWeek, p, cgID)] = true
		}
	}
}

// removeTimeOccupancy 清除 entry 的师生时间占用。
func (sctx *timeContext) removeTimeOccupancy(e timeEntry) {
	for p := e.StartPeriod; p < e.StartPeriod+e.Span; p++ {
		delete(sctx.teacherOcc, occKey(e.DayOfWeek, p, e.TeacherID))
		for _, cgID := range e.ClassGroupIDs {
			delete(sctx.classOcc, occKey(e.DayOfWeek, p, cgID))
		}
	}
}

// isTeacherUnavailable 检查教师在 (day, start, span) 是否不可用。
func (sctx *timeContext) isTeacherUnavailable(teacherID uint, day, start, span int) bool {
	slots, ok := sctx.teacherUnavailable[teacherID]
	if !ok {
		return false
	}
	for _, u := range slots {
		if int(u.DayOfWeek) == day && periodsOverlap(start, span, int(u.StartPeriod), u.Span) {
			return true
		}
	}
	return false
}

// periodsOverlap 检查两个时段区间是否重叠。
func periodsOverlap(start1, span1, start2, span2 int) bool {
	end1 := start1 + span1
	end2 := start2 + span2
	return start1 < end2 && start2 < end1
}

// validStartsForSpan 返回给定 span 的所有合法起始节次。
// 时段合法性规则: 不跨午休(4-5节)，不超出第10节。
func validStartsForSpan(span int) []int {
	if span < 1 {
		span = 2
	}
	var out []int
	for start := 0; start <= 10; start++ {
		if isSpanLegal(start, span) {
			out = append(out, start)
		}
	}
	return out
}

// isSpanLegal 检查从 start 开始跨 span 节的时段是否合法。
// 规则: start+span <= 11, 且不跨越午休（不能 start<4 且 start+span>4，
// 也不能 start<5 且 start+span>5，即不能一半在上午一半在下午）。
func isSpanLegal(start, span int) bool {
	end := start + span
	if end > 11 {
		return false
	}
	// 不能跨越午休: start < 4 且 end > 4 (跨上午-午休)
	// 或 start < 5 且 end <= 4+? 实际上: 如果 start < 4 且 end > 4，说明包含了第4节
	// 简化: period 4 是午休开始，不能有课跨过它
	// Check: 如果 start < 4 且 end > 4，则跨上午->午休
	if start < 4 && end > 4 {
		return false
	}
	// Check: 如果 start == 4 (午休)，不行
	if start == 4 {
		return false
	}
	return true
}

// classGroupsBusy 检查 entry 关联的所有班级在指定时段是否有冲突。
func (sctx *timeContext) classGroupsBusy(e timeEntry, day, start, span int) bool {
	for _, cgID := range e.ClassGroupIDs {
		for p := start; p < start+span; p++ {
			if sctx.classOcc[occKey(day, p, cgID)] {
				return true
			}
		}
	}
	return false
}
