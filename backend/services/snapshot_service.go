package services

import (
	"fmt"
	"sort"

	"scheduling-system/backend/database"
	"scheduling-system/backend/models"
)

// SnapshotService manages schedule validation snapshots.
type SnapshotService struct {
	db database.DB
}

func NewSnapshotService(db database.DB) *SnapshotService {
	return &SnapshotService{db: db}
}

// CreateSnapshot generates a snapshot from a scheduling result.
func (s *SnapshotService) CreateSnapshot(
	semester, dept, trigger, solver string,
	entries []models.ScheduleEntry,
	teachers []models.Teacher,
	classrooms []models.Classroom,
	constraints []string,
	solveTimeMs int64,
	conflictCount int,
) (*models.ScheduleSnapshot, error) {
	scorer := NewScoringService()
	breakdown := scorer.ScoreSchedule(entries, teachers, classrooms, constraints, nil)

	snapshot := &models.ScheduleSnapshot{
		Semester:   semester,
		Dept:       dept,
		Trigger:    trigger,
		HardPassed: conflictCount == 0,

		TotalScore:    breakdown.Total,
		TeacherPref:   breakdown.TeacherPref,
		CourseSpacing: breakdown.CourseSpacing,
		TeacherDays:   breakdown.TeacherDays,
		LowFloorPref:   breakdown.LowFloorPref,
		WeekendAvoid:   breakdown.WeekendAvoid,
		PePeriodPref:   breakdown.PePeriodPref,

		TotalEntries: len(entries),
		SolveTimeMs:  solveTimeMs,
		Solver:       solver,
	}

	// Build teacher-level details
	teacherMap := make(map[uint]models.Teacher)
	for _, t := range teachers {
		teacherMap[t.ID] = t
	}
	classroomMap := make(map[uint]models.Classroom)
	for _, c := range classrooms {
		classroomMap[c.ID] = c
	}

	// Aggregate by teacher
	type teacherAgg struct {
		entries    []models.ScheduleEntry
		earlyCount int
		lateCount  int
		days       map[models.DayOfWeek]bool
		totalFloor float64
		floorCount int
	}
	agg := make(map[uint]*teacherAgg)

	for _, e := range entries {
		t, ok := teacherMap[e.TeacherID]
		if !ok {
			continue
		}
		a, exists := agg[e.TeacherID]
		if !exists {
			a = &teacherAgg{days: make(map[models.DayOfWeek]bool)}
			agg[e.TeacherID] = a
		}
		a.entries = append(a.entries, e)
		a.days[e.DayOfWeek] = true

		if t.PreferNoEarly && e.StartPeriod <= 1 {
			a.earlyCount++
		}
		if t.PreferNoLate && e.StartPeriod >= 6 {
			a.lateCount++
		}

		if c, ok := classroomMap[e.ClassroomID]; ok {
			a.totalFloor += float64(c.Floor)
			a.floorCount++
		}
	}

	for tid, a := range agg {
		t := teacherMap[tid]
		maxDays := t.MaxDaysPerWeek
		if maxDays <= 0 {
			maxDays = 3
		}
		avgFloor := 0.0
		if a.floorCount > 0 {
			avgFloor = a.totalFloor / float64(a.floorCount)
		}

		// Build summary string
		daySlots := make(map[models.DayOfWeek][]string)
		for _, e := range a.entries {
			label := fmt.Sprintf("周%s%d-%d节", e.DayOfWeek.String(), e.StartPeriod.DisplayNum(),
				int(e.StartPeriod)+e.Span)
			daySlots[e.DayOfWeek] = append(daySlots[e.DayOfWeek], label)
		}
		summary := ""
		for d := models.Mon; d <= models.Sun; d++ {
			if slots, ok := daySlots[d]; ok {
				if summary != "" {
					summary += ","
				}
				summary += slots[0]
			}
		}

		detail := models.SnapshotDetail{
			EntityType: "teacher",
			EntityCode: t.Code,
			EntityName: t.Name,

			EarlyPenalty: float64(a.earlyCount),
			LatePenalty:  float64(a.lateCount),
			DaysActual:   len(a.days),
			DaysTarget:   maxDays,
			AvgFloor:     avgFloor,

			EntryCount: len(a.entries),
			DaysCount:  len(a.days),
			Summary:    summary,
		}
		snapshot.Details = append(snapshot.Details, detail)
	}

	// Save to DB
	if err := s.db.Create(snapshot).Error(); err != nil {
		return nil, fmt.Errorf("保存快照失败: %w", err)
	}

	return snapshot, nil
}

