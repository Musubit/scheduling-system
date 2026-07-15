package models

import (
	"strings"

	"gorm.io/gorm"
)

// Course represents a course in the curriculum.
type Course struct {
	gorm.Model
	Code   string  `gorm:"uniqueIndex;size:20" json:"code"`
	Name   string  `gorm:"size:100" json:"name"`
	Dept   string  `gorm:"size:50;index" json:"dept"`
	Credit float64 `json:"credit"`
	Type   string  `gorm:"size:20" json:"type"` // 专业必修, 全校选修, etc.
	Hours  int     `json:"hours"`
	Status string  `gorm:"size:20;default:active" json:"status"` // active, inactive

	// +v0.5.3: 课程类别，用于资源匹配。空值=由名称推断(兼容, Deprecated)。
	Category string `gorm:"size:30;default:''" json:"category"`
}

// IsSportsCourse returns true if the course name indicates a PE/sports course.
// Single source of truth — used by scoring, SA solver, and snapshot service.
func IsSportsCourse(name string) bool {
	return strings.Contains(name, "体育")
}
