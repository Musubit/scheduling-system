package models

import "time"

// SchemaMigration 记录已应用的迁移版本,用于 spec §2.8 的
// "migrate up if current < X" 逻辑。v0.5.5 引入的第一条记录是 "v0.5.5"。
type SchemaMigration struct {
	Version   string    `gorm:"primaryKey;size:32" json:"version"`
	AppliedAt time.Time `gorm:"not null" json:"appliedAt"`
}

// TableName 显式指定,匹配 spec §2.8 的 DDL。
func (SchemaMigration) TableName() string { return "schema_migrations" }