// GetSnapshots returns all snapshots for a semester.
func (s *SnapshotService) GetSnapshots(semester string) ([]models.ScheduleSnapshot, error) {
	var snapshots []models.ScheduleSnapshot
	if err := s.db.Where("semester = ?", semester).
		Order("created_at DESC").
		Find(&snapshots).Error(); err != nil {
		return nil, err
	}
	return snapshots, nil
}

// GetSnapshotWithDetails returns a single snapshot with its details preloaded.
func (s *SnapshotService) GetSnapshotWithDetails(id uint) (*models.ScheduleSnapshot, error) {
	var snapshot models.ScheduleSnapshot
	if err := s.db.Preload("Details").
		First(&snapshot, id).Error(); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

// CreateManualSnapshot generates a snapshot from the current schedule in the database.
func (s *SnapshotService) CreateManualSnapshot(semester string) (*models.ScheduleSnapshot, error) {
	var entries []models.ScheduleEntry
	if err := s.db.Where("semester = ?", semester).
		Preload("Course").Preload("Teacher").Preload("Classroom").
		Find(&entries).Error(); err != nil {
		return nil, fmt.Errorf("load entries: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no schedule entries for semester %s", semester)
	}

	var teachers []models.Teacher
	s.db.Find(&teachers)
	var classrooms []models.Classroom
	s.db.Find(&classrooms)

	constraints := []string{"teacher_preference", "course_dispersed", "teacher_days_limit", "low_floor_preference"}

		// Build sports course IDs
		sportsCourseIDs := make(map[uint]bool)
		for _, e := range entries {
			if models.IsSportsCourse(e.Course.Name) {
				sportsCourseIDs[e.CourseID] = true
			}
		}

	scorer := NewScoringService()
	breakdown := scorer.ScoreSchedule(entries, teachers, classrooms, constraints, sportsCourseIDs)

	conflicts := 0
	roomSlots := make(map[string]bool)
	for _, e := range entries {
		for p := e.StartPeriod; p < e.StartPeriod+models.Period(e.Span); p++ {
			key := fmt.Sprintf("r-%d-%d-%d", e.ClassroomID, e.DayOfWeek, p)
			if roomSlots[key] { conflicts++ }
			roomSlots[key] = true
		}
	}

	snapshot := &models.ScheduleSnapshot{
		Semester:      semester,
		Dept:          "全校",
		Trigger:       "manual",
		HardPassed:    conflicts == 0,
		TotalScore:    breakdown.Total,
		TeacherPref:   breakdown.TeacherPref,
		CourseSpacing: breakdown.CourseSpacing,
		TeacherDays:   breakdown.TeacherDays,
		LowFloorPref:  breakdown.LowFloorPref,
		WeekendAvoid:  breakdown.WeekendAvoid,
		PePeriodPref:  breakdown.PePeriodPref,
		TotalEntries:  len(entries),
		Solver:        "manual",
	}

	if err := s.db.Create(snapshot).Error(); err != nil {
		return nil, fmt.Errorf("save snapshot: %w", err)
	}

	return snapshot, nil
}

// DeleteSnapshot removes a snapshot and its details.
func (s *SnapshotService) DeleteSnapshot(id uint) error {
	return s.db.Delete(&models.ScheduleSnapshot{}, id).Error()
}

// SnapshotCompareResult holds the diff between two schedule snapshots.
type SnapshotCompareResult struct {
	A             *models.ScheduleSnapshot `json:"a"`
	B             *models.ScheduleSnapshot `json:"b"`
	ScoreDelta    float64                  `json:"scoreDelta"`
	ConflictDelta int                     `json:"conflictDelta"` // +1: fail->pass, -1: pass->fail, 0: unchanged
	EntryDelta    int                     `json:"entryDelta"`
	TeacherDiffs  []TeacherSnapshotDiff    `json:"teacherDiffs"`
}

// TeacherSnapshotDiff summarizes per-teacher changes between two snapshots.
type TeacherSnapshotDiff struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	EntryDelta    int     `json:"entryDelta"`
	EarlyDelta    float64 `json:"earlyDelta"`
	LateDelta     float64 `json:"lateDelta"`
	DaysActualA   int     `json:"daysActualA"`
	DaysActualB   int     `json:"daysActualB"`
	DaysTarget    int     `json:"daysTarget"`
	AvgFloorDelta float64 `json:"avgFloorDelta"`
	Status        string  `json:"status"` // improved / regressed / unchanged / added / removed
}

