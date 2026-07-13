package models

// RoomType 常量集中定义 — 单一真相源。
//
// 值域为**英文枚举**，前端通过 i18n 映射为中文展示（对应 frontend/src/types
// 的 ROOM_TYPE_LABELS）。DB / 后端 API / Solver 全部使用英文常量。
//
// 新增类型只需在此处添加，不需要修改匹配逻辑（除非有新的排他规则）。
const (
	RoomTypeNormal     = "NORMAL"     // 普通教室
	RoomTypeMultimedia = "MULTIMEDIA" // 多媒体教室
	RoomTypeLab        = "LAB"        // 实验室
	RoomTypeComputer   = "COMPUTER"   // 机房
	RoomTypeGym        = "GYM"        // 体育馆
	RoomTypeLecture    = "LECTURE"    // 阶梯教室
	// 未来: RoomTypeArtStudio = "ART_STUDIO"
	//       RoomTypeVRLab     = "VR_LAB"
	//       RoomTypeSmartRoom = "SMART_ROOM"
)

// SpecialtyRoomTypes 排他教室类型集合 — 普通课程不能使用。
// 声明为变量而非常量，以便未来通过配置扩展（如从数据库加载）。
var SpecialtyRoomTypes = map[string]bool{
	RoomTypeLab:      true,
	RoomTypeComputer: true,
	RoomTypeGym:      true,
}

// SharedVenueTypes 允许并发使用的场地类型（不参与独占占用检查）。
var SharedVenueTypes = map[string]bool{
	RoomTypeGym: true,
}

// CourseCategory 课程类别枚举。
const (
	CategoryTheory   = "theory"
	CategoryLab      = "lab"
	CategoryPE       = "pe"
	CategoryComputer = "computer"
	CategorySeminar  = "seminar"
	CategoryArt      = "art"
	// 未来: CategoryMusic = "music"
)

// CategoryRoomTypeMap 课程类别 → 默认教室类型映射。
// 这是 category→roomType 推导的唯一真相源。
var CategoryRoomTypeMap = map[string]string{
	CategoryPE:       RoomTypeGym,
	CategoryLab:      RoomTypeLab,
	CategoryComputer: RoomTypeComputer,
	// CategoryTheory / CategorySeminar / CategoryArt → "" (不限定特定类型)
}
