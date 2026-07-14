package matcher

import (
	"fmt"
	"strings"

	"scheduling-system/backend/models"
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
	reqType := InferRoomType(task, course)
	if reqType != "" {
		if room.RoomType != reqType {
			return MatchResult{
				OK:           false,
				Code:         CodeRoomTypeMismatch,
				Reason:       fmt.Sprintf("需要%s，当前教室为%s", reqType, room.RoomType),
				RequiredType: reqType,
				ActualType:   room.RoomType,
			}
		}
	} else {
		if models.SpecialtyRoomTypes[room.RoomType] {
			return MatchResult{
				OK:         false,
				Code:       CodeSpecialtyExclusion,
				Reason:     fmt.Sprintf("普通课程不能使用%s", room.RoomType),
				ActualType: room.RoomType,
			}
		}
	}

	reqEquip := models.ParseEquipment(task.RequiredEquipment)
	if !reqEquip.IsEmpty() {
		roomEquip := models.ParseEquipment(room.Equipment)
		var missing []string
		for _, item := range reqEquip.Items() {
			if !roomEquip.Has(item) {
				missing = append(missing, item)
			}
		}
		if len(missing) > 0 {
			return MatchResult{
				OK:               false,
				Code:             CodeEquipmentMissing,
				Reason:           fmt.Sprintf("缺少设备: %s", strings.Join(missing, ", ")),
				MissingEquipment: missing,
			}
		}
	}

	return MatchResult{OK: true, Code: MatchOK}
}

func InferRoomType(task models.TeachingTask, course models.Course) string {
	if task.RequiredRoomType != "" {
		return task.RequiredRoomType
	}
	if course.Category != "" {
		if rt, ok := models.CategoryRoomTypeMap[course.Category]; ok && rt != "" {
			return rt
		}
	}
	return InferRoomTypeByName(course.Name)
}

func IsSharedVenue(room models.Classroom) bool {
	return models.SharedVenueTypes[room.RoomType]
}

func AllowedRooms(task models.TeachingTask, course models.Course, rooms []models.Classroom) []models.Classroom {
	var out []models.Classroom
	for _, room := range rooms {
		if Match(task, course, room).OK {
			out = append(out, room)
		}
	}
	return out
}

func ExplainRequirement(task models.TeachingTask, course models.Course) ResourceRequirement {
	reqType := InferRoomType(task, course)
	source := "none"
	if task.RequiredRoomType != "" {
		source = "explicit"
	} else if course.Category != "" {
		if _, ok := models.CategoryRoomTypeMap[course.Category]; ok {
			source = "category"
		}
	} else if reqType != "" {
		source = "name_fallback"
	}

	reqEquip := models.ParseEquipment(task.RequiredEquipment)
	equipList := reqEquip.Items()

	isShared := false
	if reqType != "" {
		isShared = models.SharedVenueTypes[reqType]
	}

	return ResourceRequirement{
		RoomType:       reqType,
		RoomTypeSource: source,
		Equipment:      equipList,
		IsShared:       isShared,
	}
}

func ExplainMismatch(result MatchResult) string {
	if result.OK {
		return ""
	}
	switch result.Code {
	case CodeRoomTypeMismatch:
		return fmt.Sprintf("教室类型不匹配: 需要%s，当前为%s", result.RequiredType, result.ActualType)
	case CodeSpecialtyExclusion:
		return fmt.Sprintf("排他教室限制: 普通课程不能使用%s", result.ActualType)
	case CodeEquipmentMissing:
		return fmt.Sprintf("缺少设备: %s", strings.Join(result.MissingEquipment, ", "))
	default:
		return result.Reason
	}
}

func InferRoomTypeByName(courseName string) string {
	if models.IsSportsCourse(courseName) {
		return models.RoomTypeGym
	}
	if IsLabCourse(courseName) {
		return models.RoomTypeLab
	}
	if IsComputerCourse(courseName) {
		return models.RoomTypeComputer
	}
	return ""
}

func IsLabCourse(courseName string) bool {
	return strings.Contains(strings.ToLower(courseName), "实验")
}

func IsComputerCourse(courseName string) bool {
	return strings.Contains(strings.ToLower(courseName), "上机")
}