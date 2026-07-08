package services

import (
	"math"
	"scheduling-system/models"
)

// ScoringService evaluates a schedule's quality against soft constraints.
type ScoringService struct{}

func NewScoringService() *ScoringService {
	return &ScoringService{}
}

// ScoreBreakdown holds the detailed scoring result.
type ScoreBreakdown struct {
	Total         float64 `json:"total"`
	TeacherPref   float64 `json:"teacherPref"`   // 教师偏好满足度
	CourseSpacing float64 `json:"courseSpacing"` // 课程间隔均匀度
	TeacherDays   float64 `json:"teacherDays"`   // 教师到校天数
	LowFloorPref  float64 `json:"lowFloorPref"`  // 优先低楼层
	WeekendAvoid  float64 `json:"weekendAvoid"`  // 周末避让
	PePeriodPref  float64 `json:"pePeriodPref"`  // 体育课时段偏好
}

// ScoreSchedule evaluates a full schedule against soft constraints.
// enabledConstraints: list of constraint keys to evaluate. If empty, all are enabled.
// sportsCourseIDs: set of course IDs that are sports courses (for pe_preferred_periods).
// Returns a score from 0-100 with detailed breakdown.
func (s *ScoringService) ScoreSchedule(entries []models.ScheduleEntry, teachers []models.Teacher, classrooms []models.Classroom, enabledConstraints []string, sportsCourseIDs map[uint]bool) ScoreBreakdown {
	breakdown := ScoreBreakdown{}

	if len(enabledConstraints) == 0 {
		// Default: enable all
		enabledConstraints = []string{"teacher_preference", "course_dispersed", "teacher_days_limit", "low_floor_preference"}
	}

	enabled := make(map[string]bool)
	for _, c := range enabledConstraints {
		enabled[c] = true
	}

	// Count enabled categories and per-category max
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
	perCategoryMax := 100.0 / float64(enabledCount)
	if enabledCount == 0 {
		perCategoryMax = 25.0
	}

	// Build lookup maps
	teacherMap := make(map[uint]models.Teacher)
	for _, t := range teachers {
		teacherMap[t.ID] = t
	}
	classroomMap := make(map[uint]models.Classroom)
	for _, c := range classrooms {
		classroomMap[c.ID] = c
	}

	// 1. Teacher preference scoring
	if enabled["teacher_preference"] {
		breakdown.TeacherPref = s.scoreTeacherPreferences(entries, teacherMap, perCategoryMax)
	}

	// 2. Course spacing scoring
	if enabled["course_dispersed"] {
		breakdown.CourseSpacing = s.scoreCourseSpacing(entries, perCategoryMax)
	}

	// 3. Teacher days per week scoring
	if enabled["teacher_days_limit"] {
		breakdown.TeacherDays = s.scoreTeacherDays(entries, teacherMap, perCategoryMax)
	}

	// 4. Low floor preference scoring
	if enabled["low_floor_preference"] {
		breakdown.LowFloorPref = s.scoreLowFloorPref(entries, teacherMap, classroomMap, perCategoryMax)
	}

	// 5. Weekend avoidance scoring
	if enabled["avoid_saturday"] || enabled["avoid_sunday"] {
		breakdown.WeekendAvoid = s.scoreWeekendAvoid(entries, enabled, perCategoryMax)
	}

	// 6. Sports period preference
	if enabled["pe_preferred_periods"] && sportsCourseIDs != nil {
		breakdown.PePeriodPref = s.scorePePeriodPref(entries, sportsCourseIDs, perCategoryMax)
	}

	breakdown.Total = math.Round((breakdown.TeacherPref + breakdown.CourseSpacing + breakdown.TeacherDays + breakdown.LowFloorPref + breakdown.WeekendAvoid + breakdown.PePeriodPref)*100) / 100
	return breakdown
}

