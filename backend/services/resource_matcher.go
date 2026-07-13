package services

import (
	"fmt"
	"strings"

	"scheduling-system/backend/models"
)

// ResourceMatchCode 标识匹配结果分类，用于 switch/日志/诊断。
// 使用 int 枚举而非 string，便于 switch 且避免拼写错误。
type ResourceMatchCode int

const (
	MatchOK             ResourceMatchCode = iota // 匹配通过
	CodeRoomTypeMismatch                         // 教室类型不匹配
	CodeEquipmentMissing                         // 缺少所需设备
	CodeSpecialtyExclusion                       // 排他教室类型冲突（普通课不能用实验室等）
)

// MatchResult 是所有匹配调用的统一返回类型。
// 不返回 bool — 调用方可直接使用 Code / Reason 做诊断。
//
// 使用方式:
//
//	if !result.OK { log.Warn(result.Code, result.Reason) }
//	switch result.Code { case CodeRoomTypeMismatch: ... }
type MatchResult struct {
	OK               bool              `json:"ok"`
	Code             ResourceMatchCode `json:"code"`                       // 0=OK, >0=失败类型
	Reason           string            `json:"reason,omitempty"`           // 人类可读，可直接用于调课冲突提示
	RequiredType     string            `json:"requiredType,omitempty"`     // 期望的教室类型
	ActualType       string            `json:"actualType,omitempty"`       // 实际教室类型
	MissingEquipment []string          `json:"missingEquipment,omitempty"` // 缺少的设备列表
}

// ResourceRequirement 是 ExplainRequirement() 的返回，描述任务的完整资源需求。
// 纯结构化数据，不含展示文案 — 调用方自行组装展示。
// 用于排课诊断报告、调课失败提示、日志输出。
type ResourceRequirement struct {
	RoomType       string   `json:"roomType"`       // 推导出的教室类型（空=不限）
	RoomTypeSource string   `json:"roomTypeSource"` // "explicit" | "category" | "name_fallback" | "none"
	Equipment      []string `json:"equipment"`      // 所需设备列表
	IsShared       bool     `json:"isShared"`       // 是否使用共享场地
}

// ResourceMatcher V1 — 统一资源匹配框架。
// 所有函数都是纯函数：相同输入永远产生相同输出，无副作用。
// 不查 DB、不评分、不记日志、不修改数据。
// 调用方负责数据加载，matcher 只负责判断。

// Match 检查教室是否满足任务的资源需求（硬约束）。
// 返回 MatchResult，包含失败原因，可直接用于冲突报告。
func Match(task models.TeachingTask, course models.Course, room models.Classroom) MatchResult {
	// 1. 教室类型检查
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
		// 无显式需求 — 检查排他教室
		if models.SpecialtyRoomTypes[room.RoomType] {
			return MatchResult{
				OK:         false,
				Code:       CodeSpecialtyExclusion,
				Reason:     fmt.Sprintf("普通课程不能使用%s", room.RoomType),
				ActualType: room.RoomType,
			}
		}
	}

	// 2. 设备检查
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

// InferRoomType 推导任务所需的教室类型。
// 优先级: task.RequiredRoomType > Course.Category 映射 > 名称推断 (Deprecated)
func InferRoomType(task models.TeachingTask, course models.Course) string {
	// 1. 显式指定 — 最高优先级
	if task.RequiredRoomType != "" {
		return task.RequiredRoomType
	}
	// 2. Course.Category 映射
	if course.Category != "" {
		if rt, ok := models.CategoryRoomTypeMap[course.Category]; ok && rt != "" {
			return rt
		}
	}
	// 3. 名称推断 — Deprecated, v0.6 将移除
	return inferRoomTypeByName(course.Name)
}

// inferRoomTypeByName 基于课程名关键字推断教室类型。
//
// Deprecated: v0.5.3 保留作为 fallback，v0.6 将通过 migration 补全 Category 后移除。
// 新代码不应依赖此函数，应通过 Course.Category 或 task.RequiredRoomType 指定。
func inferRoomTypeByName(courseName string) string {
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

// IsSharedVenue 判断教室是否为共享场地（允许多课程并发使用）。
func IsSharedVenue(room models.Classroom) bool {
	return models.SharedVenueTypes[room.RoomType]
}

// AllowedRooms 从候选列表中筛选出所有匹配的教室。
// 用于 OR-Tools 预计算：Go 侧调用此函数，将结果序列化到 JSON payload。
// Python 侧只读预计算结果，不做任何匹配逻辑。
func AllowedRooms(task models.TeachingTask, course models.Course, rooms []models.Classroom) []models.Classroom {
	var out []models.Classroom
	for _, room := range rooms {
		if Match(task, course, room).OK {
			out = append(out, room)
		}
	}
	return out
}

// ExplainRequirement 生成任务的资源需求描述（纯结构化数据）。
// 用于排课诊断报告、调课失败提示、日志输出 — 不做匹配判断，只描述需求。
// 调用方自行组装展示文案。
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

// ExplainMismatch 将 MatchResult 转为人类可读的 mismatch 描述。
// 用于调课失败提示、排课诊断日志 — 根据 Code 分类生成文案。
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
