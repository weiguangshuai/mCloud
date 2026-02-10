package models

import "time"

type RecycleBinItem struct {
	ID               uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID           uint      `gorm:"not null;index" json:"user_id"`
	OriginalID       uint      `gorm:"not null" json:"original_id"`
	OriginalType     string    `gorm:"type:varchar(10);not null;index" json:"original_type"`
	OriginalName     string    `gorm:"type:varchar(255);not null" json:"original_name"`
	OriginalPath     string    `gorm:"type:varchar(1000)" json:"original_path"`
	OriginalFolderID *uint     `json:"original_folder_id"`
	FileSize         *int64    `json:"file_size"`
	DeletedAt        time.Time `gorm:"index;autoCreateTime" json:"deleted_at"`
	ExpiresAt        time.Time `gorm:"not null;index" json:"expires_at"`
	Metadata         string    `gorm:"type:json" json:"metadata"`
}

func (RecycleBinItem) TableName() string {
	return "recycle_bin"
}
