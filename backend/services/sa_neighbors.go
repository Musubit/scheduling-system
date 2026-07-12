package services

import (
	"scheduling-system/backend/models"
)

type neighborOp struct {
	kind        string // "move" or "swap"
	entryIdx    int    // index in ctx.entries for "move"
	swapIdx1    int    // first index for "swap"
	swapIdx2    int    // second index for "swap"
	oldDay      models.DayOfWeek
	oldStart    models.Period
	oldRoom     uint
	newDay      models.DayOfWeek
	newStart    models.Period
	newRoom     uint
	applied     bool
}

// tryNeighbor attempts a random neighbor move and returns the new score.
func (ctx *schedulingContext) tryNeighbor(currentScore float64) float64 {
	if len(ctx.entries) == 0 {
		return currentScore
	}

	// Reset last operation
	ctx.lastNeighbor = neighborOp{}

	// Choose operation: 70% move, 30% swap
	if ctx.rng.Float64() < 0.7 || len(ctx.entries) < 2 {
		return ctx.tryMove(currentScore)
	}
	return ctx.trySwap(currentScore)
}

// tryMove moves one schedule entry to a new (day, period, room).
// Teacher and teaching task (class groups) remain fixed — only time and room change.
// v0.5.1: span is preserved from the moved entry; new starts are drawn from
// ValidStartsForSpan(entry.Span) so the move keeps the session block-legal.
func (ctx *schedulingContext) tryMove(currentScore float64) float64 {
	idx := ctx.rng.Intn(len(ctx.entries))
	entry := ctx.entries[idx]

	// Save old state
	ctx.lastNeighbor.kind = "move"
	ctx.lastNeighbor.entryIdx = idx
	ctx.lastNeighbor.oldDay = entry.DayOfWeek
	ctx.lastNeighbor.oldStart = entry.StartPeriod
	ctx.lastNeighbor.oldRoom = entry.ClassroomID

	// Remove old occupancy
	ctx.removeOccupancy(entry)

	span := entry.Span
	if span < 1 {
		span = 2 // defensive: legacy rows may have 0
	}
	validStarts := toIntSlice(models.ValidStartsForSpan(span))
	if len(validStarts) == 0 {
		// No legal start for this span — abort the move, keep current placement.
		ctx.restoreOccupancy(entry)
		return currentScore
	}

	// Pick day: prefer non-weekend if avoidance is on (80% chance weekdays)
	day := ctx.rng.Intn(7)
	if ctx.hasConstraint("avoid_saturday") || ctx.hasConstraint("avoid_sunday") {
		if ctx.rng.Float64() < 0.8 {
			day = ctx.rng.Intn(5) // weekday only
		}
	}
	// Pick start period, with bias for sports courses.
	// PE preferred starts (2, 6) are only legal for span=2; when they're absent
	// from validStarts (e.g. span=3), the bias silently falls back to uniform.
	var start int
	if ctx.hasConstraint("pe_preferred_periods") && ctx.sportsCourseIDs[entry.CourseID] {
		peStarts := []int{}
		for _, s := range validStarts {
			if s == 2 || s == 6 {
				peStarts = append(peStarts, s)
			}
		}
		if len(peStarts) > 0 && ctx.rng.Float64() < 0.7 {
			start = peStarts[ctx.rng.Intn(len(peStarts))]
		} else {
			start = validStarts[ctx.rng.Intn(len(validStarts))]
		}
	} else {
		start = validStarts[ctx.rng.Intn(len(validStarts))]
	}

		// Check locked slots
		for _, ls := range ctx.lockedSlots {
			if int(ls.DayOfWeek) == day && periodsOverlapInt(start, span, int(ls.StartPeriod), ls.Span) {
				ctx.restoreOccupancy(entry)
				return currentScore
			}
		}

		// Check teacher unavailable slots
		if ctx.isTeacherUnavailable(entry.TeacherID, day, start, span) {
			ctx.restoreOccupancy(entry)
			return currentScore
		}

		// Check teacher preferences (soft — allow placement, let score decide)
	// Removed hard rejection: SA's Metropolis criterion handles preference via scoring

	// Check teacher busy at new position
	teacherBusy := false
	for p := start; p < start+span; p++ {
		key := occKey(day, p, entry.TeacherID)
		if ctx.teacherOcc[key] {
			teacherBusy = true
			break
		}
	}
	if teacherBusy {
		ctx.restoreOccupancy(entry)
		return currentScore
	}

	// Check class groups busy (from teaching task)
	classBusy := ctx.classGroupsBusy(entry, day, start, span)
	if classBusy {
		ctx.restoreOccupancy(entry)
		return currentScore
	}

	// Try rooms
	rooms := make([]models.Classroom, len(ctx.classrooms))
	copy(rooms, ctx.classrooms)
	ctx.rng.Shuffle(len(rooms), func(i, j int) { rooms[i], rooms[j] = rooms[j], rooms[i] })

	for _, room := range rooms {
		// v0.5.3: use ResourceMatcher for room type + equipment check
		if td := ctx.findTaskDataByEntry(entry); td != nil {
			if !Match(td.Task, td.Task.Course, room).OK {
				continue
			}
			// Check room capacity
			if !ctx.canRoomFitCapacity(room, td) {
				continue
			}
		}

		// Check room busy (skip for shared venues)
		if !IsSharedVenue(room) {
			roomBusy := false
			for p := start; p < start+span; p++ {
				key := occKey(day, p, room.ID)
				if ctx.roomOcc[key] {
					roomBusy = true
					break
				}
			}
			if roomBusy {
				continue
			}
		}

		// Apply move
		ctx.applyDelta(-1, ctx.entries[idx])
		ctx.entries[idx].DayOfWeek = models.DayOfWeek(day)
		ctx.entries[idx].StartPeriod = models.Period(start)
		ctx.entries[idx].ClassroomID = room.ID
		ctx.applyDelta(+1, ctx.entries[idx])

		ctx.lastNeighbor.newDay = models.DayOfWeek(day)
		ctx.lastNeighbor.newStart = models.Period(start)
		ctx.lastNeighbor.newRoom = room.ID
		ctx.lastNeighbor.applied = true

		ctx.addOccupancy(ctx.entries[idx])
		return ctx.computeScore()
	}

	// No valid move found, restore
	ctx.restoreOccupancy(entry)
	return currentScore
}

