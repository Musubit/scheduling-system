package models

import (
	"time"

	"gorm.io/gorm"
)

// Semester represents an academic semester.
// v0.5.5: 重构为结构化学期模型，删除 Name/IsActive/string StartDate。
type Semester struct {
	gorm.Model
	AcademicYear string    `gorm:"uniqueIndex:idx_semester;size:20;not null" json:"academicYear"` // "2025-2026"
	Term         string    `gorm:"uniqueIndex:idx_semester;size:10;not null" json:"term"`           // FIRST, SECOND
	StartDate    time.Time `json:"startDate"`                                                       // 学期第一周周一
	EndDate      time.Time `json:"endDate"`                                                         // 学期最后一天
	Status       string    `gorm:"size:20;default:active" json:"status"`                            // active, archived, planned
}

// Semester Term 常量
const (
	SemesterTermFirst  = "FIRST"
	SemesterTermSecond = "SECOND"
)

// Semester Status 常量
const (
	SemesterStatusActive   = "active"
	SemesterStatusArchived = "archived"
	SemesterStatusPlanned  = "planned"
)

// DisplayName 返回学期的显示名称，如 "2025-2026第一学期"。
func (s Semester) DisplayName() string {
	termLabel := "第一学期"
	if s.Term == SemesterTermSecond {
		termLabel = "第二学期"
	}
	return s.AcademicYear + termLabel
}

// Setting stores key-value application settings.
type Setting struct {
	gorm.Model
	Key   string `gorm:"uniqueIndex;size:100;not null" json:"key"`
	Value string `gorm:"type:text" json:"value"` // TEXT for long JSON (locked slots etc.)
}
