package services

import (
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

// Score implements schedtypes.IScorer.
// TODO(v0.6.1): The legacy scorer (scoring_service.go) is coupled to the old ScheduleEntry model
// and won't compile after the TA+SE split. This adapter will be deleted in v0.6.1 when
// the pure scheduling/scoring/ implementation replaces it.
// For v0.6.0, stub to return zero ScoreBreakdown so the services package compiles.
func (s *LegacyScorerAdapter) Score(
	assignments []schedtypes.TimeAssignmentDraft,
	allocations []schedtypes.RoomAllocationDraft,
	teachers []schedtypes.TeacherView,
	classrooms []schedtypes.ClassroomView,
	tasks []schedtypes.TeachingTaskView,
	dims []string,
) schedtypes.ScoreBreakdown {
	_ = assignments
	_ = allocations
	_ = teachers
	_ = classrooms
	_ = tasks
	_ = dims
	return schedtypes.ScoreBreakdown{}
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
