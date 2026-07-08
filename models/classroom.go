package models

import "gorm.io/gorm"

// Classroom represents a physical room for teaching.
type Classroom struct {
	gorm.Model
	Code     string `gorm:"uniqueIndex;size:20" json:"code"`
	Name     string `gorm:"size:100;not null" json:"name"`
	Building string `gorm:"size:50" json:"building"`
	Capacity int    `json:"capacity"`
	Type     string `gorm:"size:20" json:"type"` // 普通教室, 实验室, 体育馆, etc.
	Status   string `gorm:"size:20;default:available" json:"status"`
}
