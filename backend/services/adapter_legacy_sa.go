package services

import (
	"context"
	"encoding/json"

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

// Solve implements schedtypes.ITimeScheduler.
// TODO(v0.6.1): The legacy SA solver (sa_solver.go) is coupled to the old ScheduleEntry model
// and won't compile after the TA+SE split. This adapter will be deleted in v0.6.1 when
// the pure scheduling/time/ implementation replaces the SA solver entirely.
// For v0.6.0, stub to return empty output so the services package compiles.
func (a *LegacySASolverAdapter) Solve(
	ctx context.Context,
	input schedtypes.TimeSchedulingInput,
	progress schedtypes.ProgressReporter,
) (schedtypes.TimeSchedulingOutput, error) {
	_ = ctx
	_ = input
	_ = progress
	return schedtypes.TimeSchedulingOutput{}, nil
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
