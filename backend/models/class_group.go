package models

import "gorm.io/gorm"

// ClassGroup represents a student class/group (e.g., 计算机2301).
type ClassGroup struct {
	gorm.Model
	Code     string `gorm:"uniqueIndex;size:20" json:"code"`
	Name     string `gorm:"size:100" json:"name"`
	Dept     string `gorm:"size:50;index" json:"dept"`
	Grade    int    `json:"grade"`      // 入学年份, e.g. 2023
	Students int    `json:"students"`   // 学生人数
	Status   string `gorm:"size:20;default:active" json:"status"` // active, inactive
}
