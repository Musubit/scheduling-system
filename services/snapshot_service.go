package services

import (
	"fmt"
	"scheduling-system/database"
	"scheduling-system/models"
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
	breakdown := scorer.ScoreSchedule(entries, teachers, classrooms, constraints)

	snapshot := &models.ScheduleSnapshot{
		Semester:   semester,
		Dept:       dept,
		Trigger:    trigger,
		HardPassed: conflictCount == 0,

		TotalScore:    breakdown.Total,
		TeacherPref:   breakdown.TeacherPref,
		CourseSpacing: breakdown.CourseSpacing,
		TeacherDays:   breakdown.TeacherDays,
		LowFloorPref:  breakdown.LowFloorPref,

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

// CreateManualSnapshot generates a snapshot on user request (after micro-adjustment).
func (s *SnapshotService) CreateManualSnapshot(
	semester, dept string,
	entries []models.ScheduleEntry,
	teachers []models.Teacher,
	classrooms []models.Classroom,
	constraints []string,
	conflictCount int,
) (*models.ScheduleSnapshot, error) {
	return s.CreateSnapshot(semester, dept, "manual", "simulated_annealing",
		entries, teachers, classrooms, constraints, 0, conflictCount)
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

// DeleteSnapshot removes a snapshot and its details.
func (s *SnapshotService) DeleteSnapshot(id uint) error {
	return s.db.Delete(&models.ScheduleSnapshot{}, id).Error()
}
