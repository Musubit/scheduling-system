package services

import (
	"fmt"
	"scheduling-system/backend/models"
)

func (ctx *schedulingContext) buildInitial() {
	ctx.entries = nil
	ctx.roomOcc = make(map[uint64]bool)
	ctx.teacherOcc = make(map[uint64]bool)
	ctx.classOcc = make(map[uint64]bool)

	// v0.5.1: valid starts are now derived per-span via IsSpanLegal.

	// Shuffle teaching tasks for randomness
	taskOrder := make([]int, len(ctx.teachingTasks))
	for i := range taskOrder {
		taskOrder[i] = i
	}
	ctx.rng.Shuffle(len(taskOrder), func(i, j int) { taskOrder[i], taskOrder[j] = taskOrder[j], taskOrder[i] })

	for _, ti := range taskOrder {
		td := ctx.teachingTasks[ti]

		// v0.5.1: derive per-session spans from course hours + preferred span.
		plan := resolveSessionPlan(
			td.CourseHours,
			td.Task.StartWeek,
			td.Task.EndWeek,
			td.Task.MaxHoursPerWeek,
			td.Task.PreferredSpan,
		)

		// Determine required room type for this task
		requiredRoomType := ""
		courseName := td.Task.Course.Name
		if models.IsSportsCourse(courseName) {
			requiredRoomType = "体育馆"
		} else if IsLabCourse(courseName) {
			requiredRoomType = "实验室"
		} else if IsComputerCourse(courseName) {
			requiredRoomType = "机房"
		}

		for _, sessionSpan := range plan.Spans {
			// Legal starts depend on this session's span (block-alignment rules).
			validStarts := toIntSlice(models.ValidStartsForSpan(sessionSpan))
			if len(validStarts) == 0 {
				continue
			}

			placed := false

			// Try days in random order, but push weekends to end if avoidance is on
			baseDays := ctx.rng.Perm(7)
			var days []int
			if ctx.hasConstraint("avoid_saturday") || ctx.hasConstraint("avoid_sunday") {
				prefer, avoid := []int{}, []int{}
				for _, d := range baseDays {
					isAvoid := false
					if ctx.hasConstraint("avoid_saturday") && d == int(models.Sat) {
						isAvoid = true
					}
					if ctx.hasConstraint("avoid_sunday") && d == int(models.Sun) {
						isAvoid = true
					}
					if isAvoid {
						avoid = append(avoid, d)
					} else {
						prefer = append(prefer, d)
					}
				}
				ctx.rng.Shuffle(len(prefer), func(i, j int) { prefer[i], prefer[j] = prefer[j], prefer[i] })
				days = append(prefer, avoid...)
			} else {
				days = baseDays
			}

			// Phase 1: Try with teacher preference enforced (soft — skip conflicting starts)
			placed = ctx.tryPlaceSession(td, days, validStarts, sessionSpan, requiredRoomType, true)

			// Phase 2: Relax teacher preference if not placed
			if !placed && ctx.hasConstraint("teacher_preference") {
				placed = ctx.tryPlaceSession(td, days, validStarts, sessionSpan, requiredRoomType, false)
			}

			_ = placed
		} // end session loop
	}
}

// toIntSlice converts a []models.Period to []int for internal solver loops.
func toIntSlice(ps []models.Period) []int {
	out := make([]int, len(ps))
	for i, p := range ps {
		out[i] = int(p)
	}
	return out
}

