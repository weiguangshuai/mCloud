package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID           uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string         `gorm:"type:varchar(50);uniqueIndex;not null" json:"username"`
	Password     string         `gorm:"type:varchar(255);not null" json:"-"`
	Nickname     string         `gorm:"type:varchar(100)" json:"nickname"`
	Avatar       string         `gorm:"type:varchar(255)" json:"avatar"`
	StorageQuota int64          `gorm:"default:10737418240;comment:存储配额(字节)" json:"storage_quota"`
	StorageUsed  int64          `gorm:"default:0;comment:已使用存储空间(字节)" json:"storage_used"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}
