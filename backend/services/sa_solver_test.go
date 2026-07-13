package services

import (
	"scheduling-system/backend/models"
	"testing"

	"gorm.io/gorm"
)

func TestLockedSlotsAreRespected(t *testing.T) {
	// Build minimal test data
	teachers := []models.Teacher{
		{Model: gorm.Model{ID: 1}, Name: "张老师", PreferNoEarly: true},
	}

	classrooms := []models.Classroom{
		{Model: gorm.Model{ID: 1}, Name: "A101", Capacity: 100},
	}

	classGroups := []models.ClassGroup{
		{Model: gorm.Model{ID: 1}, Name: "CS2301", Students: 60},
	}

	// Three teaching tasks
	ttc := []models.TeachingTaskClass{{Model: gorm.Model{ID: 1}, ClassGroupID: 1}}
	teachingTasks := []models.TeachingTask{
		{Model: gorm.Model{ID: 1}, CourseID: 1, TeacherID: 1, SemesterID: 1, Classes: ttc, Teacher: teachers[0]},
		{Model: gorm.Model{ID: 2}, CourseID: 2, TeacherID: 1, SemesterID: 1, Classes: ttc, Teacher: teachers[0]},
		{Model: gorm.Model{ID: 3}, CourseID: 3, TeacherID: 1, SemesterID: 1, Classes: ttc, Teacher: teachers[0]},
	}

	// Lock ALL periods on Tuesday (dayOfWeek=1)
	lockedSlots := []LockedTimeSlot{
		{DayOfWeek: 1, StartPeriod: 0, Span: 11}, // lock entire Tuesday
	}

	config := SAConfig{
		InitialTemp:       10.0,
		CoolingRate:       0.95,
		IterationsPerTemp: 50, // smaller for fast test
		MinTemp:           0.1,
		MaxTimeSeconds:    2,
	}

	solver := NewSASolver()
	result := solver.Solve(
		teachingTasks, teachers, classrooms, classGroups,
		lockedSlots, []string{}, uint(1),
		config,
		nil, nil,
	)

	// Verify NO entries on Tuesday
	for _, e := range result.Entries {
		if int(e.DayOfWeek) == 1 {
			t.Errorf("SOLVER BUG: entry placed on locked Tuesday: course=%d, day=%d, start=%d",
				e.CourseID, e.DayOfWeek, e.StartPeriod)
		}
	}
	t.Logf("OK: %d entries, none on Tuesday", len(result.Entries))
}