// trySwap swaps the time slots of two entries (keeping teaching task and room assignments).
func (ctx *schedulingContext) trySwap(currentScore float64) float64 {
	i1 := ctx.rng.Intn(len(ctx.entries))
	i2 := ctx.rng.Intn(len(ctx.entries))
	if i1 == i2 {
		return currentScore
	}

	e1, e2 := ctx.entries[i1], ctx.entries[i2]

	// Remove both from occupancy
	ctx.removeOccupancy(e1)
	ctx.removeOccupancy(e2)

	// Check e1 at e2's position
	conflict1 := ctx.checkPositionConflict(e1, int(e2.DayOfWeek), int(e2.StartPeriod))
	// Check e2 at e1's position
	conflict2 := ctx.checkPositionConflict(e2, int(e1.DayOfWeek), int(e1.StartPeriod))

	if conflict1 || conflict2 {
		ctx.addOccupancy(e1)
		ctx.addOccupancy(e2)
		return currentScore
	}

	// Save operation for undo
	ctx.lastNeighbor.kind = "swap"
	ctx.lastNeighbor.swapIdx1 = i1
	ctx.lastNeighbor.swapIdx2 = i2
	ctx.lastNeighbor.oldDay = e1.DayOfWeek
	ctx.lastNeighbor.oldStart = e1.StartPeriod
	ctx.lastNeighbor.oldRoom = e1.ClassroomID
	ctx.lastNeighbor.newDay = e2.DayOfWeek
	ctx.lastNeighbor.newStart = e2.StartPeriod
	ctx.lastNeighbor.newRoom = e2.ClassroomID

	// Swap day/period (keep teacher, room, teaching task)
	ctx.applyDelta(-1, ctx.entries[i1])
	ctx.applyDelta(-1, ctx.entries[i2])
	ctx.entries[i1].DayOfWeek, ctx.entries[i2].DayOfWeek = e2.DayOfWeek, e1.DayOfWeek
	ctx.entries[i1].StartPeriod, ctx.entries[i2].StartPeriod = e2.StartPeriod, e1.StartPeriod
	ctx.applyDelta(+1, ctx.entries[i1])
	ctx.applyDelta(+1, ctx.entries[i2])

	ctx.lastNeighbor.applied = true
	ctx.addOccupancy(ctx.entries[i1])
	ctx.addOccupancy(ctx.entries[i2])
	return ctx.computeScore()
}

// checkPositionConflict checks if an entry would conflict at a new (day, start) position.
func (ctx *schedulingContext) checkPositionConflict(e models.ScheduleEntry, day, start int) bool {
	span := e.Span

	// Check locked slots
	for _, ls := range ctx.lockedSlots {
		if int(ls.DayOfWeek) == day && periodsOverlapInt(start, span, int(ls.StartPeriod), ls.Span) {
			return true
		}
	}

	// Check teacher unavailable slots
	if ctx.isTeacherUnavailable(e.TeacherID, day, start, span) {
		return true
	}

	// Check teacher busy
	for p := start; p < start+span; p++ {
		key := occKey(day, p, e.TeacherID)
		if ctx.teacherOcc[key] {
			return true
		}
	}

	// Check room busy (skip for shared venues)
	isShared := false
	for _, room := range ctx.classrooms {
		if room.ID == e.ClassroomID && IsSharedVenue(room) {
			isShared = true
			break
		}
	}
	if !isShared {
		for p := start; p < start+span; p++ {
			key := occKey(day, p, e.ClassroomID)
			if ctx.roomOcc[key] {
				return true
			}
		}
	}

	// Check class groups busy
	return ctx.classGroupsBusy(e, day, start, span)
}

