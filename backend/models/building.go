package models

import "gorm.io/gorm"

// Building represents a physical building on campus (e.g., "1 教", "5 教 A 区", "体育馆").
//
// Core Domain: 通用建筑抽象，不含任何学校特定规则。
//   - Code 由学校自定义，仅作唯一标识（如 "1"、"5A"、"6A"、"GYM"）。
//   - Name 作为 UI 展示名（如 "1 教"、"5 教 A 区"、"体育馆"）。
//   - Category 描述楼栋整体功能定位，但不硬约束该楼内的教室类型（教学楼里
//     也可以有机房；实验楼里也可以有多媒体教室 —— 由 seed / 用户配置决定）。
//
// 编号解析规则（"6教=实验楼" / "分 A/B 区" 等）不属于 Core，位于
// backend/services/parsers/<school>/ adapter 层。
type Building struct {
	gorm.Model
	Code     string `gorm:"uniqueIndex;size:20;not null" json:"code"`
	Name     string `gorm:"size:100;not null" json:"name"`
	Category string `gorm:"size:20;default:teaching;index" json:"category"`
	Status   string `gorm:"size:20;default:active" json:"status"`
}

// Building Category 常量：仅描述楼栋整体功能定位。
const (
	BuildingCategoryTeaching = "teaching" // 教学楼（可含普通教室 / 多媒体 / 阶梯）
	BuildingCategoryLab      = "lab"      // 实验楼
	BuildingCategorySports   = "sports"   // 体育场馆
	BuildingCategoryOther    = "other"    // 其他
)

// Building Status 常量。
const (
	BuildingStatusActive   = "active"
	BuildingStatusInactive = "inactive"
)
