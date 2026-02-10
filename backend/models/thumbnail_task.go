package models

import "time"

type ThumbnailTask struct {
	ID           uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	FileID       uint       `gorm:"not null;index" json:"file_id"`
	Status       string     `gorm:"type:varchar(20);default:pending;index" json:"status"`
	RetryCount   int        `gorm:"default:0" json:"retry_count"`
	MaxRetries   int        `gorm:"default:3" json:"max_retries"`
	ErrorMessage string     `gorm:"type:text" json:"error_message"`
	CreatedAt    time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	CompletedAt  *time.Time `json:"completed_at"`
}
