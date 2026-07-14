package services

import (
	"scheduling-system/backend/models"
	"scheduling-system/backend/scheduling/matcher"
)

type ResourceMatchCode int

const (
	MatchOK             ResourceMatchCode = iota
	CodeRoomTypeMismatch
	CodeEquipmentMissing
	CodeSpecialtyExclusion
)

type MatchResult struct {
	OK               bool              `json:"ok"`
	Code             ResourceMatchCode `json:"code"`
	Reason           string            `json:"reason,omitempty"`
	RequiredType     string            `json:"requiredType,omitempty"`
	ActualType       string            `json:"actualType,omitempty"`
	MissingEquipment []string          `json:"missingEquipment,omitempty"`
}

type ResourceRequirement struct {
	RoomType       string   `json:"roomType"`
	RoomTypeSource string   `json:"roomTypeSource"`
	Equipment      []string `json:"equipment"`
	IsShared       bool     `json:"isShared"`
}

func Match(task models.TeachingTask, course models.Course, room models.Classroom) MatchResult {
	r := matcher.Match(task, course, room)
	return MatchResult{
		OK:               r.OK,
		Code:             ResourceMatchCode(r.Code),
		Reason:           r.Reason,
		RequiredType:     r.RequiredType,
		ActualType:       r.ActualType,
		MissingEquipment: r.MissingEquipment,
	}
}

func InferRoomType(task models.TeachingTask, course models.Course) string {
	return matcher.InferRoomType(task, course)
}

func IsSharedVenue(room models.Classroom) bool {
	return matcher.IsSharedVenue(room)
}

func AllowedRooms(task models.TeachingTask, course models.Course, rooms []models.Classroom) []models.Classroom {
	return matcher.AllowedRooms(task, course, rooms)
}

func ExplainRequirement(task models.TeachingTask, course models.Course) ResourceRequirement {
	r := matcher.ExplainRequirement(task, course)
	return ResourceRequirement{
		RoomType:       r.RoomType,
		RoomTypeSource: r.RoomTypeSource,
		Equipment:      r.Equipment,
		IsShared:       r.IsShared,
	}
}

func ExplainMismatch(result MatchResult) string {
	r := matcher.MatchResult{
		OK:               result.OK,
		Code:             matcher.ResourceMatchCode(result.Code),
		Reason:           result.Reason,
		RequiredType:     result.RequiredType,
		ActualType:       result.ActualType,
		MissingEquipment: result.MissingEquipment,
	}
	return matcher.ExplainMismatch(r)
}