// tryPlaceSession attempts to place one session of a teaching task.
// If enforcePref is true, teacher preference conflicts cause the start to be skipped.
// If false, teacher preferences are ignored (relaxed phase).
// v0.5.1: span is now a per-session parameter, no longer hardcoded to 2.
func (ctx *schedulingContext) tryPlaceSession(td teachingTaskData, days []int, starts []int, span int, requiredRoomType string, enforcePref bool) bool {
	for _, day := range days {
		// Shuffle starts for randomness, but sports courses prefer specific starts
		var orderedStarts []int
		if ctx.hasConstraint("pe_preferred_periods") && ctx.sportsCourseIDs[td.Task.CourseID] {
			// Sports preferred starts are 2 and 6 (both legal only for span=2).
			// If this session's span disallows them, fall through to shuffle.
			prefer := []int{}
			other := []int{}
			for _, s := range starts {
				if s == 2 || s == 6 {
					prefer = append(prefer, s)
				} else {
					other = append(other, s)
				}
			}
			ctx.rng.Shuffle(len(prefer), func(i, j int) { prefer[i], prefer[j] = prefer[j], prefer[i] })
			ctx.rng.Shuffle(len(other), func(i, j int) { other[i], other[j] = other[j], other[i] })
			orderedStarts = append(prefer, other...)
		} else {
			orderedStarts = make([]int, len(starts))
			copy(orderedStarts, starts)
			ctx.rng.Shuffle(len(orderedStarts), func(i, j int) { orderedStarts[i], orderedStarts[j] = orderedStarts[j], orderedStarts[i] })
		}

		for _, start := range orderedStarts {
			// Check locked slots
			locked := false
			for _, ls := range ctx.lockedSlots {
				if int(ls.DayOfWeek) == day && periodsOverlapInt(start, span, int(ls.StartPeriod), ls.Span) {
					locked = true
					break
				}
			}
			if locked {
				continue
			}

			// Check teacher unavailable slots (always hard)
			if ctx.isTeacherUnavailable(td.Task.TeacherID, day, start, span) {
				continue
			}

			// Check teacher preferences (soft in relaxed mode)
			if enforcePref && ctx.hasConstraint("teacher_preference") {
				prefConflict := false
				for _, t := range ctx.teachers {
					if t.ID == td.Task.TeacherID {
						if t.PreferNoEarly && start <= 1 {
							prefConflict = true
						}
						if t.PreferNoLate && start >= 6 {
							prefConflict = true
						}
						break
					}
				}
				if prefConflict {
					continue
				}
			}

			// Check teacher busy
			teacherBusy := false
			for p := start; p < start+span; p++ {
				key := occKey(day, p, td.Task.TeacherID)
				if ctx.teacherOcc[key] {
					teacherBusy = true
					break
				}
			}
			if teacherBusy {
				continue
			}

			// Check not overlapping with already-placed sessions of same task
			taskSelfBusy := false
			for _, existing := range ctx.entries {
				if existing.TeachingTaskID != nil && *existing.TeachingTaskID == td.Task.ID {
					if int(existing.DayOfWeek) == day && periodsOverlapInt(start, span, int(existing.StartPeriod), existing.Span) {
						taskSelfBusy = true
						break
					}
				}
			}
			if taskSelfBusy {
				continue
			}

			// Check all class groups are free
			classBusy := false
			for _, cid := range td.ClassIDs {
				for p := start; p < start+span; p++ {
					key := occKey(day, p, cid)
					if ctx.classOcc[key] {
						classBusy = true
						break
					}
				}
				if classBusy {
					break
				}
			}
			if classBusy {
				continue
			}

			// Try rooms (with room type filtering)
			rooms := make([]models.Classroom, len(ctx.classrooms))
			copy(rooms, ctx.classrooms)
			ctx.rng.Shuffle(len(rooms), func(i, j int) { rooms[i], rooms[j] = rooms[j], rooms[i] })

			for _, room := range rooms {
			// Check room type
			if requiredRoomType != "" {
				if room.Type != requiredRoomType {
					continue
				}
			} else if room.Type == "体育馆" || room.Type == "实验室" || room.Type == "机房" {
				continue // regular courses cannot use specialty rooms
			}
				// Check room capacity
				if !ctx.canRoomFitCapacity(room, &td) {
					continue
				}

			// Check room conflict (skip for shared venues like 体育馆)
			if room.Type != "体育馆" {
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

				// All constraints satisfied, create entry
				entry := models.ScheduleEntry{
					CourseID:       td.Task.CourseID,
					TeacherID:      td.Task.TeacherID,
					ClassroomID:    room.ID,
					TeachingTaskID: &td.Task.ID,
					ClassGroupID:   nil,
					Semester:       ctx.semester,
					DayOfWeek:      models.DayOfWeek(day),
					StartPeriod:    models.Period(start),
					Span:           span,
					Weeks:          fmt.Sprintf("%d-%d", td.Task.StartWeek, td.Task.EndWeek),
				}
				ctx.entries = append(ctx.entries, entry)

		// Occupy room (skip for shared venues), teacher, and all class groups
		for p := start; p < start+span; p++ {
			if room.Type != "体育馆" {
				ctx.roomOcc[occKey(day, p, room.ID)] = true
			}
			ctx.teacherOcc[occKey(day, p, td.Task.TeacherID)] = true
				}
				for _, cid := range td.ClassIDs {
					for p := start; p < start+span; p++ {
						ctx.classOcc[occKey(day, p, cid)] = true
					}
				}
				return true
			}
		}
	}
	return false
}

// Neighbor operation types
