package models

import "gorm.io/gorm"

// TeachingTaskClass links a TeachingTask to its enrolled ClassGroups.
// A composite unique index prevents duplicate bindings.
type TeachingTaskClass struct {
	gorm.Model
	TeachingTaskID uint `gorm:"index;not null;uniqueIndex:idx_tt_class" json:"teachingTaskId"`
	ClassGroupID   uint `gorm:"index;not null;uniqueIndex:idx_tt_class" json:"classGroupId"`

	// Associations
	TeachingTask TeachingTask `gorm:"foreignKey:TeachingTaskID" json:"-"`
	ClassGroup   ClassGroup   `gorm:"foreignKey:ClassGroupID" json:"classGroup,omitempty"`
}
