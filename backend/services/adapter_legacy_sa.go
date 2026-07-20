package services

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
	"scheduling-system/backend/models"
	schedtypes "scheduling-system/backend/scheduling/types"
)

// LegacySASolverAdapter bridges the new scheduling types.ITimeScheduler interface
// to the old SA solver. This is a v0.6.0 temporary adapter — it will be deleted
// in v0.6.1 when the pure scheduling/time/ implementation replaces it.
type LegacySASolverAdapter struct{}

// NewLegacySASolverAdapter creates a new adapter instance.
func NewLegacySASolverAdapter() *LegacySASolverAdapter {
	return &LegacySASolverAdapter{}
}

// Compile-time interface check.
var _ schedtypes.ITimeScheduler = (*LegacySASolverAdapter)(nil)

// Solve implements schedtypes.ITimeScheduler by converting the pure-type inputs
// back into models, building virtual classrooms, and delegating to the old
// SA solver via SolveMultiRun.
func (a *LegacySASolverAdapter) Solve(
	ctx context.Context,
	input schedtypes.TimeSchedulingInput,
	progress schedtypes.ProgressReporter,
) (schedtypes.TimeSchedulingOutput, error) {
	// Fast path: zero tasks.
	if len(input.Tasks) == 0 {
		return schedtypes.TimeSchedulingOutput{}, nil
	}

	// ProgressReporter defensively replaced with no-op if nil.
	if progress == nil {
		progress = schedtypes.NoopReporter{}
	}

	// 1. Convert pure-type views → GORM models.
	tasks := convertTaskViewsToModels(input.Tasks, input.Teachers, input.SemesterID)
	teachers := convertTeacherViewsToModels(input.Teachers)
	classGroups := convertClassGroupViewsToModels(input.ClassGroups)
	lockedSlots := convertLockedSlots(input.LockedSlots)

	// 2. Build virtual classrooms (TIME_ONLY mode).
	classrooms := buildVirtualClassroomsForTimeOnly(tasks, classGroups)

	// 3. Configure SA solver.
	saConfig := defaultSAConfig()
	saConfig.TimeOnly = true
	if !input.Deadline.IsZero() {
		if remaining := time.Until(input.Deadline).Seconds(); remaining > 0 {
			saConfig.MaxTimeSeconds = remaining
		}
	}
	if input.Seed != 0 {
		saConfig.Seed = input.Seed
	}
	if input.ConstraintWeights != nil {
		saConfig.ConstraintWeights = input.ConstraintWeights
	}

	// 4. Bridge ProgressReporter → legacy progressFn callback.
	progressFn := func(iter, total int, currentScore, bestScore, temp float64) {
		progress.Iteration(iter, total, currentScore, bestScore, temp)
	}
	progress.Stage("SA Solve (legacy adapter)", 0)

	// 5. Derive cancel channel from context.
	var cancelCh <-chan struct{}
	if ctx != nil {
		cancelCh = ctx.Done()
	}

	// 6. Run the old SA solver (multi-restart, 3 runs).
	saResult := NewSASolver().SolveMultiRun(
		tasks, teachers, classrooms, classGroups,
		lockedSlots, input.Constraints, input.SemesterID,
		saConfig, 3, cancelCh, progressFn,
	)

	// 7. Extract time-only assignments from ScheduleEntry results.
	assignments := make([]schedtypes.TimeAssignmentDraft, 0, len(saResult.Entries))
	for _, entry := range saResult.Entries {
		if entry.TeachingTaskID == nil {
			continue
		}
		assignments = append(assignments, schedtypes.TimeAssignmentDraft{
			TeachingTaskID: *entry.TeachingTaskID,
			DayOfWeek:      schedtypes.DayOfWeek(entry.DayOfWeek),
			StartPeriod:    schedtypes.Period(entry.StartPeriod),
			Span:           entry.Span,
		})
	}

	return schedtypes.TimeSchedulingOutput{
		Assignments: assignments,
		Iterations:  saResult.Iterations,
		ElapsedMs:   saResult.ElapsedMs,
	}, nil
}