// scoreTeacherPreferences evaluates how well teacher time preferences are met.
// Each teacher with PreferNoEarly should not have entries in periods 0-1 (第1-2节).
// Each teacher with PreferNoLate should not have entries in periods 6+ (第7节及以后).
func (s *ScoringService) scoreTeacherPreferences(entries []models.ScheduleEntry, teacherMap map[uint]models.Teacher, maxScore float64) float64 {
	type teacherStat struct {
		hasPref    bool
		earlyCount int
		lateCount  int
		totalCount int
	}
	stats := make(map[uint]*teacherStat)

	for _, e := range entries {
		t, ok := teacherMap[e.TeacherID]
		if !ok {
			continue
		}
		st, exists := stats[e.TeacherID]
		if !exists {
			st = &teacherStat{
				hasPref: t.PreferNoEarly || t.PreferNoLate,
			}
			stats[e.TeacherID] = st
		}
		st.totalCount++
		if t.PreferNoEarly && e.StartPeriod <= 1 {
			st.earlyCount++
		}
		if t.PreferNoLate && e.StartPeriod >= 6 {
			st.lateCount++
		}
	}

	if len(stats) == 0 {
		return maxScore
	}

	totalPenalty := 0.0
	prefTeacherCount := 0
	for _, st := range stats {
		if !st.hasPref {
			continue
		}
		prefTeacherCount++
		if st.totalCount > 0 {
			penalty := float64(st.earlyCount+st.lateCount) / float64(st.totalCount)
			totalPenalty += penalty
		}
	}

	if prefTeacherCount == 0 {
		return maxScore
	}

	// Average across preference teachers, scale to 0-maxScore
	avgPenalty := totalPenalty / float64(prefTeacherCount)
	score := maxScore * (1.0 - avgPenalty)
	if score < 0 {
		score = 0
	}
	return math.Round(score*100) / 100
}

// scoreCourseSpacing evaluates how evenly each course's sessions are distributed
// across the week. Courses concentrated on fewer days get lower scores.
func (s *ScoringService) scoreCourseSpacing(entries []models.ScheduleEntry, maxScore float64) float64 {
	type courseInfo struct {
		days      map[int]bool
		count     int
	}
	courses := make(map[uint]*courseInfo)

	for _, e := range entries {
		ci, exists := courses[e.CourseID]
		if !exists {
			ci = &courseInfo{days: make(map[int]bool)}
			courses[e.CourseID] = ci
		}
		ci.days[int(e.DayOfWeek)] = true
		ci.count++
	}

	if len(courses) == 0 {
		return maxScore
	}

	totalScore := 0.0
	for _, ci := range courses {
		if ci.count <= 1 {
			totalScore += 1.0 // single-session courses are fine
			continue
		}
		// Ideal: each session on a different day
		// Score = unique_days / total_sessions (capped at 1.0)
		ratio := float64(len(ci.days)) / float64(ci.count)
		if ratio > 1.0 {
			ratio = 1.0
		}
		totalScore += ratio
	}

	avgScore := totalScore / float64(len(courses))
	return math.Round(maxScore*avgScore*100) / 100
}

// scoreTeacherDays evaluates how many distinct days each teacher comes to campus.
// Target: ≤3 days per week = full score. Each extra day reduces score.
func (s *ScoringService) scoreTeacherDays(entries []models.ScheduleEntry, teacherMap map[uint]models.Teacher, maxScore float64) float64 {
	teacherDays := make(map[uint]map[int]bool)

	for _, e := range entries {
		days, exists := teacherDays[e.TeacherID]
		if !exists {
			days = make(map[int]bool)
			teacherDays[e.TeacherID] = days
		}
		days[int(e.DayOfWeek)] = true
	}

	if len(teacherDays) == 0 {
		return maxScore
	}

	totalScore := 0.0
	for tid, days := range teacherDays {
		actualDays := len(days)
		maxDays := 3 // default
		if t, ok := teacherMap[tid]; ok && t.MaxDaysPerWeek > 0 {
			maxDays = t.MaxDaysPerWeek
		}
		if actualDays <= maxDays {
			totalScore += 1.0
		} else {
			// Penalty: each extra day reduces score proportionally
			extra := actualDays - maxDays
			penalty := float64(extra) / float64(maxDays)
			score := 1.0 - penalty
			if score < 0 {
				score = 0
			}
			totalScore += score
		}
	}

	avgScore := totalScore / float64(len(teacherDays))
	return math.Round(maxScore*avgScore*100) / 100
}

