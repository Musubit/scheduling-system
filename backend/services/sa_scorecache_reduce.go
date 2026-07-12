package services

// v0.5.2 Goal 3 continued: pure reduction from cache -> FinalTotal.

import (
	"math"
	"sort"
)

// scoreFromCache reduces the incrementally-maintained cache into a ScoreBreakdown
// that must equal ScoreSchedule.ScoreSchedule(...) for the same entries.
//
// Correctness parity checked by TestDeltaScoreMatchesFullScore.
func (c *scoreCache) scoreFromCache(
	enabled map[string]bool,
	sportsCourseIDs map[uint]bool,
	expectedTotalSessions int,
) ScoreBreakdown {
	// --- per-category max (same formula as ScoreSchedule) ---
	enabledCount := 0
	if enabled["teacher_preference"] {
		enabledCount++
	}
	if enabled["course_dispersed"] {
		enabledCount++
	}
	if enabled["teacher_days_limit"] {
		enabledCount++
	}
	if enabled["low_floor_preference"] {
		enabledCount++
	}
	if enabled["avoid_saturday"] || enabled["avoid_sunday"] {
		enabledCount++
	}
	if enabled["pe_preferred_periods"] && sportsCourseIDs != nil {
		enabledCount++
	}
	if enabled["student_fatigue"] && len(c.classDayBits) > 0 {
		enabledCount++
	}
	perCategoryMax := 100.0 / float64(enabledCount)
	if enabledCount == 0 {
		perCategoryMax = 25.0
	}

	bd := ScoreBreakdown{
		PerCategoryMax:       math.Round(perCategoryMax*100) / 100,
		EnabledCategoryCount: enabledCount,
	}

	// 1) teacher_preference — mirror scoreTeacherPreferences
	if enabled["teacher_preference"] {
		var totalPenalty float64
		prefTeacherCount := 0
		anyStats := false
		for tid, total := range c.teacherTotal {
			if total <= 0 {
				continue
			}
			anyStats = true
			t := c.teacherByID[tid]
			if t == nil || !(t.PreferNoEarly || t.PreferNoLate) {
				continue
			}
			prefTeacherCount++
			early := c.teacherEarly[tid]
			late := c.teacherLate[tid]
			totalPenalty += float64(early+late) / float64(total)
		}
		if !anyStats || prefTeacherCount == 0 {
			bd.TeacherPref = perCategoryMax
		} else {
			avgPenalty := totalPenalty / float64(prefTeacherCount)
			s := perCategoryMax * (1.0 - avgPenalty)
			if s < 0 {
				s = 0
			}
			bd.TeacherPref = s
		}
	}

	// 2) course_dispersed — mirror scoreCourseSpacing
	if enabled["course_dispersed"] {
		if len(c.courseDayCount) == 0 {
			bd.CourseSpacing = perCategoryMax
		} else {
			var totalScore float64
			for _, dc := range c.courseDayCount {
				totalSessions := 0
				days := make([]int, 0, 7)
				for d := 0; d < 7; d++ {
					if dc[d] > 0 {
						totalSessions += dc[d]
						days = append(days, d)
					}
				}
				if totalSessions <= 1 {
					totalScore += 1.0
					continue
				}
				if len(days) == 1 {
					totalScore += 1.0 / float64(totalSessions)
					continue
				}
				sort.Ints(days)
				gapSum := 0.0
				for i := 0; i < len(days)-1; i++ {
					gap := days[i+1] - days[i]
					switch {
					case gap >= 3:
						gapSum += 1.0
					case gap == 2:
						gapSum += 0.8
					case gap == 1:
						gapSum += 0.4
					}
				}
				gapScore := gapSum / float64(len(days)-1)
				sameDayExcess := 0
				maxDaily := 0
				for _, d := range days {
					cnt := dc[d]
					if cnt > maxDaily {
						maxDaily = cnt
					}
					if cnt > 1 {
						sameDayExcess += cnt - 1
					}
				}
				concentrationPenalty := float64(sameDayExcess) * 0.3
				idealMax := (totalSessions + len(days) - 1) / len(days)
				balancePenalty := 0.0
				if maxDaily > idealMax {
					balancePenalty = float64(maxDaily-idealMax) * 0.15
				}
				cs := gapScore * (1.0 - concentrationPenalty - balancePenalty)
				if cs < 0 {
					cs = 0
				}
				totalScore += cs
			}
			bd.CourseSpacing = perCategoryMax * (totalScore / float64(len(c.courseDayCount)))
		}
	}

	// 3) teacher_days_limit — mirror scoreTeacherDays
	if enabled["teacher_days_limit"] {
		if len(c.teacherDayCount) == 0 {
			bd.TeacherDays = perCategoryMax
		} else {
			var totalScore float64
			active := 0
			for tid, dc := range c.teacherDayCount {
				actualDays := 0
				for d := 0; d < 7; d++ {
					if dc[d] > 0 {
						actualDays++
					}
				}
				if actualDays == 0 {
					continue
				}
				active++
				maxDays := 3
				if t := c.teacherByID[tid]; t != nil && t.MaxDaysPerWeek > 0 {
					maxDays = t.MaxDaysPerWeek
				}
				if actualDays <= maxDays {
					totalScore += 1.0
				} else {
					extra := actualDays - maxDays
					penalty := float64(extra) / float64(maxDays)
					s := 1.0 - penalty
					if s < 0 {
						s = 0
					}
					totalScore += s
				}
			}
			if active == 0 {
				bd.TeacherDays = perCategoryMax
			} else {
				bd.TeacherDays = perCategoryMax * (totalScore / float64(active))
			}
		}
	}

	// 4) low_floor_preference — mirror scoreLowFloorPref
	if enabled["low_floor_preference"] {
		active := 0
		var totalScore float64
		if c.maxFloor <= 1 {
			bd.LowFloorPref = perCategoryMax
		} else {
			for tid, cnt := range c.teacherFloorCount {
				if cnt <= 0 {
					continue
				}
				active++
				avgFloor := c.teacherFloorSum[tid] / float64(cnt)
				s := 1.0 - (avgFloor-1.0)/float64(c.maxFloor-1)
				if s < 0 {
					s = 0
				}
				if s > 1.0 {
					s = 1.0
				}
				totalScore += s
				_ = tid
			}
			if active == 0 {
				bd.LowFloorPref = perCategoryMax
			} else {
				bd.LowFloorPref = perCategoryMax * (totalScore / float64(active))
			}
		}
	}

	// 5) avoid_saturday / avoid_sunday — mirror scoreWeekendAvoid
	if enabled["avoid_saturday"] || enabled["avoid_sunday"] {
		if c.totalEntries == 0 {
			bd.WeekendAvoid = perCategoryMax
		} else {
			total := 0
			if enabled["avoid_saturday"] {
				total += c.weekendSat
			}
			if enabled["avoid_sunday"] {
				total += c.weekendSun
			}
			if total == 0 {
				bd.WeekendAvoid = perCategoryMax
			} else {
				penalty := float64(total) / float64(c.totalEntries)
				s := perCategoryMax * (1.0 - penalty)
				if s < 0 {
					s = 0
				}
				bd.WeekendAvoid = s
			}
		}
	}

	// 6) pe_preferred_periods — mirror scorePePeriodPref
	if enabled["pe_preferred_periods"] && sportsCourseIDs != nil {
		if c.peTotal == 0 {
			bd.PePeriodPref = perCategoryMax
		} else {
			ratio := float64(c.peAtPref) / float64(c.peTotal)
			bd.PePeriodPref = perCategoryMax * ratio
		}
	}

	// 7) student_fatigue — mirror scoreStudentFatigue via bitmap max-run
	if enabled["student_fatigue"] && len(c.classDayBits) > 0 {
		maxConsecutive := 0
		for _, days := range c.classDayBits {
			for d := 0; d < 7; d++ {
				bits := days[d]
				if bits == 0 {
					continue
				}
				longest := 0
				current := 0
				for p := 0; p <= 10; p++ {
					if bits&(uint16(1)<<uint(p)) != 0 {
						current++
						if current > longest {
							longest = current
						}
					} else {
						current = 0
					}
				}
				if longest > maxConsecutive {
					maxConsecutive = longest
				}
			}
		}
		threshold := 4
		if maxConsecutive <= threshold {
			bd.StudentFatigue = perCategoryMax
		} else {
			extra := maxConsecutive - threshold
			maxPenaltyRange := 6
			if extra > maxPenaltyRange {
				extra = maxPenaltyRange
			}
			penaltyFactor := float64(extra) / float64(maxPenaltyRange)
			s := perCategoryMax * (1.0 - penaltyFactor)
			if s < 0 {
				s = 0
			}
			bd.StudentFatigue = s
		}
	}

	bd.Total = math.Round((bd.TeacherPref+bd.CourseSpacing+bd.TeacherDays+bd.LowFloorPref+bd.WeekendAvoid+bd.PePeriodPref+bd.StudentFatigue)*100) / 100

	// Placement completeness (β curve, same as ScoreSchedule)
	bd.PlacedSessions = c.totalEntries
	if expectedTotalSessions > 0 {
		bd.ExpectedSessions = expectedTotalSessions
		ratio := float64(bd.PlacedSessions) / float64(expectedTotalSessions)
		if ratio > 1 {
			ratio = 1
		}
		if ratio < 0 {
			ratio = 0
		}
		bd.Completeness = math.Round(ratio*10000) / 10000
		factor := ratio * (0.5 + 0.5*ratio)
		bd.FinalTotal = math.Round(bd.Total*factor*100) / 100
	} else {
		bd.ExpectedSessions = bd.PlacedSessions
		bd.Completeness = 1.0
		bd.FinalTotal = bd.Total
	}
	return bd
}