// convertTaskViewsToModels converts TeachingTaskView slices into models.TeachingTask
// slices suitable for the old SA solver. Teacher preferences are embedded by
// looking up the corresponding TeacherView.
func convertTaskViewsToModels(
	views []schedtypes.TeachingTaskView,
	teacherViews []schedtypes.TeacherView,
	semesterID uint,
) []models.TeachingTask {
	teacherMap := make(map[uint]schedtypes.TeacherView, len(teacherViews))
	for _, tv := range teacherViews {
		teacherMap[tv.ID] = tv
	}

	tasks := make([]models.TeachingTask, len(views))
	for i, v := range views {
		classes := make([]models.TeachingTaskClass, len(v.ClassGroupIDs))
		for j, cgID := range v.ClassGroupIDs {
			classes[j] = models.TeachingTaskClass{
				TeachingTaskID: v.ID,
				ClassGroupID:   cgID,
			}
		}

		task := models.TeachingTask{
			Model:            gorm.Model{ID: v.ID},
			CourseID:         v.CourseID,
			TeacherID:        v.TeacherID,
			SemesterID:       semesterID,
			TotalHours:       v.CourseHours,
			StartWeek:        v.StartWeek,
			EndWeek:          v.EndWeek,
			MaxHoursPerWeek:  v.MaxHoursPerWeek,
			PreferredSpan:    v.PreferredSpan,
			RequiredRoomType: v.RequiredRoomType,
			Course: models.Course{
				Model: gorm.Model{ID: v.CourseID},
				Name:  v.CourseName,
				Hours: v.CourseHours,
			},
			Classes: classes,
		}

		if tv, ok := teacherMap[v.TeacherID]; ok {
			task.Teacher = models.Teacher{
				Model:          gorm.Model{ID: tv.ID},
				Name:           tv.Name,
				PreferNoEarly:  tv.PreferNoEarly,
				PreferNoLate:   tv.PreferNoLate,
				MaxDaysPerWeek: tv.MaxDaysPerWeek,
				PreferLowFloor: tv.PreferLowFloor,
			}
		}

		tasks[i] = task
	}
	return tasks
}

// convertTeacherViewsToModels converts TeacherView slices into models.Teacher
// slices. UnavailableSlots are marshalled to JSON for the string field in the
// model, matching the format expected by the SA solver's internal unmarshalling.
func convertTeacherViewsToModels(views []schedtypes.TeacherView) []models.Teacher {
	teachers := make([]models.Teacher, len(views))
	for i, v := range views {
		var unavailableJSON string
		if len(v.UnavailableSlots) > 0 {
			svcSlots := make([]LockedTimeSlot, len(v.UnavailableSlots))
			for j, s := range v.UnavailableSlots {
				svcSlots[j] = LockedTimeSlot{
					DayOfWeek:   models.DayOfWeek(s.DayOfWeek),
					StartPeriod: models.Period(s.StartPeriod),
					Span:        s.Span,
				}
			}
			b, _ := json.Marshal(svcSlots)
			unavailableJSON = string(b)
		}
		teachers[i] = models.Teacher{
			Model:            gorm.Model{ID: v.ID},
			Name:             v.Name,
			PreferNoEarly:    v.PreferNoEarly,
			PreferNoLate:     v.PreferNoLate,
			MaxDaysPerWeek:   v.MaxDaysPerWeek,
			PreferLowFloor:   v.PreferLowFloor,
			UnavailableSlots: unavailableJSON,
		}
	}
	return teachers
}

// convertClassGroupViewsToModels converts ClassGroupView slices into
// models.ClassGroup slices.
func convertClassGroupViewsToModels(views []schedtypes.ClassGroupView) []models.ClassGroup {
	groups := make([]models.ClassGroup, len(views))
	for i, v := range views {
		groups[i] = models.ClassGroup{
			Model:    gorm.Model{ID: v.ID},
			Name:     v.Name,
			Students: v.Students,
		}
	}
	return groups
}

// convertLockedSlots converts types.LockedTimeSlot → services.LockedTimeSlot.
// Both use int-backed type aliases (types.DayOfWeek / models.DayOfWeek and
// types.Period / models.Period), so the conversion is an explicit type cast.
func convertLockedSlots(slots []schedtypes.LockedTimeSlot) []LockedTimeSlot {
	result := make([]LockedTimeSlot, len(slots))
	for i, s := range slots {
		result[i] = LockedTimeSlot{
			DayOfWeek:   models.DayOfWeek(s.DayOfWeek),
			StartPeriod: models.Period(s.StartPeriod),
			Span:        s.Span,
		}
	}
	return result
}
