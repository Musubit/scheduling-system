package models

import "gorm.io/gorm"

// Teacher represents a teaching staff member.
type Teacher struct {
	gorm.Model
	Code   string `gorm:"uniqueIndex;size:20" json:"code"`
	Name   string `gorm:"size:50;not null" json:"name"`
	Dept   string `gorm:"size:50;not null;index" json:"dept"`
	Title  string `gorm:"size:20" json:"title"`  // 教授, 副教授, 讲师
	Status string `gorm:"size:20;default:active" json:"status"` // active, inactive
}
