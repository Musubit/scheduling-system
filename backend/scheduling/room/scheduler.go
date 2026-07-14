package room

import (
	"scheduling-system/backend/models"
	"scheduling-system/backend/scheduling/matcher"
)

type Matcher interface {
	Match(task models.TeachingTask, course models.Course, cls models.Classroom) matcher.MatchResult
}

type realMatcher struct{}

func (realMatcher) Match(task models.TeachingTask, course models.Course, cls models.Classroom) matcher.MatchResult {
	return matcher.Match(task, course, cls)
}

func NewRealMatcher() Matcher { return realMatcher{} }