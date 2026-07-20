// Package score 提供 4-bucket 排课评分实现。
//
// Scorer 实现 types.IScorer，是排课结果的最终评分权威。
// 四个维度按 dims 顺序计算: time, teacher, student, resource。
// TIME_ONLY 模式下 resource 维度被禁用（dims 不含 "resource"）。
package score

import (
	"math"
	"sort"

	schedtypes "scheduling-system/backend/scheduling/types"
)

// Scorer 是 4-bucket 评分器的实现。
type Scorer struct{}

// NewScorer 创建一个 Scorer。
func NewScorer() *Scorer {
	return &Scorer{}
}

// 编译期接口检查。
var _ schedtypes.IScorer = (*Scorer)(nil)

// Score 实现 types.IScorer。
func (s *Scorer) Score(
	assignments []schedtypes.TimeAssignmentDraft,
	allocations []schedtypes.RoomAllocationDraft,
	teachers []schedtypes.TeacherView,
	classrooms []schedtypes.ClassroomView,
	tasks []schedtypes.TeachingTaskView,
	dims []string,
) schedtypes.ScoreBreakdown {
	result := schedtypes.ScoreBreakdown{
		EnabledDimensions: dims,
		PlacedSessions:    len(assignments),
	}

	// 计算 perBucketMax
	if len(dims) == 0 {
		dims = []string{"time", "teacher", "student", "resource"}
	}
	perBucketMax := 100.0 / float64(len(dims))
	result.PerBucketMax = math.Round(perBucketMax*100) / 100

	// 预期 session 数
	expected := 0
	for _, t := range tasks {
		plan := sessionsPerWeek(t.CourseHours, t.StartWeek, t.EndWeek, t.MaxHoursPerWeek, t.PreferredSpan)
		expected += plan
	}
	result.ExpectedSessions = expected

	// 构建查找表
	taskByID := make(map[uint]schedtypes.TeachingTaskView, len(tasks))
	for _, t := range tasks {
		taskByID[t.ID] = t
	}
	teacherByID := make(map[uint]schedtypes.TeacherView, len(teachers))
	for _, t := range teachers {
		teacherByID[t.ID] = t
	}
	classroomByID := make(map[uint]schedtypes.ClassroomView, len(classrooms))
	for _, c := range classrooms {
		classroomByID[c.ID] = c
	}
	allocByRef := make(map[int]schedtypes.RoomAllocationDraft, len(allocations))
	for _, a := range allocations {
		allocByRef[a.LocalRef] = a
	}

	// 按 dims 顺序计算各 bucket
	for _, dim := range dims {
		bucket := schedtypes.ScoreBucket{
			Max:     math.Round(perBucketMax*100) / 100,
			Details: make(map[string]float64),
		}

		switch dim {
		case "time":
			bucket.Value = s.scoreTimeBucket(assignments, taskByID, teacherByID)
			result.Time = &bucket
		case "teacher":
			bucket.Value = s.scoreTeacherBucket(assignments, taskByID, teacherByID)
			result.Teacher = &bucket
		case "student":
			bucket.Value = s.scoreStudentBucket(assignments, taskByID)
			result.Student = &bucket
		case "resource":
			bucket.Value = s.scoreResourceBucket(assignments, allocations, taskByID, teacherByID, classroomByID)
			result.Resource = &bucket
		}
	}

	// 计算 Total
	var total float64
	for _, b := range []*schedtypes.ScoreBucket{result.Time, result.Teacher, result.Student, result.Resource} {
		if b != nil {
			total += b.Value
		}
	}
	result.Total = math.Round(total*100) / 100

	// Completeness
	if result.ExpectedSessions > 0 {
		ratio := float64(result.PlacedSessions) / float64(result.ExpectedSessions)
		if ratio > 1 {
			ratio = 1
		}
		result.Completeness = math.Round(ratio*10000) / 10000
		factor := ratio * (0.5 + 0.5*ratio)
		result.FinalTotal = math.Round(result.Total*factor*100) / 100
	} else {
		result.Completeness = 1.0
		result.FinalTotal = result.Total
	}

	return result
}

