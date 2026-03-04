package models

import "time"

type UploadTask struct {
	ID                  uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	UploadID            string     `gorm:"type:varchar(36);uniqueIndex;not null" json:"upload_id"`
	UserID              uint       `gorm:"not null;index" json:"user_id"`
	FolderID            uint       `gorm:"default:0" json:"folder_id"`
	FileName            string     `gorm:"type:varchar(255);not null" json:"file_name"`
	FileSize            int64      `gorm:"not null" json:"file_size"`
	FileMD5             string     `gorm:"type:varchar(32);not null" json:"file_md5"`
	TotalChunks         int        `gorm:"not null" json:"total_chunks"`
	UploadedChunks      string     `gorm:"type:text" json:"uploaded_chunks"`
	UploadedChunksCount int        `gorm:"default:0" json:"uploaded_chunks_count"`
	UploadedSize        int64      `gorm:"default:0" json:"uploaded_size"`
	LastChunkAt         *time.Time `json:"last_chunk_at"`
	CompletedAt         *time.Time `json:"completed_at"`
	LastError           string     `gorm:"type:varchar(500)" json:"last_error"`
	Status              string     `gorm:"type:varchar(20);default:pending;index" json:"status"`
	TempDir             string     `gorm:"type:varchar(500)" json:"temp_dir"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	ExpiresAt           time.Time  `gorm:"not null;index" json:"expires_at"`
}
