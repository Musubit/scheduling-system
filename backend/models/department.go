package models

import "gorm.io/gorm"

// Department represents a college/faculty (e.g., 计算机学院, 材料科学与工程学院).
type Department struct {
	gorm.Model
	Code string `gorm:"uniqueIndex;size:20;not null" json:"code"` // 院系代码，如 "cs"
	Name string `gorm:"size:100;not null" json:"name"`            // 院系名称，如 "计算机学院"
}
