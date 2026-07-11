package services

import (
	"scheduling-system/backend/models"
	"testing"
)

// TestSASeedLikePlacesAll mirrors the seed's 22 teaching tasks and verifies the
// SA solver places every weekly session (no silent drops) with zero hard conflicts.
func TestSASeedLikePlacesAll(t *testing.T) {
	teachers := make([]models.Teacher, 13)
	for i := range teachers {
		teachers[i].ID = uint(i + 1)
	}
	classrooms := []models.Classroom{
		{Capacity: 80}, {Capacity: 90}, {Capacity: 60}, {Capacity: 100},
		{Capacity: 70}, {Capacity: 100}, {Capacity: 120}, {Capacity: 80},
		{Capacity: 200}, {Capacity: 50}, {Capacity: 300},
	}
	for i := range classrooms {
		classrooms[i].ID = uint(i + 1)
	}
	groups := []models.ClassGroup{
		{Students: 86}, {Students: 82}, {Students: 72}, {Students: 68},
		{Students: 55}, {Students: 78}, {Students: 40},
	}
	for i := range groups {
		groups[i].ID = uint(i + 1)
	}
	specs := [][4]int{
		{18, 5, 1, 80}, {9, 7, 1, 64}, {16, 4, 6, 48}, {23, 11, 1, 32}, {19, 5, 2, 48}, {3, 2, 4, 64},
		{20, 8, 4, 64}, {14, 6, 6, 48}, {10, 7, 2, 64}, {6, 8, 5, 64}, {21, 9, 7, 32}, {12, 10, 7, 48},
		{1, 1, 3, 64}, {7, 12, 5, 64}, {11, 7, 1, 48}, {15, 6, 6, 48}, {4, 2, 4, 48}, {17, 4, 7, 32},
		{5, 3, 3, 64}, {22, 9, 7, 16}, {13, 13, 7, 48}, {2, 1, 3, 48},
	}
	var tasks []models.TeachingTask
	expectedSessions := 0
	for i, sp := range specs {
		hours := sp[3]
		expectedSessions += (hours + 31) / 32
		tasks = append(tasks, models.TeachingTask{
			TeacherID:  uint(sp[1]),
			CourseID:   uint(sp[0]),
			TotalHours: hours,
			Classes:    []models.TeachingTaskClass{{ClassGroupID: uint(sp[2])}},
		})
		tasks[i].ID = uint(i + 1)
	}

	cfg := defaultSAConfig()
	cfg.MaxTimeSeconds = 15
	cfg.Seed = 12345
	solver := NewSASolver()
	result := solver.SolveMultiRun(tasks, teachers, classrooms, groups, nil, []string{}, "2025-S2", cfg, 3, nil, nil)

	if len(result.Entries) < expectedSessions {
		t.Errorf("SA silently dropped sessions: placed %d, expected %d", len(result.Entries), expectedSessions)
	}

	// Hard-conflict check
	tc, rc, cc := 0, 0, 0
	overlap := func(a, b models.ScheduleEntry) bool {
		return int(a.StartPeriod) < int(b.StartPeriod)+b.Span && int(b.StartPeriod) < int(a.StartPeriod)+a.Span
	}
	for i := range result.Entries {
		e := result.Entries[i]
		for j := i + 1; j < len(result.Entries); j++ {
			f := result.Entries[j]
			if e.DayOfWeek != f.DayOfWeek || !overlap(e, f) {
				continue
			}
			if e.ClassroomID == f.ClassroomID {
				rc++
			}
			if e.TeacherID == f.TeacherID {
				tc++
			}
			if e.ClassGroupID != nil && f.ClassGroupID != nil && *e.ClassGroupID == *f.ClassGroupID {
				cc++
			}
		}
	}
	if tc+rc+cc > 0 {
		t.Errorf("SA produced hard conflicts: teacher=%d room=%d class=%d", tc, rc, cc)
	}
	t.Logf("OK: SA placed %d/%d sessions, conflicts t=%d r=%d c=%d, score=%.1f",
		len(result.Entries), expectedSessions, tc, rc, cc, result.Score)
}
