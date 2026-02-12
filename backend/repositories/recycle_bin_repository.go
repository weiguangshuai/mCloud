package repositories

import (
	"context"
	"time"

	"mcloud/models"

	"gorm.io/gorm"
)

type GormRecycleBinRepository struct {
	db *gorm.DB
}

func NewGormRecycleBinRepository(db *gorm.DB) *GormRecycleBinRepository {
	return &GormRecycleBinRepository{db: db}
}

func (r *GormRecycleBinRepository) CountByUser(_ context.Context, tx *gorm.DB, userID uint) (int64, error) {
	var total int64
	err := useTx(r.db, tx).Model(&models.RecycleBinItem{}).Where("user_id = ?", userID).Count(&total).Error
	return total, err
}

func (r *GormRecycleBinRepository) ListByUser(_ context.Context, tx *gorm.DB, in RecycleBinListInput) ([]models.RecycleBinItem, error) {
	db := useTx(r.db, tx).Where("user_id = ?", in.UserID)
	if in.SortSQL != "" {
		db = db.Order(in.SortSQL)
	}
	var items []models.RecycleBinItem
	err := db.Offset(in.Offset).Limit(in.Limit).Find(&items).Error
	return items, err
}

func (r *GormRecycleBinRepository) ListAllByUser(_ context.Context, tx *gorm.DB, userID uint) ([]models.RecycleBinItem, error) {
	var items []models.RecycleBinItem
	err := useTx(r.db, tx).Where("user_id = ?", userID).Find(&items).Error
	return items, err
}

func (r *GormRecycleBinRepository) ListExpired(_ context.Context, tx *gorm.DB, now time.Time) ([]models.RecycleBinItem, error) {
	var items []models.RecycleBinItem
	err := useTx(r.db, tx).Where("expires_at < ?", now).Find(&items).Error
	return items, err
}

func (r *GormRecycleBinRepository) GetByIDAndUser(_ context.Context, tx *gorm.DB, itemID uint, userID uint) (models.RecycleBinItem, error) {
	var item models.RecycleBinItem
	err := useTx(r.db, tx).Where("id = ? AND user_id = ?", itemID, userID).First(&item).Error
	return item, err
}

func (r *GormRecycleBinRepository) Create(_ context.Context, tx *gorm.DB, item *models.RecycleBinItem) error {
	return useTx(r.db, tx).Create(item).Error
}

func (r *GormRecycleBinRepository) DeleteByID(_ context.Context, tx *gorm.DB, itemID uint) error {
	return useTx(r.db, tx).Delete(&models.RecycleBinItem{}, itemID).Error
}

func (r *GormRecycleBinRepository) DeleteByUser(_ context.Context, tx *gorm.DB, userID uint) error {
	return useTx(r.db, tx).Where("user_id = ?", userID).Delete(&models.RecycleBinItem{}).Error
}

func (r *GormRecycleBinRepository) DeleteByOriginalIDs(_ context.Context, tx *gorm.DB, userID uint, originalType string, originalIDs []uint) error {
	if len(originalIDs) == 0 {
		return nil
	}
	return useTx(r.db, tx).
		Where("user_id = ? AND original_type = ? AND original_id IN ?", userID, originalType, originalIDs).
		Delete(&models.RecycleBinItem{}).Error
}
