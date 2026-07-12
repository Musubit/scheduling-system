package services

import (
	"fmt"
	"scheduling-system/backend/models"
	"testing"
	"time"

	"gorm.io/gorm"
)

// buildBenchmarkFixture creates a moderately sized fixture (~20 teachers,
// ~10 classrooms, ~12 class groups, ~30 teaching tasks) resembling the
// HBUT Vocational Teacher College scale for benchmarking SA performance.
func buildBenchmarkFixture() (
	[]models.TeachingTask,
	[]models.Teacher,
	[]models.Classroom,
	[]models.ClassGroup,
) {
	teachers := make([]models.Teacher, 20)
	for i := 0; i < 20; i++ {
		t := models.Teacher{
			Model:          gorm.Model{ID: uint(i + 1)},
			Name:           fmt.Sprintf("教师%d", i+1),
			MaxDaysPerWeek: 3,
		}
		if i%3 == 0 {
			t.PreferNoEarly = true
		}
		if i%5 == 0 {
			t.PreferNoLate = true
		}
		if i%4 == 0 {
			t.PreferLowFloor = true
		}
		teachers[i] = t
	}

	classrooms := make([]models.Classroom, 10)
	for i := 0; i < 10; i++ {
		classrooms[i] = models.Classroom{
			Model:    gorm.Model{ID: uint(i + 1)},
			Name:     fmt.Sprintf("A%d0%d", (i/5)+1, (i%5)+1),
			Building: "A",
			Floor:    (i / 3) + 1,
			Capacity: 80,
			Type:     "普通教室",
		}
	}

	classGroups := make([]models.ClassGroup, 12)
	for i := 0; i < 12; i++ {
		classGroups[i] = models.ClassGroup{
			Model:    gorm.Model{ID: uint(i + 1)},
			Name:     fmt.Sprintf("Class%d", i+1),
			Students: 40,
		}
	}

	// 30 teaching tasks: distributed across teachers/classes, mix of hour budgets
	tasks := make([]models.TeachingTask, 30)
	for i := 0; i < 30; i++ {
		teacherID := uint((i % 20) + 1)
		classID := uint((i % 12) + 1)
		hours := 32 // 2 hrs/wk over 16 weeks → span=2, 1 session
		if i%7 == 0 {
			hours = 48 // 3 hrs/wk → span=3, evening
		} else if i%5 == 0 {
			hours = 64 // 4 hrs/wk → [2,2]
		}
		tasks[i] = models.TeachingTask{
			Model:      gorm.Model{ID: uint(i + 1)},
			CourseID:   uint(i + 1),
			TeacherID:  teacherID,
			SemesterID: 1,
			TotalHours: hours,
			StartWeek:  1,
			EndWeek:    16,
			Teacher:    teachers[teacherID-1],
			Course: models.Course{
				Model: gorm.Model{ID: uint(i + 1)},
				Name:  fmt.Sprintf("课程%d", i+1),
				Hours: hours,
			},
			Classes: []models.TeachingTaskClass{{
				Model:        gorm.Model{ID: uint(i + 1)},
				ClassGroupID: classID,
				ClassGroup:   classGroups[classID-1],
			}},
		}
	}
	return tasks, teachers, classrooms, classGroups
}

// BenchmarkSASolve measures iterations/second for a fixed SA run.
// v0.5.2 baseline (pre Goal 3 optimization) — run with:
//
//	go test -run '^$' -bench BenchmarkSASolve -benchtime=1x ./backend/services
func BenchmarkSASolve(b *testing.B) {
	tasks, teachers, classrooms, classGroups := buildBenchmarkFixture()
	config := SAConfig{
		InitialTemp:       10.0,
		CoolingRate:       0.95,
		IterationsPerTemp: 200,
		MinTemp:           0.1,
		MaxTimeSeconds:    5,
		Seed:              42,
	}
	solver := NewSASolver()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		start := time.Now()
		result := solver.Solve(tasks, teachers, classrooms, classGroups,
			nil, FullDefaultConstraints(), "2025-S2", config, nil, nil)
		elapsed := time.Since(start).Seconds()
		if elapsed > 0 {
			b.ReportMetric(float64(result.Iterations)/elapsed, "iter/s")
		}
		b.ReportMetric(float64(result.Score), "score")
		b.ReportMetric(float64(result.Scheduled), "entries")
	}
}
