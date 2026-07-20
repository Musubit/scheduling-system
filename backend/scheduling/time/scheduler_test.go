package time_test

import (
	"context"
	"testing"

	schedtypes "scheduling-system/backend/scheduling/types"
	"scheduling-system/backend/scheduling/time"
)

func TestTimeScheduler_ProducesValidOutput(t *testing.T) {
	ts := time.New(time.DefaultConfig())
	input := schedtypes.TimeSchedulingInput{
		Tasks: []schedtypes.TeachingTaskView{
			{ID: 1, CourseID: 10, TeacherID: 100, CourseHours: 32, StartWeek: 1, EndWeek: 16, ClassGroupIDs: []uint{1}, PreferredSpan: 2},
			{ID: 2, CourseID: 20, TeacherID: 200, CourseHours: 32, StartWeek: 1, EndWeek: 16, ClassGroupIDs: []uint{2}, PreferredSpan: 2},
		},
		Teachers: []schedtypes.TeacherView{
			{ID: 100, MaxDaysPerWeek: 5},
			{ID: 200, MaxDaysPerWeek: 5},
		},
		ClassGroups: []schedtypes.ClassGroupView{
			{ID: 1, Students: 30},
			{ID: 2, Students: 30},
		},
		Constraints: []string{"teacher_preference", "course_dispersed", "teacher_days_limit"},
	}

	output, err := ts.Solve(context.Background(), input, schedtypes.NoopReporter{})
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}

	// INV: assignments 数量 >= 1
	if len(output.Assignments) == 0 {
		t.Error("expected at least 1 assignment, got 0")
	}

	// INV: 每个 assignment 的 DayOfWeek 在 [0,6]
	for i, a := range output.Assignments {
		if int(a.DayOfWeek) < 0 || int(a.DayOfWeek) > 6 {
			t.Errorf("assignment %d: DayOfWeek=%d out of range", i, a.DayOfWeek)
		}
		if a.Span < 1 || a.Span > 4 {
			t.Errorf("assignment %d: Span=%d out of range [1,4]", i, a.Span)
		}
	}

	// INV: 教师不冲突（同一教师同一时段最多一个 assignment）
	teacherSlots := make(map[uint64]bool)
	for _, a := range output.Assignments {
		for p := int(a.StartPeriod); p < int(a.StartPeriod)+a.Span; p++ {
			key := uint64(int(a.DayOfWeek))<<48 | uint64(p)<<40 | uint64(a.TeachingTaskID)
			// Find teacher for this task
			for _, task := range input.Tasks {
				if task.ID == a.TeachingTaskID {
					tKey := uint64(int(a.DayOfWeek))<<48 | uint64(p)<<40 | uint64(task.TeacherID)
					if teacherSlots[tKey] {
						t.Errorf("teacher conflict: task %d at day=%d period=%d", a.TeachingTaskID, a.DayOfWeek, p)
					}
					teacherSlots[tKey] = true
				}
			}
			_ = key
		}
	}

	// INV: ScoreDetail 的三个维度都存在
	if output.ScoreDetail.Time == nil {
		t.Error("Time bucket should not be nil")
	}
	if output.ScoreDetail.Teacher == nil {
		t.Error("Teacher bucket should not be nil")
	}
	if output.ScoreDetail.Student == nil {
		t.Error("Student bucket should not be nil")
	}

	t.Logf("Placed %d assignments, iterations=%d, elapsed=%dms",
		len(output.Assignments), output.Iterations, output.ElapsedMs)
}

func TestTimeScheduler_SeedDeterminism(t *testing.T) {
	ts := time.New(time.DefaultConfig())
	input := schedtypes.TimeSchedulingInput{
		Tasks: []schedtypes.TeachingTaskView{
			{ID: 1, CourseID: 10, TeacherID: 100, CourseHours: 32, StartWeek: 1, EndWeek: 16, ClassGroupIDs: []uint{1}, PreferredSpan: 2},
		},
		Teachers: []schedtypes.TeacherView{
			{ID: 100, MaxDaysPerWeek: 5},
		},
		ClassGroups: []schedtypes.ClassGroupView{
			{ID: 1, Students: 30},
		},
		Seed: 42,
	}

	out1, _ := ts.Solve(context.Background(), input, schedtypes.NoopReporter{})
	out2, _ := ts.Solve(context.Background(), input, schedtypes.NoopReporter{})

	// 相同 seed 应得相同输出
	if len(out1.Assignments) != len(out2.Assignments) {
		t.Errorf("seed determinism violated: %d vs %d assignments", len(out1.Assignments), len(out2.Assignments))
	}
	for i := range out1.Assignments {
		if i >= len(out2.Assignments) {
			break
		}
		if out1.Assignments[i] != out2.Assignments[i] {
			t.Errorf("assignment %d differs with same seed", i)
			break
		}
	}
}

func TestTimeScheduler_CancelContext(t *testing.T) {
	ts := time.New(time.DefaultConfig())
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	input := schedtypes.TimeSchedulingInput{
		Tasks: []schedtypes.TeachingTaskView{
			{ID: 1, CourseID: 10, TeacherID: 100, CourseHours: 32, StartWeek: 1, EndWeek: 16, ClassGroupIDs: []uint{1}},
		},
		Teachers:    []schedtypes.TeacherView{{ID: 100}},
		ClassGroups: []schedtypes.ClassGroupView{{ID: 1}},
	}

	_, err := ts.Solve(ctx, input, schedtypes.NoopReporter{})
	if err != nil {
		t.Fatalf("Solve should handle cancel gracefully, got: %v", err)
	}
	// 取消后应尽早返回（输出可能为空）
}