// teacherDetailPenalty derives a single "badness" score from a teacher detail
// (lower is better) so we can judge improvement/regression.
func teacherDetailPenalty(d models.SnapshotDetail) float64 {
	penalty := d.EarlyPenalty + d.LatePenalty
	if d.DaysActual > d.DaysTarget {
		penalty += float64(d.DaysActual - d.DaysTarget)
	}
	penalty += d.AvgFloor * 0.1
	return penalty
}

// CompareSnapshots returns a structured diff between two snapshots
// (A = baseline, B = comparison target).
func (s *SnapshotService) CompareSnapshots(aID, bID uint) (*SnapshotCompareResult, error) {
	a, err := s.GetSnapshotWithDetails(aID)
	if err != nil {
		return nil, fmt.Errorf("加载快照A失败: %w", err)
	}
	b, err := s.GetSnapshotWithDetails(bID)
	if err != nil {
		return nil, fmt.Errorf("加载快照B失败: %w", err)
	}

	res := &SnapshotCompareResult{
		A:          a,
		B:          b,
		ScoreDelta: b.TotalScore - a.TotalScore,
		EntryDelta: b.TotalEntries - a.TotalEntries,
	}
	switch {
	case a.HardPassed && !b.HardPassed:
		res.ConflictDelta = -1
	case !a.HardPassed && b.HardPassed:
		res.ConflictDelta = 1
	}

	aMap := map[string]models.SnapshotDetail{}
	for _, d := range a.Details {
		if d.EntityType == "teacher" {
			aMap[d.EntityCode] = d
		}
	}
	bMap := map[string]models.SnapshotDetail{}
	for _, d := range b.Details {
		if d.EntityType == "teacher" {
			bMap[d.EntityCode] = d
		}
	}

	seen := map[string]bool{}
	var codes []string
	for c := range aMap {
		if !seen[c] {
			seen[c] = true
			codes = append(codes, c)
		}
	}
	for c := range bMap {
		if !seen[c] {
			seen[c] = true
			codes = append(codes, c)
		}
	}
	sort.Strings(codes)

	for _, code := range codes {
		ad, aok := aMap[code]
		bd, bok := bMap[code]
		diff := TeacherSnapshotDiff{Code: code}
		if aok {
			diff.Name = ad.EntityName
			diff.DaysActualA = ad.DaysActual
			diff.DaysTarget = ad.DaysTarget
			diff.EarlyDelta -= ad.EarlyPenalty
			diff.LateDelta -= ad.LatePenalty
			diff.AvgFloorDelta -= ad.AvgFloor
			diff.EntryDelta -= ad.EntryCount
		}
		if bok {
			diff.Name = bd.EntityName
			diff.DaysActualB = bd.DaysActual
			diff.DaysTarget = bd.DaysTarget
			diff.EarlyDelta += bd.EarlyPenalty
			diff.LateDelta += bd.LatePenalty
			diff.AvgFloorDelta += bd.AvgFloor
			diff.EntryDelta += bd.EntryCount
		}
		switch {
		case !aok:
			diff.Status = "added"
		case !bok:
			diff.Status = "removed"
		case teacherDetailPenalty(bd) < teacherDetailPenalty(ad):
			diff.Status = "improved"
		case teacherDetailPenalty(bd) > teacherDetailPenalty(ad):
			diff.Status = "regressed"
		default:
			diff.Status = "unchanged"
		}
		res.TeacherDiffs = append(res.TeacherDiffs, diff)
	}
	return res, nil
}
