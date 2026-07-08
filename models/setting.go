package models

import "gorm.io/gorm"

// Semester represents an academic semester.
type Semester struct {
	gorm.Model
	Name     string `gorm:"uniqueIndex;size:100;not null" json:"name"`
	IsActive bool   `gorm:"default:false" json:"isActive"`
}

// Setting stores key-value application settings.
type Setting struct {
	gorm.Model
	Key   string `gorm:"uniqueIndex;size:100;not null" json:"key"`
	Value string `gorm:"size:500" json:"value"`
}