// classGroupsBusy checks if any class group in the entry's teaching task is occupied.
func (ctx *schedulingContext) classGroupsBusy(e models.ScheduleEntry, day, start, span int) bool {
	if e.TeachingTaskID == nil {
		return false
	}
	// Find the teaching task data
	for _, td := range ctx.teachingTasks {
		if td.Task.ID == *e.TeachingTaskID {
			for _, cid := range td.ClassIDs {
				for p := start; p < start+span; p++ {
					key := occKey(day, p, cid)
					if ctx.classOcc[key] {
						return true
					}
				}
			}
			return false
		}
	}
	return false
}

// undoNeighbor reverts the last neighbor operation.
func (ctx *schedulingContext) undoNeighbor() {
	if !ctx.lastNeighbor.applied {
		return
	}

	switch ctx.lastNeighbor.kind {
	case "move":
		idx := ctx.lastNeighbor.entryIdx
		ctx.removeOccupancy(ctx.entries[idx])
		ctx.applyDelta(-1, ctx.entries[idx])
		ctx.entries[idx].DayOfWeek = ctx.lastNeighbor.oldDay
		ctx.entries[idx].StartPeriod = ctx.lastNeighbor.oldStart
		ctx.entries[idx].ClassroomID = ctx.lastNeighbor.oldRoom
		ctx.applyDelta(+1, ctx.entries[idx])
		ctx.addOccupancy(ctx.entries[idx])

	case "swap":
		i1, i2 := ctx.lastNeighbor.swapIdx1, ctx.lastNeighbor.swapIdx2
		ctx.removeOccupancy(ctx.entries[i1])
		ctx.removeOccupancy(ctx.entries[i2])
		ctx.applyDelta(-1, ctx.entries[i1])
		ctx.applyDelta(-1, ctx.entries[i2])
		ctx.entries[i1].DayOfWeek = ctx.lastNeighbor.oldDay
		ctx.entries[i1].StartPeriod = ctx.lastNeighbor.oldStart
		ctx.entries[i1].ClassroomID = ctx.lastNeighbor.oldRoom
		ctx.entries[i2].DayOfWeek = ctx.lastNeighbor.newDay
		ctx.entries[i2].StartPeriod = ctx.lastNeighbor.newStart
		ctx.entries[i2].ClassroomID = ctx.lastNeighbor.newRoom
		ctx.applyDelta(+1, ctx.entries[i1])
		ctx.applyDelta(+1, ctx.entries[i2])
		ctx.addOccupancy(ctx.entries[i1])
		ctx.addOccupancy(ctx.entries[i2])
	}

	ctx.lastNeighbor.applied = false
}

// computeScore scores the current schedule using ScoringService.
// v0.5.2: SA optimizes FinalTotal (completeness-scaled).
// v0.5.2 Goal 3 delta: when the incremental cache is initialized, we use it
// (O(#teachers + #courses + #classGroups)) instead of ScoreSchedule (O(N_entries × 7)).
// Correctness parity is verified by TestDeltaScoreMatchesFullScore.
func (ctx *schedulingContext) computeScore() float64 {
	if len(ctx.entries) == 0 {
		return 0
	}
	if ctx.sCache != nil && ctx.enabledMap != nil {
		bd := ctx.sCache.scoreFromCache(ctx.enabledMap, ctx.sportsCourseIDs, ctx.expectedTotalSessions)
		return bd.FinalTotal
	}
	if ctx.cachedScorer == nil {
		ttList := make([]models.TeachingTask, len(ctx.teachingTasks))
		for i, td := range ctx.teachingTasks {
			ttList[i] = td.Task
		}
		scoringCtx := NewScoringContextWithExpected(ctx.constraints, ctx.sportsCourseIDs, ttList, ctx.expectedTotalSessions)
		return NewScoringService().ScoreSchedule(ctx.entries, ctx.teachers, ctx.classrooms, scoringCtx).FinalTotal
	}
	breakdown := ctx.cachedScorer.ScoreSchedule(ctx.entries, ctx.teachers, ctx.classrooms, ctx.cachedScoreCtx)
	return breakdown.FinalTotal
}

// applyDelta updates the score cache for an entry being added or removed.
// sign=+1 add, sign=-1 remove. Callers must pair these symmetrically around
// any temporary mutation so undoNeighbor can restore invariants exactly.
func (ctx *schedulingContext) applyDelta(sign int, e models.ScheduleEntry) {
	if ctx.sCache == nil {
		return
	}
	ctx.sCache.applyEntry(sign, e, ctx.sportsCourseIDs[e.CourseID])
}

// ---- Occupancy helpers ----

