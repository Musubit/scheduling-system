package models

import "gorm.io/gorm"

// Teacher represents a teaching staff member.
type Teacher struct {
	gorm.Model
	Code   string `gorm:"uniqueIndex;size:20" json:"code"`
	Name   string `gorm:"size:50;not null" json:"name"`
	Dept   string `gorm:"size:50;not null;index" json:"dept"`
	Status string `gorm:"size:20;default:active" json:"status"` // active, inactive

	// Soft constraint preferences
	PreferNoEarly  bool `gorm:"default:false" json:"preferNoEarly"`   // 避免早课（1-2节）
	PreferNoLate   bool `gorm:"default:false" json:"preferNoLate"`    // 避免晚课（7-8节及晚上）
	MaxDaysPerWeek int  `gorm:"default:3" json:"maxDaysPerWeek"`     // 每周最多到校天数
	PreferLowFloor bool `gorm:"default:false" json:"preferLowFloor"`  // 优先低楼层
}
