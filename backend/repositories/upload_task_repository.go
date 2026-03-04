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

func (r *GormUploadTaskRepository) FindResumableBySignature(_ context.Context, tx *gorm.DB, userID uint, folderID uint, fileName string, fileSize int64, fileMD5 string, now time.Time) (models.UploadTask, error) {
	var task models.UploadTask
	err := useTx(r.db, tx).
		Where("user_id = ? AND folder_id = ? AND file_name = ? AND file_size = ? AND file_md5 = ?", userID, folderID, fileName, fileSize, fileMD5).
		Where("expires_at > ?", now).
		Where("status IN ?", []string{"pending", "uploading", "paused", "failed"}).
		Order("updated_at DESC").
		First(&task).Error
	return task, err
}

func (r *GormUploadTaskRepository) ListVisibleByUser(_ context.Context, tx *gorm.DB, userID uint, _ time.Time, completedSince time.Time) ([]models.UploadTask, error) {
	var tasks []models.UploadTask
	err := useTx(r.db, tx).
		Where("user_id = ?", userID).
		Where("status != ? OR completed_at >= ?", "completed", completedSince).
		Order("updated_at DESC").
		Find(&tasks).Error
	return tasks, err
}

func (r *GormUploadTaskRepository) UpdateStatus(_ context.Context, tx *gorm.DB, uploadID string, status string) error {
	return useTx(r.db, tx).Model(&models.UploadTask{}).Where("upload_id = ?", uploadID).Update("status", status).Error
}

func (r *GormUploadTaskRepository) UpdateProgress(_ context.Context, tx *gorm.DB, uploadID string, uploadedChunksCount int, uploadedSize int64, lastChunkAt time.Time) error {
	updates := map[string]interface{}{
		"uploaded_chunks_count": uploadedChunksCount,
		"uploaded_size":         uploadedSize,
		"last_chunk_at":         lastChunkAt,
		"status":                "uploading",
		"last_error":            "",
	}
	return useTx(r.db, tx).Model(&models.UploadTask{}).Where("upload_id = ?", uploadID).Updates(updates).Error
}

func (r *GormUploadTaskRepository) MarkCompleted(_ context.Context, tx *gorm.DB, uploadID string, completedAt time.Time) error {
	updates := map[string]interface{}{
		"status":       "completed",
		"completed_at": completedAt,
		"last_error":   "",
	}
	return useTx(r.db, tx).Model(&models.UploadTask{}).Where("upload_id = ?", uploadID).Updates(updates).Error
}

func (r *GormUploadTaskRepository) UpdateUploadedChunksSnapshot(_ context.Context, tx *gorm.DB, uploadID string, uploadedChunks string) error {
	return useTx(r.db, tx).Model(&models.UploadTask{}).Where("upload_id = ?", uploadID).Update("uploaded_chunks", uploadedChunks).Error
}

func (r *GormUploadTaskRepository) DeleteByID(_ context.Context, tx *gorm.DB, id uint) error {
	return useTx(r.db, tx).Delete(&models.UploadTask{}, id).Error
}

func (r *GormUploadTaskRepository) ListExpiredAndUncompleted(_ context.Context, tx *gorm.DB, now time.Time) ([]models.UploadTask, error) {
	var tasks []models.UploadTask
	err := useTx(r.db, tx).Where("expires_at < ? AND status != ?", now, "completed").Find(&tasks).Error
	return tasks, err
}
