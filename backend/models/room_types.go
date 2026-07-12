package models

// RoomType 常量集中定义 — 单一真相源。
// 新增类型只需在此处添加，不需要修改匹配逻辑（除非有新的排他规则）。
const (
	RoomTypeStandard    = "普通教室"
	RoomTypeMultimedia  = "多媒体教室"
	RoomTypeLab         = "实验室"
	RoomTypeComputer    = "机房"
	RoomTypeGymnasium   = "体育馆"
	RoomTypeLectureHall = "阶梯教室"
	// 未来: RoomTypeArtStudio = "画室"
	//       RoomTypeVRLab     = "VR实验室"
	//       RoomTypeSmartRoom = "智慧教室"
)

// SpecialtyRoomTypes 排他教室类型集合 — 普通课程不能使用。
// 声明为变量而非常量，以便未来通过配置扩展（如从数据库加载）。
var SpecialtyRoomTypes = map[string]bool{
	RoomTypeLab:       true,
	RoomTypeComputer:  true,
	RoomTypeGymnasium: true,
}

// SharedVenueTypes 允许并发使用的场地类型（不参与独占占用检查）。
var SharedVenueTypes = map[string]bool{
	RoomTypeGymnasium: true,
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
	CategoryPE:       RoomTypeGymnasium,
	CategoryLab:      RoomTypeLab,
	CategoryComputer: RoomTypeComputer,
	// CategoryTheory / CategorySeminar / CategoryArt → "" (不限定特定类型)
}
