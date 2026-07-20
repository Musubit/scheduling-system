package time

import (
	"math"
	"sort"

	schedtypes "scheduling-system/backend/scheduling/types"
)

// timeScoreDetail 是三个时间族维度的内部中间评分。
type timeScoreDetail struct {
	timeVal    float64
	teacherVal float64
	studentVal float64
	perMax     float64
	placed     int
	expected   int
}

// finalTotal 计算 completeness-scaled 总分。
func (d timeScoreDetail) finalTotal() float64 {
	total := d.timeVal + d.teacherVal + d.studentVal
	if d.expected > 0 {
		ratio := float64(d.placed) / float64(d.expected)
		if ratio > 1 {
			ratio = 1
		}
		factor := ratio * (0.5 + 0.5*ratio)
		return math.Round(total*factor*100) / 100
	}
	return math.Round(total*100) / 100
}

// toTimeScoreDetail 转为 IScorer 期望的 TimeScoreDetail。
func (d timeScoreDetail) toTimeScoreDetail() schedtypes.TimeScoreDetail {
	makeBucket := func(val float64) *schedtypes.ScoreBucket {
		return &schedtypes.ScoreBucket{
			Value: math.Round(val*100) / 100,
			Max:   math.Round(d.perMax*100) / 100,
		}
	}
	return schedtypes.TimeScoreDetail{
		Time:    makeBucket(d.timeVal),
		Teacher: makeBucket(d.teacherVal),
		Student: makeBucket(d.studentVal),
	}
}

// scoreDetail 从增量缓存计算 TimeScoreDetail。
func (c *scoreCache) scoreDetail(
	enabled map[string]bool,
	sportsCourseIDs map[uint]bool,
	expectedTotalSessions int,
) timeScoreDetail {
	// 计算 perBucketMax
	enabledCount := 0
	timeDims := []string{"teacher_preference", "course_dispersed", "avoid_saturday", "avoid_sunday", "pe_preferred_periods"}
	for _, dim := range timeDims {
		if enabled[dim] {
			enabledCount++
		}
	}
	if enabled["teacher_days_limit"] {
		enabledCount++
	}
	if enabled["student_fatigue"] && len(c.classDayBits) > 0 {
		enabledCount++
	}
	perMax := 100.0 / float64(enabledCount)
	if enabledCount == 0 {
		perMax = 25.0
	}

	d := timeScoreDetail{
		perMax:   perMax,
		placed:   c.totalEntries,
		expected: expectedTotalSessions,
	}

	// === TIME bucket: teacher_preference + course_dispersed + weekend + PE ===
	var timeScore float64
	timeContribs := 0

	// 1) teacher_preference
	if enabled["teacher_preference"] {
		var totalPenalty float64
		prefTeacherCount := 0
		anyStats := false
		for tid, total := range c.teacherTotal {
			if total <= 0 {
				continue
			}
			anyStats = true
			tv := c.teacherByID[tid]
			if tv == nil || !(tv.PreferNoEarly || tv.PreferNoLate) {
				continue
			}
			prefTeacherCount++
			early := c.teacherEarly[tid]
			late := c.teacherLate[tid]
			totalPenalty += float64(early+late) / float64(total)
		}
		if !anyStats || prefTeacherCount == 0 {
			timeScore += perMax
		} else {
			avgPenalty := totalPenalty / float64(prefTeacherCount)
			s := perMax * (1.0 - avgPenalty)
			if s < 0 {
				s = 0
			}
			timeScore += s
		}
		timeContribs++
	}

	// 2) course_dispersed
	if enabled["course_dispersed"] {
		if len(c.courseDayCount) == 0 {
			timeScore += perMax
		} else {
			var totalDispersed float64
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
					totalDispersed += 1.0
					continue
				}
				if len(days) == 1 {
					totalDispersed += 1.0 / float64(totalSessions)
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
				totalDispersed += cs
			}
			timeScore += perMax * (totalDispersed / float64(len(c.courseDayCount)))
		}
		timeContribs++
	}

	// 3) weekend avoidance (combined saturday + sunday)
	if enabled["avoid_saturday"] || enabled["avoid_sunday"] {
		if c.totalEntries == 0 {
			timeScore += perMax
		} else {
			total := 0
			if enabled["avoid_saturday"] {
				total += c.weekendSat
			}
			if enabled["avoid_sunday"] {
				total += c.weekendSun
			}
			if total == 0 {
				timeScore += perMax
			} else {
				penalty := float64(total) / float64(c.totalEntries)
				s := perMax * (1.0 - penalty)
				if s < 0 {
					s = 0
				}
				timeScore += s
			}
		}
		timeContribs++
	}

	// 4) PE preferred periods
	if enabled["pe_preferred_periods"] && sportsCourseIDs != nil {
		if c.peTotal == 0 {
			timeScore += perMax
		} else {
			ratio := float64(c.peAtPref) / float64(c.peTotal)
			timeScore += perMax * ratio
		}
		timeContribs++
	}

	// 归一化 time bucket 分数到 perMax
	if timeContribs > 0 {
		d.timeVal = timeScore / float64(timeContribs)
	} else {
		d.timeVal = perMax
	}

	// === TEACHER bucket: teacher_days_limit ===
	if enabled["teacher_days_limit"] {
		if len(c.teacherDayCount) == 0 {
			d.teacherVal = perMax
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
				if tv := c.teacherByID[tid]; tv != nil && tv.MaxDaysPerWeek > 0 {
					maxDays = tv.MaxDaysPerWeek
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
				d.teacherVal = perMax
			} else {
				d.teacherVal = perMax * (totalScore / float64(active))
			}
		}
	} else {
		d.teacherVal = perMax
	}

	// === STUDENT bucket: student_fatigue ===
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
			d.studentVal = perMax
		} else {
			extra := maxConsecutive - threshold
			maxPenaltyRange := 6
			if extra > maxPenaltyRange {
				extra = maxPenaltyRange
			}
			penaltyFactor := float64(extra) / float64(maxPenaltyRange)
			s := perMax * (1.0 - penaltyFactor)
			if s < 0 {
				s = 0
			}
			d.studentVal = s
		}
	} else {
		d.studentVal = perMax
	}

	return d
}
