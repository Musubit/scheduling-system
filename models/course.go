package models

import "gorm.io/gorm"

// Course represents a course in the curriculum.
type Course struct {
	gorm.Model
	Code   string  `gorm:"uniqueIndex;size:20" json:"code"`
	Name   string  `gorm:"size:100;not null" json:"name"`
	Dept   string  `gorm:"size:50;not null;index" json:"dept"`
	Credit float64 `json:"credit"`
	Type   string  `gorm:"size:20" json:"type"` // 专业必修, 全校选修, etc.
	Hours  int     `json:"hours"`
	Status string  `gorm:"size:20;default:active" json:"status"` // active, inactive
}
