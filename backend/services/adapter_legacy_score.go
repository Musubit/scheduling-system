package services

import (
	"fmt"

	"gorm.io/gorm"
	"scheduling-system/backend/models"
	schedtypes "scheduling-system/backend/scheduling/types"
)

// LegacyScorerAdapter implements schedtypes.IScorer by reconstructing
// models.ScheduleEntry and models.TeachingTask slices from the new type
// system, then delegating to the old ScoringService.ScoreSchedule. This
// is a v0.6.0 temporary adapter — it will be deleted in v0.6.1 when
// the pure scheduling/scoring/ implementation replaces it.
type LegacyScorerAdapter struct {
	scoringService *ScoringService
}

// NewLegacyScorerAdapter creates a new adapter instance.
func NewLegacyScorerAdapter() *LegacyScorerAdapter {
	return &LegacyScorerAdapter{scoringService: NewScoringService()}
}

// Compile-time interface check.
var _ schedtypes.IScorer = (*LegacyScorerAdapter)(nil)

// Score implements schedtypes.IScorer by converting the new-type inputs
// back to old models, building a ScoringContext, and calling the old
// ScoringService.ScoreSchedule. The result is converted back to the
// new schedtypes.ScoreBreakdown.
func (s *LegacyScorerAdapter) Score(
	assignments []schedtypes.TimeAssignmentDraft,
	allocations []schedtypes.RoomAllocationDraft,
	teachers []schedtypes.TeacherView,
	classrooms []schedtypes.ClassroomView,
	tasks []schedtypes.TeachingTaskView,
	dims []string,
) schedtypes.ScoreBreakdown {
	// --- Build lookup maps ---

	// Allocation map keyed by LocalRef (the index into assignments).
	allocMap := make(map[int]schedtypes.RoomAllocationDraft, len(allocations))
	for _, a := range allocations {
		allocMap[a.LocalRef] = a
	}

	taskMap := make(map[uint]schedtypes.TeachingTaskView, len(tasks))
	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	// --- 1. Reconstruct []models.ScheduleEntry from drafts ---

	entries := make([]models.ScheduleEntry, 0, len(assignments))
	for i, asgn := range assignments {
		task, ok := taskMap[asgn.TeachingTaskID]
		if !ok {
			continue
		}

		taskID := task.ID // capture for pointer
		entry := models.ScheduleEntry{
			CourseID:       task.CourseID,
			TeacherID:      task.TeacherID,
			TeachingTaskID: &taskID,
			DayOfWeek:      models.DayOfWeek(asgn.DayOfWeek),
			StartPeriod:    models.Period(asgn.StartPeriod),
			Span:           asgn.Span,
			Weeks:          fmt.Sprintf("%d-%d", task.StartWeek, task.EndWeek),
		}

		// Lookup allocation by LocalRef for ClassroomID.
		if alloc, ok := allocMap[i]; ok {
			entry.ClassroomID = alloc.ClassroomID
		}

		entries = append(entries, entry)
	}

	// --- 2. Convert views → models ---

	teacherModels := convertTeacherViewsToModels(teachers)
	classroomModels := convertClassroomViewsToModels(classrooms)
	taskModels := convertTaskViewsToModels(tasks, teachers, 0) // semesterID=0 unused by scoring

	// --- 3. Build sportsCourseIDs from tasks where IsSports ---

	sportsCourseIDs := make(map[uint]bool)
	for _, t := range tasks {
		if t.IsSports {
			sportsCourseIDs[t.CourseID] = true
		}
	}

	// --- 4. Derive mode from dims ---

	mode := schedtypes.ModeTimeOnlyScheduling
	for _, d := range dims {
		if d == "resource" {
			mode = schedtypes.ModeFullScheduling
			break
		}
	}

	// --- 5. Build ScoringContext and call old scorer ---

	ctx := NewScoringContext(dims, sportsCourseIDs, taskModels).WithMode(mode)
	oldBreakdown := s.scoringService.ScoreSchedule(entries, teacherModels, classroomModels, ctx)

	// --- 6. Convert old ScoreBreakdown → new schedtypes.ScoreBreakdown ---

	return schedtypes.ScoreBreakdown{
		Time:              convertScoreBucket(oldBreakdown.Buckets.Time),
		Teacher:           convertScoreBucket(oldBreakdown.Buckets.Teacher),
		Student:           convertScoreBucket(oldBreakdown.Buckets.Student),
		Resource:          convertScoreBucket(oldBreakdown.Buckets.Resource),
		EnabledDimensions: oldBreakdown.EnabledDimensions,
		PerBucketMax:      oldBreakdown.PerCategoryMax,
		PlacedSessions:    oldBreakdown.PlacedSessions,
		ExpectedSessions:  oldBreakdown.ExpectedSessions,
		Completeness:      oldBreakdown.Completeness,
		Total:             oldBreakdown.Total,
		FinalTotal:        oldBreakdown.FinalTotal,
	}
}

// convertScoreBucket converts a services.ScoreBucket pointer to a
// schedtypes.ScoreBucket pointer. Nil in → nil out.
func convertScoreBucket(b *ScoreBucket) *schedtypes.ScoreBucket {
	if b == nil {
		return nil
	}
	return &schedtypes.ScoreBucket{
		Value:   b.Value,
		Max:     b.Max,
		Details: b.Details,
	}
}

// convertClassroomViewsToModels converts ClassroomView slices into
// models.Classroom slices suitable for the old scorer. Equipment is
// passed through verbatim; the old scorer only uses Floor.
func convertClassroomViewsToModels(views []schedtypes.ClassroomView) []models.Classroom {
	classrooms := make([]models.Classroom, len(views))
	for i, v := range views {
		classrooms[i] = models.Classroom{
			Model:    gorm.Model{ID: v.ID},
			Floor:    v.Floor,
			Capacity: v.Capacity,
			RoomType: v.Type,
			Equipment: v.Equipment,
		}
	}
	return classrooms
}
