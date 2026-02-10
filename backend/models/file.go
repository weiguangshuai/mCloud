package models

import (
	"time"

	"gorm.io/gorm"
)

type File struct {
	ID            uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Name          string         `gorm:"type:varchar(255);not null" json:"name"`
	OriginalName  string         `gorm:"type:varchar(255);not null" json:"original_name"`
	FilePath      string         `gorm:"type:varchar(1000);not null" json:"file_path"`
	ThumbnailPath string         `gorm:"type:varchar(1000)" json:"thumbnail_path"`
	FolderID      uint           `gorm:"default:0;index" json:"folder_id"`
	UserID        uint           `gorm:"not null;index" json:"user_id"`
	FileSize      int64          `gorm:"not null" json:"file_size"`
	MimeType      string         `gorm:"type:varchar(100)" json:"mime_type"`
	IsImage       bool           `gorm:"default:false" json:"is_image"`
	Width         int            `json:"width"`
	Height        int            `json:"height"`
	FileMD5       string         `gorm:"type:varchar(32);index:idx_user_md5" json:"file_md5"`
	CreatedAt     time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	DeletedBy     *uint          `json:"deleted_by,omitempty"`
}
