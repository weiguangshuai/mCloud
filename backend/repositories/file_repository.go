package repositories

import (
	"context"
	"strings"

	"mcloud/models"

	"gorm.io/gorm"
)

type GormFileRepository struct {
	db *gorm.DB
}

func NewGormFileRepository(db *gorm.DB) *GormFileRepository {
	return &GormFileRepository{db: db}
}

func (r *GormFileRepository) folderQuery(db *gorm.DB, userID uint, folderID uint, rootFolderID uint, includeLegacyRoot bool) *gorm.DB {
	if includeLegacyRoot && folderID == rootFolderID {
		return db.Where("user_id = ? AND (folder_id = ? OR folder_id = 0)", userID, rootFolderID)
	}
	return db.Where("user_id = ? AND folder_id = ?", userID, folderID)
}

func (r *GormFileRepository) CountByFolder(_ context.Context, tx *gorm.DB, userID uint, folderID uint, rootFolderID uint, includeLegacyRoot bool) (int64, error) {
	db := useTx(r.db, tx)
	var total int64
	err := r.folderQuery(db.Model(&models.File{}), userID, folderID, rootFolderID, includeLegacyRoot).Count(&total).Error
	return total, err
}

func (r *GormFileRepository) CountByFolderAndOriginalName(_ context.Context, tx *gorm.DB, userID uint, folderID uint, originalName string, excludeID uint, unscoped bool) (int64, error) {
	db := useTx(r.db, tx)
	if unscoped {
		db = db.Unscoped()
	}
	query := db.Model(&models.File{}).
		Where("user_id = ? AND folder_id = ? AND original_name = ?", userID, folderID, originalName)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	err := query.Count(&count).Error
	return count, err
}

func (r *GormFileRepository) ListByFolder(_ context.Context, tx *gorm.DB, in ListFilesInput) ([]models.File, error) {
	db := useTx(r.db, tx)
	query := r.folderQuery(db.Preload("FileObject").Model(&models.File{}), in.UserID, in.FolderID, in.RootFolderID, in.IncludeLegacyRoot)

	if in.SortBy == "file_size" {
		query = query.Joins("LEFT JOIN file_objects ON file_objects.id = files.file_object_id").Select("files.*")
	}

	sortColumns := map[string]string{
		"name":       "files.name",
		"created_at": "files.created_at",
		"file_size":  "file_objects.file_size",
	}
	sortCol := sortColumns[in.SortBy]
	if sortCol == "" {
		sortCol = sortColumns["created_at"]
	}

	order := strings.ToUpper(in.Order)
	if order != "ASC" {
		order = "DESC"
	}

	var files []models.File
	err := query.Order(sortCol + " " + order).Offset(in.Offset).Limit(in.Limit).Find(&files).Error
	return files, err
}

func (r *GormFileRepository) ListByFolderIDs(_ context.Context, tx *gorm.DB, userID uint, folderIDs []uint, preloadObject bool, unscoped bool) ([]models.File, error) {
	db := useTx(r.db, tx)
	if preloadObject {
		db = db.Preload("FileObject")
	}
	if unscoped {
		db = db.Unscoped()
	}
	var files []models.File
	err := db.Where("user_id = ? AND folder_id IN ?", userID, folderIDs).Find(&files).Error
	return files, err
}

func (r *GormFileRepository) Create(_ context.Context, tx *gorm.DB, file *models.File) error {
	return useTx(r.db, tx).Create(file).Error
}

func (r *GormFileRepository) GetByIDAndUser(_ context.Context, tx *gorm.DB, fileID uint, userID uint, preloadObject bool) (models.File, error) {
	db := useTx(r.db, tx)
	if preloadObject {
		db = db.Preload("FileObject")
	}
	var file models.File
	err := db.Where("id = ? AND user_id = ?", fileID, userID).First(&file).Error
	return file, err
}

func (r *GormFileRepository) GetByIDAndUserUnscoped(_ context.Context, tx *gorm.DB, fileID uint, userID uint, preloadObject bool) (models.File, error) {
	db := useTx(r.db, tx).Unscoped()
	if preloadObject {
		db = db.Preload("FileObject")
	}
	var file models.File
	err := db.Where("id = ? AND user_id = ?", fileID, userID).First(&file).Error
	return file, err
}

func (r *GormFileRepository) GetByIDsAndUser(_ context.Context, tx *gorm.DB, userID uint, fileIDs []uint, preloadObject bool) ([]models.File, error) {
	db := useTx(r.db, tx)
	if preloadObject {
		db = db.Preload("FileObject")
	}
	var files []models.File
	err := db.Where("user_id = ? AND id IN ?", userID, fileIDs).Find(&files).Error
	return files, err
}

func (r *GormFileRepository) UpdateByIDAndUser(_ context.Context, tx *gorm.DB, fileID uint, userID uint, updates map[string]interface{}) error {
	return useTx(r.db, tx).Model(&models.File{}).Where("id = ? AND user_id = ?", fileID, userID).Updates(updates).Error
}

func (r *GormFileRepository) UpdateByIDsAndUser(_ context.Context, tx *gorm.DB, fileIDs []uint, userID uint, updates map[string]interface{}) error {
	if len(fileIDs) == 0 {
		return nil
	}
	return useTx(r.db, tx).Model(&models.File{}).Where("id IN ? AND user_id = ?", fileIDs, userID).Updates(updates).Error
}

func (r *GormFileRepository) SoftDeleteByIDAndUser(_ context.Context, tx *gorm.DB, fileID uint, userID uint) error {
	return useTx(r.db, tx).Where("id = ? AND user_id = ?", fileID, userID).Delete(&models.File{}).Error
}

func (r *GormFileRepository) SoftDeleteByFolderIDs(_ context.Context, tx *gorm.DB, userID uint, folderIDs []uint) error {
	if len(folderIDs) == 0 {
		return nil
	}
	return useTx(r.db, tx).Where("user_id = ? AND folder_id IN ?", userID, folderIDs).Delete(&models.File{}).Error
}

func (r *GormFileRepository) UnscopedDeleteByIDAndUser(_ context.Context, tx *gorm.DB, fileID uint, userID uint) error {
	return useTx(r.db, tx).Unscoped().Where("id = ? AND user_id = ?", fileID, userID).Delete(&models.File{}).Error
}

func (r *GormFileRepository) UnscopedRestoreByIDAndUser(_ context.Context, tx *gorm.DB, fileID uint, userID uint, updates map[string]interface{}) error {
	return useTx(r.db, tx).Unscoped().Model(&models.File{}).Where("id = ? AND user_id = ?", fileID, userID).Updates(updates).Error
}

func (r *GormFileRepository) UnscopedRestoreByFolderIDs(_ context.Context, tx *gorm.DB, userID uint, folderIDs []uint, updates map[string]interface{}) error {
	if len(folderIDs) == 0 {
		return nil
	}
	return useTx(r.db, tx).Unscoped().Model(&models.File{}).Where("user_id = ? AND folder_id IN ?", userID, folderIDs).Updates(updates).Error
}

func (r *GormFileRepository) FindByUserAndMD5(_ context.Context, tx *gorm.DB, userID uint, md5 string) (models.FileObject, error) {
	var obj models.FileObject
	err := useTx(r.db, tx).
		Joins("JOIN files ON files.file_object_id = file_objects.id").
		Where("files.user_id = ? AND file_objects.file_md5 = ? AND files.deleted_at IS NULL", userID, md5).
		First(&obj).Error
	return obj, err
}