// scoreLowFloorPref evaluates whether teachers who prefer low floors
// are assigned to classrooms on lower floors.
func (s *ScoringService) scoreLowFloorPref(entries []models.ScheduleEntry, teacherMap map[uint]models.Teacher, classroomMap map[uint]models.Classroom, maxScore float64) float64 {
	type floorStat struct {
		totalFloor   float64
		count        int
	}
	stats := make(map[uint]*floorStat)

	for _, e := range entries {
		t, ok := teacherMap[e.TeacherID]
		if !ok || !t.PreferLowFloor {
			continue
		}
		c, ok := classroomMap[e.ClassroomID]
		if !ok {
			continue
		}
		st, exists := stats[e.TeacherID]
		if !exists {
			st = &floorStat{}
			stats[e.TeacherID] = st
		}
		st.totalFloor += float64(c.Floor)
		st.count++
	}

	if len(stats) == 0 {
		return maxScore
	}

	// Assume max floor across all classrooms for normalization
	maxFloor := 1
	for _, c := range classroomMap {
		if c.Floor > maxFloor {
			maxFloor = c.Floor
		}
	}
	if maxFloor <= 1 {
		return maxScore
	}

	totalScore := 0.0
	for _, st := range stats {
		avgFloor := st.totalFloor / float64(st.count)
		// Score: floor 1 = 1.0, floor maxFloor = 0.0
		score := 1.0 - (avgFloor-1.0)/float64(maxFloor-1)
		if score < 0 {
			score = 0
		}
		if score > 1.0 {
			score = 1.0
		}
		totalScore += score
	}

	avgScore := totalScore / float64(len(stats))
	return math.Round(maxScore*avgScore*100) / 100
}

// scoreWeekendAvoid penalizes entries placed on Saturday and/or Sunday.
// Full score = no weekend entries. Each weekend entry reduces the score proportionally.
func (s *ScoringService) scoreWeekendAvoid(entries []models.ScheduleEntry, enabled map[string]bool, maxScore float64) float64 {
	if len(entries) == 0 {
		return maxScore
	}

	avoidSaturday := enabled["avoid_saturday"]
	avoidSunday := enabled["avoid_sunday"]

	saturdayCount := 0
	sundayCount := 0
	for _, e := range entries {
		if avoidSaturday && e.DayOfWeek == models.Sat {
			saturdayCount++
		}
		if avoidSunday && e.DayOfWeek == models.Sun {
			sundayCount++
		}
	}

	totalWeekend := saturdayCount + sundayCount
	if totalWeekend == 0 {
		return maxScore
	}

	// Penalty: each weekend entry loses 1/N of the max score, where N is total entries
	penalty := float64(totalWeekend) / float64(len(entries))
	score := maxScore * (1.0 - penalty)
	if score < 0 {
		score = 0
	}
	return math.Round(score*100) / 100
}

// scorePePeriodPref evaluates whether sports courses are placed at preferred periods
// (3-4节 = startPeriod 2, 7-8节 = startPeriod 6).
func (s *ScoringService) scorePePeriodPref(entries []models.ScheduleEntry, sportsCourseIDs map[uint]bool, maxScore float64) float64 {
	preferredStarts := map[int]bool{2: true, 6: true} // startPeriod 2 (3-4节), 6 (7-8节)

	sportsCount := 0
	preferredCount := 0

	for _, e := range entries {
		if !sportsCourseIDs[e.CourseID] {
			continue
		}
		sportsCount++
		if preferredStarts[int(e.StartPeriod)] {
			preferredCount++
		}
	}

	if sportsCount == 0 {
		return maxScore
	}

	ratio := float64(preferredCount) / float64(sportsCount)
	return math.Round(maxScore * ratio * 100) / 100
}
