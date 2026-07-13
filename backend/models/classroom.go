package models

import "gorm.io/gorm"

// Classroom represents a physical room for teaching.
//
// v0.5.5 Stage B: 通用资源领域重构。
//   - Building string → BuildingID FK + Building Association（关联 Building 表）
//   - Type → RoomType（英文枚举，见 room_types.go）
//   - 新增 Number 字段（教室号，不含楼栋前缀，如 "302" / "001" / "422"）
//   - Code 字段仅是"唯一标识字符串"，无内在结构约束；编号解析规则在 adapter 层
type Classroom struct {
	gorm.Model
	Code       string `gorm:"uniqueIndex;size:20;not null" json:"code"`
	Name       string `gorm:"size:100;not null" json:"name"`
	BuildingID uint   `gorm:"index;not null" json:"buildingId"`
	Floor      int    `gorm:"default:1" json:"floor"`
	Number     string `gorm:"size:10" json:"number"`
	Capacity   int    `json:"capacity"`
	RoomType   string `gorm:"size:20;not null;index" json:"roomType"`
	Status     string `gorm:"size:20;default:available" json:"status"`

	// +v0.5.3: 设备列表 JSON array，如 ["projector","smartboard","aircon"]
	Equipment string `gorm:"type:text" json:"equipment"`

	// Association
	Building Building `gorm:"foreignKey:BuildingID" json:"building,omitempty"`
}
