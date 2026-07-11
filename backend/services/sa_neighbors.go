package services

import (
	"fmt"
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

var lastNeighbor neighborOp

// tryNeighbor attempts a random neighbor move and returns the new score.
func (ctx *schedulingContext) tryNeighbor(currentScore float64) float64 {
	if len(ctx.entries) == 0 {
		return currentScore
	}

	// Reset last operation
	lastNeighbor = neighborOp{}

	// Choose operation: 70% move, 30% swap
	if ctx.rng.Float64() < 0.7 || len(ctx.entries) < 2 {
		return ctx.tryMove(currentScore)
	}
	return ctx.trySwap(currentScore)
}

// tryMove moves one schedule entry to a new (day, period, room).
// Teacher and teaching task (class groups) remain fixed — only time and room change.
func (ctx *schedulingContext) tryMove(currentScore float64) float64 {
	idx := ctx.rng.Intn(len(ctx.entries))
	entry := ctx.entries[idx]

	// Save old state
	lastNeighbor.kind = "move"
	lastNeighbor.entryIdx = idx
	lastNeighbor.oldDay = entry.DayOfWeek
	lastNeighbor.oldStart = entry.StartPeriod
	lastNeighbor.oldRoom = entry.ClassroomID

	// Remove old occupancy
	ctx.removeOccupancy(entry)

	// Try new assignments
	validStarts := []int{0, 2, 4, 6, 8}
	// Pick day: prefer non-weekend if avoidance is on (80% chance weekdays)
	day := ctx.rng.Intn(7)
	if ctx.hasConstraint("avoid_saturday") || ctx.hasConstraint("avoid_sunday") {
		if ctx.rng.Float64() < 0.8 {
			day = ctx.rng.Intn(5) // weekday only
		}
	}
	// Pick start period, with bias for sports courses
	var start int
	if ctx.hasConstraint("pe_preferred_periods") && ctx.sportsCourseIDs[entry.CourseID] {
		// 70% chance to pick preferred periods (2 or 6), 30% any
		if ctx.rng.Float64() < 0.7 {
			prefer := []int{2, 6}
			start = prefer[ctx.rng.Intn(len(prefer))]
		} else {
			start = validStarts[ctx.rng.Intn(len(validStarts))]
		}
	} else {
		start = validStarts[ctx.rng.Intn(len(validStarts))]
	}
	span := entry.Span

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
		key := fmt.Sprintf("%d-%d-%d", day, p, entry.TeacherID)
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
		// Check room type
		if td := ctx.findTaskDataByEntry(entry); td != nil {
			requiredRoomType := ctx.getRequiredRoomType(td.Task.Course.Name)
			if requiredRoomType != "" && room.Type != requiredRoomType {
				continue
			}
		}
		// Check room capacity
		if td := ctx.findTaskDataByEntry(entry); td != nil && !ctx.canRoomFitCapacity(room, td) {
			continue
		}

		roomBusy := false
		for p := start; p < start+span; p++ {
			key := fmt.Sprintf("%d-%d-%d", day, p, room.ID)
			if ctx.roomOcc[key] {
				roomBusy = true
				break
			}
		}
		if roomBusy {
			continue
		}

		// Apply move
		ctx.entries[idx].DayOfWeek = models.DayOfWeek(day)
		ctx.entries[idx].StartPeriod = models.Period(start)
		ctx.entries[idx].ClassroomID = room.ID

		lastNeighbor.newDay = models.DayOfWeek(day)
		lastNeighbor.newStart = models.Period(start)
		lastNeighbor.newRoom = room.ID
		lastNeighbor.applied = true

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
	lastNeighbor.kind = "swap"
	lastNeighbor.swapIdx1 = i1
	lastNeighbor.swapIdx2 = i2
	lastNeighbor.oldDay = e1.DayOfWeek
	lastNeighbor.oldStart = e1.StartPeriod
	lastNeighbor.oldRoom = e1.ClassroomID
	lastNeighbor.newDay = e2.DayOfWeek
	lastNeighbor.newStart = e2.StartPeriod
	lastNeighbor.newRoom = e2.ClassroomID

	// Swap day/period (keep teacher, room, teaching task)
	ctx.entries[i1].DayOfWeek, ctx.entries[i2].DayOfWeek = e2.DayOfWeek, e1.DayOfWeek
	ctx.entries[i1].StartPeriod, ctx.entries[i2].StartPeriod = e2.StartPeriod, e1.StartPeriod

	lastNeighbor.applied = true
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
		key := fmt.Sprintf("%d-%d-%d", day, p, e.TeacherID)
		if ctx.teacherOcc[key] {
			return true
		}
	}

	// Check room busy
	for p := start; p < start+span; p++ {
		key := fmt.Sprintf("%d-%d-%d", day, p, e.ClassroomID)
		if ctx.roomOcc[key] {
			return true
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
					key := fmt.Sprintf("%d-%d-%d", day, p, cid)
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
	if !lastNeighbor.applied {
		return
	}

	switch lastNeighbor.kind {
	case "move":
		idx := lastNeighbor.entryIdx
		ctx.removeOccupancy(ctx.entries[idx])
		ctx.entries[idx].DayOfWeek = lastNeighbor.oldDay
		ctx.entries[idx].StartPeriod = lastNeighbor.oldStart
		ctx.entries[idx].ClassroomID = lastNeighbor.oldRoom
		ctx.addOccupancy(ctx.entries[idx])

	case "swap":
		i1, i2 := lastNeighbor.swapIdx1, lastNeighbor.swapIdx2
		ctx.removeOccupancy(ctx.entries[i1])
		ctx.removeOccupancy(ctx.entries[i2])
		ctx.entries[i1].DayOfWeek = lastNeighbor.oldDay
		ctx.entries[i1].StartPeriod = lastNeighbor.oldStart
		ctx.entries[i1].ClassroomID = lastNeighbor.oldRoom
		ctx.entries[i2].DayOfWeek = lastNeighbor.newDay
		ctx.entries[i2].StartPeriod = lastNeighbor.newStart
		ctx.entries[i2].ClassroomID = lastNeighbor.newRoom
		ctx.addOccupancy(ctx.entries[i1])
		ctx.addOccupancy(ctx.entries[i2])
	}

	lastNeighbor.applied = false
}

// computeScore scores the current schedule using ScoringService.
func (ctx *schedulingContext) computeScore() float64 {
	if len(ctx.entries) == 0 {
		return 0
	}
	// Extract teaching tasks for fatigue scoring
	ttList := make([]models.TeachingTask, len(ctx.teachingTasks))
	for i, td := range ctx.teachingTasks {
		ttList[i] = td.Task
	}
	breakdown := (&ScoringService{}).ScoreSchedule(ctx.entries, ctx.teachers, ctx.classrooms, ctx.constraints, ctx.sportsCourseIDs, ttList)
	return breakdown.Total
}

// ---- Occupancy helpers ----

