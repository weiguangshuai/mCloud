package services

import (
	"context"
	"errors"
	"net/http"

	"mcloud/repositories"

	"gorm.io/gorm"
)

// StorageQuotaOutput 为用户空间配额查询结果。
type StorageQuotaOutput struct {
	// StorageQuota 为用户总配额（字节）。
	StorageQuota int64 `json:"storage_quota"`
	// StorageUsed 为当前已使用空间（字节）。
	StorageUsed int64 `json:"storage_used"`
	// AvailableSpace 为剩余可用空间（字节）。
	AvailableSpace int64 `json:"available_space"`
	// UsagePercent 为使用率百分比，范围通常在 [0, 100+]。
	UsagePercent float64 `json:"usage_percent"`
}

// UserService 定义用户侧基础能力。
type UserService interface {
	// GetStorageQuota 查询并计算用户空间占用信息。
	GetStorageQuota(ctx context.Context, userID uint) (StorageQuotaOutput, error)
}

// userService 为 UserService 的默认实现。
type userService struct {
	users repositories.UserRepository
}

// NewUserService 创建用户服务实例。
func NewUserService(users repositories.UserRepository) UserService {
	return &userService{users: users}
}

// GetStorageQuota 查询用户配额并计算剩余空间与使用率。
func (s *userService) GetStorageQuota(ctx context.Context, userID uint) (StorageQuotaOutput, error) {
	user, err := s.users.GetByID(ctx, nil, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return StorageQuotaOutput{}, newAppError(http.StatusNotFound, "用户不存在", nil)
		}
		return StorageQuotaOutput{}, newAppError(http.StatusInternalServerError, "查询用户失败", err)
	}

	// 仅在总配额大于 0 时计算占比，避免除零错误。
	usagePercent := 0.0
	if user.StorageQuota > 0 {
		usagePercent = float64(user.StorageUsed) / float64(user.StorageQuota) * 100
	}

	return StorageQuotaOutput{
		StorageQuota:   user.StorageQuota,
		StorageUsed:    user.StorageUsed,
		AvailableSpace: user.StorageQuota - user.StorageUsed,
		UsagePercent:   usagePercent,
	}, nil
}
