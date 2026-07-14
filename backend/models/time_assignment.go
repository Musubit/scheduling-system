package models

import (
	"time"
)

// TimeAssignment 是 v0.5.5 双模式排课的 **时间事实实体**。
// 一条 TA 行 = 一次周课(一个 TeachingTask 在某天某节起某跨度的排课),
// 与"教室"完全解耦(INV-T2 / spec §2.2)。
//
// - FULL_SCHEDULING 模式:每条 TA 对应 exactly one ScheduleEntry(教室分配)。
// - TIME_ONLY_SCHEDULING 模式:只写 TA 不写 ScheduleEntry(INV-E1)。
//
// 这个模型在 v0.5.5 PR-03 阶段被引入但**尚未被生产写路径使用** ——
// PR-09 才做数据模型切换。此时先注册 AutoMigrate 让 schema 落地,
// 为后续 refactor 铺路。
type TimeAssignment struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	SemesterID        uint      `gorm:"index:idx_ta_semester_version,priority:1;not null" json:"semesterId"`
	ScheduleVersionID uint      `gorm:"index:idx_ta_semester_version,priority:2;index:idx_ta_version_task,priority:1;index:idx_ta_version_day_period,priority:1;not null" json:"scheduleVersionId"`
	TeachingTaskID    uint      `gorm:"index:idx_ta_version_task,priority:2;not null" json:"teachingTaskId"`
	DayOfWeek         DayOfWeek `gorm:"index:idx_ta_version_day_period,priority:2;not null" json:"dayOfWeek"`
	StartPeriod       Period    `gorm:"index:idx_ta_version_day_period,priority:3;not null" json:"startPeriod"`
	Span              int       `gorm:"default:2;not null" json:"span"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
	DeletedAt         *time.Time `gorm:"index" json:"deletedAt,omitempty"`
}

// TableName 显式指定表名以对齐 spec。
func (TimeAssignment) TableName() string { return "time_assignments" }
