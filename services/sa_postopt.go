package services

import (
	"fmt"
	"math/rand"
	"scheduling-system/models"
	"time"
)

func (s *SASolver) PostOptimize(
	entries []models.ScheduleEntry,
	teachingTasks []models.TeachingTask,
	teachers []models.Teacher,
	classrooms []models.Classroom,
	lockedSlots []lockedTimeSlot,
	constraints []string,
	topN int,
) []models.ScheduleEntry {
	if len(entries) == 0 || topN <= 0 {
		return entries
	}

	// Build teaching task lookup
	taskMap := make(map[uint]teachingTaskData)
	for _, tt := range teachingTasks {
		classIDs := make([]uint, len(tt.Classes))
		for j, c := range tt.Classes {
			classIDs[j] = c.ClassGroupID
		}
		taskMap[tt.ID] = teachingTaskData{Task: tt, ClassIDs: classIDs}
	}

	validStarts := []int{0, 2, 4, 6, 8}

	// Score each entry individually to find the worst ones
	type entryScore struct {
		idx   int
		entry models.ScheduleEntry
	}
	// Use a simple heuristic: entries on weekends or with early/late periods score lower
	var scored []entryScore
	for i, e := range entries {
		scored = append(scored, entryScore{idx: i, entry: e})
	}

	// Sort by a simple quality heuristic (weekend = worse, early/late = worse)
	// We'll just try all entries up to topN
	limit := topN
	if limit > len(entries) {
		limit = len(entries)
	}

	// Shuffle scored to avoid bias (shuffle the order we process entries, not entries themselves)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	rng.Shuffle(len(scored), func(i, j int) { scored[i], scored[j] = scored[j], scored[i] })

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
			for p := start; p < start+span; p++ {
				roomOcc[fmt.Sprintf("%d-%d-%d", day, p, e.ClassroomID)] = true
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
		originalDay, originalStart, originalRoom := int(e.DayOfWeek), int(e.StartPeriod), e.ClassroomID

		roomOcc, teacherOcc, classOcc := buildOcc(eIdx)

		bestDay, bestStart, bestRoom := originalDay, originalStart, originalRoom
		bestPenalty := 999 // lower is better for this heuristic

		// Simple penalty: early/late/wekkend
		calcPenalty := func(day, start int) int {
			p := 0
			if day >= 5 {
				p += 3 // weekend penalty
			}
			if start <= 1 {
				p += 1 // early
			}
			if start >= 6 {
				p += 1 // late
			}
			return p
		}

		for _, day := range rng.Perm(7) {
			// Check locked slots
			lsBlocked := false
			for _, ls := range lockedSlots {
				if int(ls.DayOfWeek) == day && periodsOverlapInt(0, 11, int(ls.StartPeriod), ls.Span) {
					// Check if any start period would overlap
					for _, start := range validStarts {
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

			for _, start := range validStarts {
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
				for _, room := range classrooms {
					roomBusy := false
					for p := start; p < start+span; p++ {
						if roomOcc[fmt.Sprintf("%d-%d-%d", day, p, room.ID)] {
							roomBusy = true
							break
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

	return entries
}

// ---- Helpers ----

