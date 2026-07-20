package time

// neighborOp 记录一次邻域操作，用于回滚。
type neighborOp struct {
	kind     string // "move" or "swap"
	idx      int    // move: entry index
	idx1     int    // swap: first index
	idx2     int    // swap: second index
	oldDay   int
	oldStart int
	newDay   int
	newStart int
	applied  bool
}

// tryNeighbor 随机尝试一次邻域移动。70% 改时间，30% 交换时间。
func (sctx *timeContext) tryNeighbor(currentScore float64) float64 {
	if len(sctx.entries) == 0 {
		return currentScore
	}
	sctx.lastNeighbor = neighborOp{}

	if sctx.rng.Float64() < 0.7 || len(sctx.entries) < 2 {
		return sctx.tryMoveTime(currentScore)
	}
	return sctx.trySwapTime(currentScore)
}

// tryMoveTime 随机选一个 entry，尝试换到新 (day, start) 位置。
func (sctx *timeContext) tryMoveTime(currentScore float64) float64 {
	idx := sctx.rng.Intn(len(sctx.entries))
	entry := sctx.entries[idx]

	// 保存旧状态
	sctx.lastNeighbor.kind = "move"
	sctx.lastNeighbor.idx = idx
	sctx.lastNeighbor.oldDay = entry.DayOfWeek
	sctx.lastNeighbor.oldStart = entry.StartPeriod

	// 移除旧占用
	sctx.removeTimeOccupancy(entry)

	span := entry.Span
	if span < 1 {
		span = 2
	}
	validStarts := validStartsForSpan(span)
	if len(validStarts) == 0 {
		sctx.addTimeOccupancy(entry)
		return currentScore
	}

	// 选日期: 有周末约束时 80% 选工作日
	day := sctx.rng.Intn(7)
	if sctx.hasConstraint("avoid_saturday") || sctx.hasConstraint("avoid_sunday") {
		if sctx.rng.Float64() < 0.8 {
			day = sctx.rng.Intn(5)
		}
	}

	// 选起始节次: PE 课偏好时段 2, 6
	var start int
	ti := sctx.taskByID[entry.TeachingTaskID]
	if ti != nil && ti.IsSports && sctx.hasConstraint("pe_preferred_periods") {
		peStarts := make([]int, 0)
		for _, s := range validStarts {
			if s == 2 || s == 6 {
				peStarts = append(peStarts, s)
			}
		}
		if len(peStarts) > 0 && sctx.rng.Float64() < 0.7 {
			start = peStarts[sctx.rng.Intn(len(peStarts))]
		} else {
			start = validStarts[sctx.rng.Intn(len(validStarts))]
		}
	} else {
		start = validStarts[sctx.rng.Intn(len(validStarts))]
	}

	// 检查新位置的冲突
	if sctx.hasTimeConflictAt(entry, day, start, span) {
		sctx.addTimeOccupancy(entry)
		return currentScore
	}

	// 应用移动（含增量评分更新）
	newScore := sctx.applyMove(idx, day, start)

	sctx.lastNeighbor.newDay = day
	sctx.lastNeighbor.newStart = start
	sctx.lastNeighbor.applied = true
	sctx.addTimeOccupancy(sctx.entries[idx])

	return newScore
}

// trySwapTime 随机选两个 entry，交换它们的 (day, start)。
func (sctx *timeContext) trySwapTime(currentScore float64) float64 {
	i1 := sctx.rng.Intn(len(sctx.entries))
	i2 := sctx.rng.Intn(len(sctx.entries))
	if i1 == i2 {
		return currentScore
	}

	e1, e2 := sctx.entries[i1], sctx.entries[i2]

	// 保存旧状态
	sctx.lastNeighbor.kind = "swap"
	sctx.lastNeighbor.idx1 = i1
	sctx.lastNeighbor.idx2 = i2
	sctx.lastNeighbor.oldDay = e1.DayOfWeek
	sctx.lastNeighbor.oldStart = e1.StartPeriod
	sctx.lastNeighbor.newDay = e2.DayOfWeek
	sctx.lastNeighbor.newStart = e2.StartPeriod

	// 移除占用
	sctx.removeTimeOccupancy(e1)
	sctx.removeTimeOccupancy(e2)

	// 检查 e1 换到 e2 的位置
	if sctx.hasTimeConflictAt(e1, e2.DayOfWeek, e2.StartPeriod, e1.Span) {
		sctx.addTimeOccupancy(e1)
		sctx.addTimeOccupancy(e2)
		return currentScore
	}
	// 检查 e2 换到 e1 的位置
	if sctx.hasTimeConflictAt(e2, e1.DayOfWeek, e1.StartPeriod, e2.Span) {
		sctx.addTimeOccupancy(e1)
		sctx.addTimeOccupancy(e2)
		return currentScore
	}

	// 应用交换
	newScore := sctx.applySwap(i1, i2, e2.DayOfWeek, e2.StartPeriod, e1.DayOfWeek, e1.StartPeriod)

	sctx.lastNeighbor.applied = true
	sctx.addTimeOccupancy(sctx.entries[i1])
	sctx.addTimeOccupancy(sctx.entries[i2])

	return newScore
}

