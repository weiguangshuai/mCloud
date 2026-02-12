package services

import (
	"context"
	"errors"
	"net/http"

	"mcloud/repositories"

	"gorm.io/gorm"
)

type StorageQuotaOutput struct {
	StorageQuota   int64   `json:"storage_quota"`
	StorageUsed    int64   `json:"storage_used"`
	AvailableSpace int64   `json:"available_space"`
	UsagePercent   float64 `json:"usage_percent"`
}

type UserService interface {
	GetStorageQuota(ctx context.Context, userID uint) (StorageQuotaOutput, error)
}

type userService struct {
	users repositories.UserRepository
}

func NewUserService(users repositories.UserRepository) UserService {
	return &userService{users: users}
}

func (s *userService) GetStorageQuota(ctx context.Context, userID uint) (StorageQuotaOutput, error) {
	user, err := s.users.GetByID(ctx, nil, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return StorageQuotaOutput{}, newAppError(http.StatusNotFound, "user not found", nil)
		}
		return StorageQuotaOutput{}, newAppError(http.StatusInternalServerError, "failed to query user", err)
	}

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
