package models

import "gorm.io/gorm"

// ClassGroup represents a student class/group (e.g., 计算机2301).
type ClassGroup struct {
	gorm.Model
	Code     string `gorm:"uniqueIndex;size:20" json:"code"`
	Name     string `gorm:"size:100;not null" json:"name"`
	Dept     string `gorm:"size:50;not null;index" json:"dept"`
	Grade    int    `json:"grade"`      // 入学年份, e.g. 2023
	Students int    `json:"students"`   // 学生人数
}
