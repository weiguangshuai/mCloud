package models

import (
	"time"

	"gorm.io/gorm"
)

type File struct {
	ID            uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Name          string         `gorm:"type:varchar(255);not null" json:"name"`
	OriginalName  string         `gorm:"type:varchar(255);not null" json:"original_name"`
	FolderID      uint           `gorm:"index" json:"folder_id"`
	UserID        uint           `gorm:"not null;index" json:"user_id"`
	FileObjectID  uint           `gorm:"not null;index" json:"file_object_id"`
	FileObject    FileObject     `gorm:"foreignKey:FileObjectID" json:"file_object,omitempty"`
	CreatedAt     time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	DeletedBy     *uint          `json:"deleted_by,omitempty"`
}