// scoreTimeBucket 计算时间维度评分。
// 子维度: teacher_preference (早晚偏好), course_dispersed (分散度),
//
//	weekend_avoidance (周末规避), pe_preferred_periods (体育偏好时段)
func (s *Scorer) scoreTimeBucket(
	assignments []schedtypes.TimeAssignmentDraft,
	taskByID map[uint]schedtypes.TeachingTaskView,
	teacherByID map[uint]schedtypes.TeacherView,
) float64 {
	if len(assignments) == 0 {
		return 100.0
	}

	subScores := make([]float64, 0, 4)

	// 1. Teacher preference: 早晚课比例
	{
		type teacherStats struct {
			early, late, total int
			hasPref            bool
		}
		tstats := make(map[uint]*teacherStats)
		for _, a := range assignments {
			task := taskByID[a.TeachingTaskID]
			tv := teacherByID[task.TeacherID]
			ts := tstats[task.TeacherID]
			if ts == nil {
				ts = &teacherStats{}
				tstats[task.TeacherID] = ts
			}
			ts.total++
			if tv.PreferNoEarly && int(a.StartPeriod) <= 1 {
				ts.early++
			}
			if tv.PreferNoLate && int(a.StartPeriod) >= 6 {
				ts.late++
			}
			if tv.PreferNoEarly || tv.PreferNoLate {
				ts.hasPref = true
			}
		}
		var totalPenalty float64
		prefCount := 0
		for _, ts := range tstats {
			if ts.hasPref && ts.total > 0 {
				prefCount++
				totalPenalty += float64(ts.early+ts.late) / float64(ts.total)
			}
		}
		if prefCount == 0 {
			subScores = append(subScores, 100.0)
		} else {
			s := 100.0 * (1.0 - totalPenalty/float64(prefCount))
			if s < 0 {
				s = 0
			}
			subScores = append(subScores, s)
		}
	}

	// 2. Course dispersed: 同课程分散度
	{
		courseDays := make(map[uint]*[7]int)
		for _, a := range assignments {
			task := taskByID[a.TeachingTaskID]
			if _, ok := courseDays[task.CourseID]; !ok {
				courseDays[task.CourseID] = &[7]int{}
			}
			courseDays[task.CourseID][int(a.DayOfWeek)]++
		}
		if len(courseDays) == 0 {
			subScores = append(subScores, 100.0)
		} else {
			var totalDispersion float64
			for _, dc := range courseDays {
				totalSessions := 0
				days := make([]int, 0, 7)
				for d := 0; d < 7; d++ {
					if dc[d] > 0 {
						totalSessions += dc[d]
						days = append(days, d)
					}
				}
				if totalSessions <= 1 {
					totalDispersion += 1.0
					continue
				}
				if len(days) == 1 {
					totalDispersion += 1.0 / float64(totalSessions)
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
				// 同日聚集惩罚
				sameDayExcess := 0
				maxDaily := 0
				for _, d := range days {
					if dc[d] > maxDaily {
						maxDaily = dc[d]
					}
					if dc[d] > 1 {
						sameDayExcess += dc[d] - 1
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
				totalDispersion += cs
			}
			subScores = append(subScores, 100.0*(totalDispersion/float64(len(courseDays))))
		}
	}

	// 3. Weekend avoidance
	{
		sat, sun, total := 0, 0, len(assignments)
		for _, a := range assignments {
			if int(a.DayOfWeek) == 5 {
				sat++
			}
			if int(a.DayOfWeek) == 6 {
				sun++
			}
		}
		if total == 0 || (sat+sun == 0) {
			subScores = append(subScores, 100.0)
		} else {
			penalty := float64(sat+sun) / float64(total)
			s := 100.0 * (1.0 - penalty)
			if s < 0 {
				s = 0
			}
			subScores = append(subScores, s)
		}
	}

	// 4. PE preferred periods
	{
		peAtPref, peTotal := 0, 0
		for _, a := range assignments {
			task := taskByID[a.TeachingTaskID]
			if task.IsSports {
				peTotal++
				start := int(a.StartPeriod)
				if start == 2 || start == 6 {
					peAtPref++
				}
			}
		}
		if peTotal == 0 {
			subScores = append(subScores, 100.0)
		} else {
			subScores = append(subScores, 100.0*float64(peAtPref)/float64(peTotal))
		}
	}

	// 归一化
	var sum float64
	for _, v := range subScores {
		sum += v
	}
	return math.Round(sum/float64(len(subScores))*100) / 100
}

// scoreTeacherBucket 计算教师维度评分。
// 子维度: teacher_days_limit (最大上课天数)
func (s *Scorer) scoreTeacherBucket(
	assignments []schedtypes.TimeAssignmentDraft,
	taskByID map[uint]schedtypes.TeachingTaskView,
	teacherByID map[uint]schedtypes.TeacherView,
) float64 {
	if len(assignments) == 0 {
		return 100.0
	}

	teacherDays := make(map[uint]*[7]int)
	for _, a := range assignments {
		task := taskByID[a.TeachingTaskID]
		if _, ok := teacherDays[task.TeacherID]; !ok {
			teacherDays[task.TeacherID] = &[7]int{}
		}
		teacherDays[task.TeacherID][int(a.DayOfWeek)]++
	}

	var totalScore float64
	active := 0
	for tid, dc := range teacherDays {
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
		if tv := teacherByID[tid]; tv.MaxDaysPerWeek > 0 {
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
		return 100.0
	}
	return math.Round(100.0*(totalScore/float64(active))*100) / 100
}

// scoreStudentBucket 计算学生维度评分。
// 子维度: student_fatigue (最大连续课时数)
func (s *Scorer) scoreStudentBucket(
	assignments []schedtypes.TimeAssignmentDraft,
	taskByID map[uint]schedtypes.TeachingTaskView,
) float64 {
	if len(assignments) == 0 {
		return 100.0
	}

	// 按 classGroup × day 构建占用位图
	classDayBits := make(map[uint]*[7]uint16)
	for _, a := range assignments {
		task := taskByID[a.TeachingTaskID]
		for _, cgID := range task.ClassGroupIDs {
			if _, ok := classDayBits[cgID]; !ok {
				classDayBits[cgID] = &[7]uint16{}
			}
			bits := &classDayBits[cgID][int(a.DayOfWeek)]
			for p := int(a.StartPeriod); p < int(a.StartPeriod)+a.Span; p++ {
				if p >= 0 && p <= 10 {
					*bits |= uint16(1) << uint(p)
				}
			}
		}
	}

	maxConsecutive := 0
	for _, days := range classDayBits {
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
		return 100.0
	}
	extra := maxConsecutive - threshold
	maxPenaltyRange := 6
	if extra > maxPenaltyRange {
		extra = maxPenaltyRange
	}
	penaltyFactor := float64(extra) / float64(maxPenaltyRange)
	scoreVal := 100.0 * (1.0 - penaltyFactor)
	if scoreVal < 0 {
		scoreVal = 0
	}
	return math.Round(scoreVal*100) / 100
}

// scoreResourceBucket 计算资源维度评分。
// 子维度: low_floor_preference (教师低楼层偏好), capacity_fit (容量利用率)
func (s *Scorer) scoreResourceBucket(
	assignments []schedtypes.TimeAssignmentDraft,
	allocations []schedtypes.RoomAllocationDraft,
	taskByID map[uint]schedtypes.TeachingTaskView,
	teacherByID map[uint]schedtypes.TeacherView,
	classroomByID map[uint]schedtypes.ClassroomView,
) float64 {
	if len(allocations) == 0 {
		return 100.0
	}

	subScores := make([]float64, 0, 2)

	// 1. Low floor preference
	{
		// 找最大楼层
		maxFloor := 1
		for _, c := range classroomByID {
			if c.Floor > maxFloor {
				maxFloor = c.Floor
			}
		}

		teacherFloorSum := make(map[uint]float64)
		teacherFloorCount := make(map[uint]int)

		for i, a := range assignments {
			alloc, ok := allocationsByRef(allocations, i)
			if !ok {
				continue
			}
			task := taskByID[a.TeachingTaskID]
			tv := teacherByID[task.TeacherID]
			if !tv.PreferLowFloor {
				continue
			}
			room := classroomByID[alloc.ClassroomID]
			teacherFloorSum[task.TeacherID] += float64(room.Floor)
			teacherFloorCount[task.TeacherID]++
		}

		if maxFloor <= 1 || len(teacherFloorCount) == 0 {
			subScores = append(subScores, 100.0)
		} else {
			var totalScore float64
			for tid, cnt := range teacherFloorCount {
				if cnt == 0 {
					continue
				}
				avgFloor := teacherFloorSum[tid] / float64(cnt)
				s := 1.0 - (avgFloor-1.0)/float64(maxFloor-1)
				if s < 0 {
					s = 0
				}
				if s > 1.0 {
					s = 1.0
				}
				totalScore += s
				_ = tid
			}
			subScores = append(subScores, 100.0*(totalScore/float64(len(teacherFloorCount))))
		}
	}

	// 2. Capacity utilization
	{
		totalWaste := 0
		totalCapacity := 0
		for i, a := range assignments {
			alloc, ok := allocationsByRef(allocations, i)
			if !ok {
				continue
			}
			room := classroomByID[alloc.ClassroomID]
			task := taskByID[a.TeachingTaskID]
			if room.Capacity > 0 {
				totalCapacity += room.Capacity
				totalWaste += room.Capacity - task.TotalStudents
			}
		}
		if totalCapacity == 0 || totalWaste <= 0 {
			subScores = append(subScores, 100.0)
		} else {
			ratio := 1.0 - float64(totalWaste)/float64(totalCapacity)
			if ratio < 0 {
				ratio = 0
			}
			subScores = append(subScores, 100.0*ratio)
		}
	}

	var sum float64
	for _, v := range subScores {
		sum += v
	}
	return math.Round(sum/float64(len(subScores))*100) / 100
}

// allocationsByRef 按 LocalRef 查找分配。按 index 匹配 fallback。
func allocationsByRef(allocations []schedtypes.RoomAllocationDraft, ref int) (schedtypes.RoomAllocationDraft, bool) {
	for _, a := range allocations {
		if a.LocalRef == ref {
			return a, true
		}
	}
	return schedtypes.RoomAllocationDraft{}, false
}

// sessionsPerWeek 计算教学任务每周的 session 数（与 time 包逻辑一致）。
func sessionsPerWeek(courseHours, startWeek, endWeek, maxHoursPerWeek, preferredSpan int) int {
	weeks := endWeek - startWeek + 1
	if weeks < 1 {
		weeks = 1
	}
	if courseHours <= 0 {
		return 1
	}
	weeklyHoursR := (courseHours + weeks/2) / weeks
	if weeklyHoursR < 1 {
		weeklyHoursR = 1
	}
	if maxHoursPerWeek > 0 && weeklyHoursR > maxHoursPerWeek {
		weeklyHoursR = maxHoursPerWeek
	}
	if preferredSpan >= 1 && preferredSpan <= 3 {
		return planSessionsFromSpan(weeklyHoursR, preferredSpan)
	}
	return planSessionsFromHours(weeklyHoursR)
}

func planSessionsFromHours(weeklyHours int) int {
	switch {
	case weeklyHours <= 2:
		return 1
	case weeklyHours <= 4:
		return 2
	case weeklyHours <= 6:
		return 3
	default:
		return 4
	}
}

func planSessionsFromSpan(weeklyHours, span int) int {
	if span <= 0 {
		span = 2
	}
	n := weeklyHours / span
	if n < 1 {
		n = 1
	}
	if n > 4 {
		n = 4
	}
	return n
}
