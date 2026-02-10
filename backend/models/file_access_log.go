package models

import "time"

type FileAccessLog struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	FileID     uint      `gorm:"not null;index" json:"file_id"`
	UserID     uint      `gorm:"not null;index" json:"user_id"`
	Action     string    `gorm:"type:varchar(20);not null;index" json:"action"`
	IPAddress  string    `gorm:"type:varchar(45)" json:"ip_address"`
	UserAgent  string    `gorm:"type:varchar(500)" json:"user_agent"`
	AccessTime time.Time `gorm:"index;autoCreateTime" json:"access_time"`
	FileSize   *int64    `json:"file_size"`
}
