package repositories

import (
	"context"

	"mcloud/models"

	"gorm.io/gorm"
)

type GormUserRepository struct {
	db *gorm.DB
}

func NewGormUserRepository(db *gorm.DB) *GormUserRepository {
	return &GormUserRepository{db: db}
}

func (r *GormUserRepository) CountByUsername(_ context.Context, username string) (int64, error) {
	var count int64
	err := r.db.Model(&models.User{}).Where("username = ?", username).Count(&count).Error
	return count, err
}

func (r *GormUserRepository) Create(_ context.Context, tx *gorm.DB, user *models.User) error {
	return useTx(r.db, tx).Create(user).Error
}

func (r *GormUserRepository) GetByUsername(_ context.Context, tx *gorm.DB, username string) (models.User, error) {
	var user models.User
	err := useTx(r.db, tx).Where("username = ?", username).First(&user).Error
	return user, err
}

func (r *GormUserRepository) GetByID(_ context.Context, tx *gorm.DB, userID uint) (models.User, error) {
	var user models.User
	err := useTx(r.db, tx).First(&user, userID).Error
	return user, err
}

func (r *GormUserRepository) AddStorageUsed(_ context.Context, tx *gorm.DB, userID uint, delta int64) error {
	if delta == 0 {
		return nil
	}
	return useTx(r.db, tx).Model(&models.User{}).
		Where("id = ?", userID).
		UpdateColumn("storage_used", gorm.Expr("storage_used + ?", delta)).Error
}

func (r *GormUserRepository) SubStorageUsed(_ context.Context, tx *gorm.DB, userID uint, delta int64) error {
	if delta <= 0 {
		return nil
	}
	return useTx(r.db, tx).Model(&models.User{}).
		Where("id = ?", userID).
		UpdateColumn("storage_used", gorm.Expr("GREATEST(storage_used - ?, 0)", delta)).Error
}
