package services

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"scheduling-system/backend/models"
	"time"
)

func (s *SASolver) PostOptimize(
	entries []models.ScheduleEntry,
	teachingTasks []models.TeachingTask,
	teachers []models.Teacher,
	classrooms []models.Classroom,
	lockedSlots []LockedTimeSlot,
	constraints []string,
	topN int,
) []models.ScheduleEntry {
	if len(entries) == 0 || topN <= 0 {
		return entries
	}

	// v0.5.2 Goal 4: capture a pristine copy so any final score regression can be rolled back.
	pristine := make([]models.ScheduleEntry, len(entries))
	copy(pristine, entries)

	// Build per-class-group student map
	classGroupStudents := make(map[uint]int)
	for _, tt := range teachingTasks {
		for _, c := range tt.Classes {
			if c.ClassGroup.ID > 0 {
				classGroupStudents[c.ClassGroupID] = c.ClassGroup.Students
			}
		}
	}

	// Build teaching task lookup with total students
	taskMap := make(map[uint]teachingTaskData)
	for _, tt := range teachingTasks {
		classIDs := make([]uint, len(tt.Classes))
		totalStudents := 0
		for j, c := range tt.Classes {
			classIDs[j] = c.ClassGroupID
			totalStudents += classGroupStudents[c.ClassGroupID]
		}
		taskMap[tt.ID] = teachingTaskData{
			Task:          tt,
			ClassIDs:      classIDs,
			TotalStudents: totalStudents,
		}
	}

	// Build teacher unavailable map
	teacherUnavailable := make(map[uint][]LockedTimeSlot)
	for _, t := range teachers {
		if t.UnavailableSlots != "" {
			var slots []LockedTimeSlot
			if err := json.Unmarshal([]byte(t.UnavailableSlots), &slots); err == nil && len(slots) > 0 {
				teacherUnavailable[t.ID] = slots
			}
		}
	}

	// Helper: check teacher unavailable
	isTeacherUnavailable := func(teacherID uint, day, start, span int) bool {
		slots, ok := teacherUnavailable[teacherID]
		if !ok {
			return false
		}
		for _, u := range slots {
			if int(u.DayOfWeek) == day && periodsOverlapInt(start, span, int(u.StartPeriod), u.Span) {
				return true
			}
		}
		return false
	}

	// Helper: check room capacity (sports venues have unlimited capacity)
	canRoomFit := func(room models.Classroom, td teachingTaskData) bool {
		if room.Type == "体育馆" {
			return true
		}
		return td.TotalStudents <= 0 || room.Capacity >= td.TotalStudents
	}

	validStarts := []int{0, 2, 4, 6, 8}
	_ = validStarts // v0.5.1: per-entry validStarts derived from entry.Span below

	// Score each entry individually to find the worst ones
	type entryScore struct {
		idx      int
		entry    models.ScheduleEntry
		marginal float64 // v0.5.2 Goal 4: how much score drops when this entry is removed
	}
	var scored []entryScore
	for i, e := range entries {
		scored = append(scored, entryScore{idx: i, entry: e})
	}

	limit := topN
	if limit > len(entries) {
		limit = len(entries)
	}

	// v0.5.2 Goal 4: rank entries by their true marginal contribution to
	// ScoreSchedule.FinalTotal. Entries whose removal barely changes the score
	// are "least important" and become PostOptimize candidates first.
	// This replaces the previous random shuffle disguised as "worst entries".
	//
	// Cost: O(N × ScoreSchedule) — acceptable at the end of solving (N ≤ ~50).
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	scorer := NewScoringService()
	ttListFull := make([]models.TeachingTask, len(teachingTasks))
	copy(ttListFull, teachingTasks)
	// Recover expected total sessions so completeness scaling participates
	// in the marginal ranking (removing an entry drops PlacedSessions by 1).
	expectedSessions := 0
	for _, tt := range teachingTasks {
		th := tt.TotalHours
		if th <= 0 {
			th = tt.Course.Hours
		}
		plan := resolveSessionPlan(th, tt.StartWeek, tt.EndWeek, tt.MaxHoursPerWeek, tt.PreferredSpan)
		expectedSessions += plan.SessionsPerWeek()
	}
	sportsIDs := make(map[uint]bool)
	for _, tt := range teachingTasks {
		if models.IsSportsCourse(tt.Course.Name) {
			sportsIDs[tt.CourseID] = true
		}
	}
	scoringCtx := NewScoringContextWithExpected(constraints, sportsIDs, ttListFull, expectedSessions)

	baselineBreakdown := scorer.ScoreSchedule(entries, teachers, classrooms, scoringCtx)
	baselineScore := baselineBreakdown.FinalTotal

	// Compute marginal contribution: FinalTotal(entries) − FinalTotal(entries \ {e})
	scratch := make([]models.ScheduleEntry, 0, len(entries))
	for i := range scored {
		scratch = scratch[:0]
		for j, e := range entries {
			if j == scored[i].idx {
				continue
			}
			scratch = append(scratch, e)
		}
		var b ScoreBreakdown
		if len(scratch) == 0 {
			b = ScoreBreakdown{}
		} else {
			b = scorer.ScoreSchedule(scratch, teachers, classrooms, scoringCtx)
		}
		scored[i].marginal = baselineScore - b.FinalTotal
	}

	// Sort ascending: lowest marginal contribution first = "least important" entries
	// are optimization candidates first. This is the real worst-first, replacing
	// the pre-v0.5.2 random shuffle.
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].marginal < scored[i].marginal {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Build a quick lookup for shared rooms (体育馆 have unlimited capacity and can be shared)
	sharedRoom := make(map[uint]bool)
	for _, room := range classrooms {
		if room.Type == "体育馆" {
			sharedRoom[room.ID] = true
		}
	}

	// Build current occupancy from all OTHER entries (excluding the one being optimized)
	buildOcc := func(excludeIdx int) (roomOcc, teacherOcc, classOcc map[string]bool) {
		roomOcc = make(map[string]bool)
		teacherOcc = make(map[string]bool)
		classOcc = make(map[string]bool)
		for i, e := range entries {
			if i == excludeIdx {
				continue
			}
			day, start, span := int(e.DayOfWeek), int(e.StartPeriod), e.Span
			if !sharedRoom[e.ClassroomID] {
				for p := start; p < start+span; p++ {
					roomOcc[fmt.Sprintf("%d-%d-%d", day, p, e.ClassroomID)] = true
				}
			}
			for p := start; p < start+span; p++ {
				teacherOcc[fmt.Sprintf("%d-%d-%d", day, p, e.TeacherID)] = true
			}
			if td, ok := taskMap[*e.TeachingTaskID]; ok {
				for _, cid := range td.ClassIDs {
					for p := start; p < start+span; p++ {
						classOcc[fmt.Sprintf("%d-%d-%d", day, p, cid)] = true
					}
				}
			}
		}
		return
	}

	improved := 0
	for _, es := range scored {
		if improved >= limit {
			break
		}

		e := es.entry
		eIdx := es.idx
		span := e.Span
		if span < 1 {
			span = 2
		}
		// v0.5.1: legal starts depend on this entry's span (block-alignment).
		perEntryStarts := make([]int, 0, 5)
		for _, p := range models.ValidStartsForSpan(span) {
			perEntryStarts = append(perEntryStarts, int(p))
		}
		if len(perEntryStarts) == 0 {
			continue
		}
		originalDay, originalStart, originalRoom := int(e.DayOfWeek), int(e.StartPeriod), e.ClassroomID

		roomOcc, teacherOcc, classOcc := buildOcc(eIdx)

		bestDay, bestStart, bestRoom := originalDay, originalStart, originalRoom
		bestPenalty := 999

		// Simple penalty: early/late/weekend
		calcPenalty := func(day, start int) int {
			p := 0
			if day >= 5 {
				p += 3
			}
			if start <= 1 {
				p += 1
			}
			if start >= 6 {
				p += 1
			}
			return p
		}

		for _, day := range rng.Perm(7) {
			// Check locked slots (at day level)
			lsBlocked := false
			for _, ls := range lockedSlots {
				if int(ls.DayOfWeek) == day && periodsOverlapInt(0, 11, int(ls.StartPeriod), ls.Span) {
					for _, start := range perEntryStarts {
						if periodsOverlapInt(start, span, int(ls.StartPeriod), ls.Span) {
							lsBlocked = true
							break
						}
					}
				}
			}
			if lsBlocked {
				continue
			}

			for _, start := range perEntryStarts {
				// Check teacher unavailable (at specific day+start)
				if isTeacherUnavailable(e.TeacherID, day, start, span) {
					continue
				}

				penalty := calcPenalty(day, start)
				if penalty > bestPenalty {
					continue
				}

				// Check teacher busy
				teacherBusy := false
				for p := start; p < start+span; p++ {
					if teacherOcc[fmt.Sprintf("%d-%d-%d", day, p, e.TeacherID)] {
						teacherBusy = true
						break
					}
				}
				if teacherBusy {
					continue
				}

				// Check class groups busy
				classBusy := false
				if td, ok := taskMap[*e.TeachingTaskID]; ok {
					for _, cid := range td.ClassIDs {
						for p := start; p < start+span; p++ {
							if classOcc[fmt.Sprintf("%d-%d-%d", day, p, cid)] {
								classBusy = true
								break
							}
						}
						if classBusy {
							break
						}
					}
				}
				if classBusy {
					continue
				}

				// Try rooms
				td, tdOk := taskMap[*e.TeachingTaskID]
				// Determine required room type (same logic as sa_initial.go)
				requiredRoomType := ""
				if tdOk {
					requiredRoomType = roomTypeForCourse(td.Task.Course.Name)
				}
				for _, room := range classrooms {
				// Check room type
				if requiredRoomType != "" {
					if room.Type != requiredRoomType {
						continue
					}
				} else if room.Type == "体育馆" || room.Type == "实验室" || room.Type == "机房" {
					continue // regular courses cannot use specialty rooms
				}
					// Check room capacity
					if tdOk && !canRoomFit(room, td) {
						continue
					}

				roomBusy := false
				if room.Type != "体育馆" {
					for p := start; p < start+span; p++ {
						if roomOcc[fmt.Sprintf("%d-%d-%d", day, p, room.ID)] {
							roomBusy = true
							break
						}
					}
				}
				if roomBusy {
						continue
					}

					if penalty < bestPenalty || (penalty == bestPenalty && day == originalDay && start == originalStart && room.ID == originalRoom) {
						bestDay, bestStart, bestRoom = day, start, room.ID
						bestPenalty = penalty
						if penalty == 0 {
							goto found
						}
					}
				}
			}
		}
	found:
		if bestDay != originalDay || bestStart != originalStart || bestRoom != originalRoom {
			e.DayOfWeek = models.DayOfWeek(bestDay)
			e.StartPeriod = models.Period(bestStart)
			e.ClassroomID = bestRoom
			entries[eIdx] = e
			improved++
		}
	}

	// v0.5.2 Goal 4: verify the accumulated changes actually improve (or match)
	// the pre-optimize baseline. If not, roll back to the snapshot. This turns
	// PostOptimize from "best effort local search" into a Score-monotone
	// operation — the acceptance criterion "Score cannot drop after PostOptimize"
	// becomes a hard guarantee.
	if improved > 0 {
		finalBreakdown := scorer.ScoreSchedule(entries, teachers, classrooms, scoringCtx)
		if finalBreakdown.FinalTotal < baselineScore {
			// Roll back all changes by returning the pristine copy captured earlier.
			copy(entries, pristine)
		}
	}

	return entries
}

// ---- Helpers ----

// roomTypeForCourse determines the required room type from course name.
// Same logic as schedulingContext.getRequiredRoomType in sa_solver.go.
func roomTypeForCourse(courseName string) string {
	if models.IsSportsCourse(courseName) {
		return "体育馆"
	}
	if IsLabCourse(courseName) {
		return "实验室"
	}
	if IsComputerCourse(courseName) {
		return "机房"
	}
	return ""
}
