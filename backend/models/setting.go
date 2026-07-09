package models

import "gorm.io/gorm"

// Semester represents an academic semester.
type Semester struct {
	gorm.Model
	Name      string `gorm:"uniqueIndex;size:100;not null" json:"name"`
	IsActive  bool   `gorm:"default:false" json:"isActive"`
	StartDate string `gorm:"size:20" json:"startDate"` // e.g. "2025-09-01", determines day-to-date mapping
}

// Setting stores key-value application settings.
type Setting struct {
	gorm.Model
	Key   string `gorm:"uniqueIndex;size:100;not null" json:"key"`
	Value string `gorm:"type:text" json:"value"` // TEXT for long JSON (locked slots etc.)
}
