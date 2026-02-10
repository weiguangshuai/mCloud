package models

import (
	"time"

	"gorm.io/gorm"
)

type Folder struct {
	ID        uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string         `gorm:"type:varchar(255);not null" json:"name"`
	ParentID  *uint          `gorm:"index" json:"parent_id"`
	UserID    uint           `gorm:"not null;index" json:"user_id"`
	IsRoot    *bool          `gorm:"index" json:"is_root"`
	Path      string         `gorm:"type:varchar(1000);not null" json:"path"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