// hasTimeConflictAt 检查 entry 在 (day, start, span) 是否有硬冲突。
func (sctx *timeContext) hasTimeConflictAt(e timeEntry, day, start, span int) bool {
	// 锁定槽位
	for _, ls := range sctx.lockedSlots {
		if int(ls.DayOfWeek) == day && periodsOverlap(start, span, int(ls.StartPeriod), ls.Span) {
			return true
		}
	}

	// 教师不可用
	if sctx.isTeacherUnavailable(e.TeacherID, day, start, span) {
		return true
	}

	// 教师占用
	for p := start; p < start+span; p++ {
		if sctx.teacherOcc[occKey(day, p, e.TeacherID)] {
			return true
		}
	}

	// 班级占用
	return sctx.classGroupsBusy(e, day, start, span)
}

// applyMove 执行 move 操作并更新评分缓存。
func (sctx *timeContext) applyMove(idx, newDay, newStart int) float64 {
	e := &sctx.entries[idx]

	// 增量更新: 先减旧状态
	sctx.sCache.applyDelta(-1, *e, sctx.sportsCourseIDs[e.CourseID])

	e.DayOfWeek = newDay
	e.StartPeriod = newStart

	// 再加新状态
	sctx.sCache.applyDelta(+1, *e, sctx.sportsCourseIDs[e.CourseID])

	return sctx.scoreFromCache()
}

// applySwap 执行 swap 操作并更新评分缓存。
func (sctx *timeContext) applySwap(i1, i2 int, day1, start1, day2, start2 int) float64 {
	e1, e2 := &sctx.entries[i1], &sctx.entries[i2]

	sctx.sCache.applyDelta(-1, *e1, sctx.sportsCourseIDs[e1.CourseID])
	sctx.sCache.applyDelta(-1, *e2, sctx.sportsCourseIDs[e2.CourseID])

	e1.DayOfWeek = day1
	e1.StartPeriod = start1
	e2.DayOfWeek = day2
	e2.StartPeriod = start2

	sctx.sCache.applyDelta(+1, *e1, sctx.sportsCourseIDs[e1.CourseID])
	sctx.sCache.applyDelta(+1, *e2, sctx.sportsCourseIDs[e2.CourseID])

	return sctx.scoreFromCache()
}

// undoNeighbor 回滚最近一次邻域操作。
func (sctx *timeContext) undoNeighbor() {
	if !sctx.lastNeighbor.applied {
		return
	}

	switch sctx.lastNeighbor.kind {
	case "move":
		idx := sctx.lastNeighbor.idx
		sctx.removeTimeOccupancy(sctx.entries[idx])
		sctx.sCache.applyDelta(-1, sctx.entries[idx], sctx.sportsCourseIDs[sctx.entries[idx].CourseID])
		sctx.entries[idx].DayOfWeek = sctx.lastNeighbor.oldDay
		sctx.entries[idx].StartPeriod = sctx.lastNeighbor.oldStart
		sctx.sCache.applyDelta(+1, sctx.entries[idx], sctx.sportsCourseIDs[sctx.entries[idx].CourseID])
		sctx.addTimeOccupancy(sctx.entries[idx])

	case "swap":
		i1, i2 := sctx.lastNeighbor.idx1, sctx.lastNeighbor.idx2
		sctx.removeTimeOccupancy(sctx.entries[i1])
		sctx.removeTimeOccupancy(sctx.entries[i2])
		sctx.sCache.applyDelta(-1, sctx.entries[i1], sctx.sportsCourseIDs[sctx.entries[i1].CourseID])
		sctx.sCache.applyDelta(-1, sctx.entries[i2], sctx.sportsCourseIDs[sctx.entries[i2].CourseID])
		sctx.entries[i1].DayOfWeek = sctx.lastNeighbor.oldDay
		sctx.entries[i1].StartPeriod = sctx.lastNeighbor.oldStart
		sctx.entries[i2].DayOfWeek = sctx.lastNeighbor.newDay
		sctx.entries[i2].StartPeriod = sctx.lastNeighbor.newStart
		sctx.sCache.applyDelta(+1, sctx.entries[i1], sctx.sportsCourseIDs[sctx.entries[i1].CourseID])
		sctx.sCache.applyDelta(+1, sctx.entries[i2], sctx.sportsCourseIDs[sctx.entries[i2].CourseID])
		sctx.addTimeOccupancy(sctx.entries[i1])
		sctx.addTimeOccupancy(sctx.entries[i2])
	}

	sctx.lastNeighbor.applied = false
}

// scoreFromCache 从增量缓存计算当前总分。
func (sctx *timeContext) scoreFromCache() float64 {
	if sctx.sCache == nil {
		return 0
	}
	bd := sctx.sCache.scoreDetail(sctx.enabledMap, sctx.sportsCourseIDs, sctx.expectedTotalSessions)
	return bd.finalTotal()
}
