package repositories

import (
	"context"
	"time"

	"mcloud/models"

	"gorm.io/gorm"
)

type GormUploadTaskRepository struct {
	db *gorm.DB
}

func NewGormUploadTaskRepository(db *gorm.DB) *GormUploadTaskRepository {
	return &GormUploadTaskRepository{db: db}
}

func (r *GormUploadTaskRepository) Create(_ context.Context, tx *gorm.DB, task *models.UploadTask) error {
	return useTx(r.db, tx).Create(task).Error
}

func (r *GormUploadTaskRepository) GetByUploadID(_ context.Context, tx *gorm.DB, uploadID string) (models.UploadTask, error) {
	var task models.UploadTask
	err := useTx(r.db, tx).Where("upload_id = ?", uploadID).First(&task).Error
	return task, err
}

func (r *GormUploadTaskRepository) GetByUploadIDAndUser(_ context.Context, tx *gorm.DB, uploadID string, userID uint) (models.UploadTask, error) {
	var task models.UploadTask
	err := useTx(r.db, tx).Where("upload_id = ? AND user_id = ?", uploadID, userID).First(&task).Error
	return task, err
}

func (r *GormUploadTaskRepository) UpdateStatus(_ context.Context, tx *gorm.DB, uploadID string, status string) error {
	return useTx(r.db, tx).Model(&models.UploadTask{}).Where("upload_id = ?", uploadID).Update("status", status).Error
}

func (r *GormUploadTaskRepository) DeleteByID(_ context.Context, tx *gorm.DB, id uint) error {
	return useTx(r.db, tx).Delete(&models.UploadTask{}, id).Error
}

func (r *GormUploadTaskRepository) ListExpiredAndUncompleted(_ context.Context, tx *gorm.DB, now time.Time) ([]models.UploadTask, error) {
	var tasks []models.UploadTask
	err := useTx(r.db, tx).Where("expires_at < ? AND status != ?", now, "completed").Find(&tasks).Error
	return tasks, err
}
