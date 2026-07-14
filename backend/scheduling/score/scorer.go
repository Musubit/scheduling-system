package score

import (
	"scheduling-system/backend/models"
	"scheduling-system/backend/services"
)

type Scorer struct{}

func NewScorer() *Scorer {
	return &Scorer{}
}

func (s *Scorer) Score(entries []models.ScheduleEntry, teachers []models.Teacher, classrooms []models.Classroom, ctx services.ScoringContext) services.ScoreBreakdown {
	return services.NewScoringService().ScoreSchedule(entries, teachers, classrooms, ctx)
}