package models

import "time"

type FileObject struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	FilePath      string    `gorm:"type:varchar(1000);not null" json:"file_path"`
	ThumbnailPath string    `gorm:"type:varchar(1000)" json:"thumbnail_path"`
	FileSize      int64     `gorm:"not null" json:"file_size"`
	MimeType      string    `gorm:"type:varchar(100)" json:"mime_type"`
	IsImage       bool      `gorm:"default:false" json:"is_image"`
	Width         int       `json:"width"`
	Height        int       `json:"height"`
	FileMD5       string    `gorm:"type:varchar(32);index" json:"file_md5"`
	RefCount      int       `gorm:"default:1" json:"ref_count"`
	CreatedAt     time.Time `json:"created_at"`
}
