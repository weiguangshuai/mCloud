package repositories

import (
	"context"

	"mcloud/models"

	"gorm.io/gorm"
)

type GormFolderRepository struct {
	db *gorm.DB
}

func NewGormFolderRepository(db *gorm.DB) *GormFolderRepository {
	return &GormFolderRepository{db: db}
}

func (r *GormFolderRepository) GetByIDAndUser(_ context.Context, tx *gorm.DB, folderID uint, userID uint) (models.Folder, error) {
	var folder models.Folder
	err := useTx(r.db, tx).Where("id = ? AND user_id = ?", folderID, userID).First(&folder).Error
	return folder, err
}

func (r *GormFolderRepository) GetByIDAndUserUnscoped(_ context.Context, tx *gorm.DB, folderID uint, userID uint) (models.Folder, error) {
	var folder models.Folder
	err := useTx(r.db, tx).Unscoped().Where("id = ? AND user_id = ?", folderID, userID).First(&folder).Error
	return folder, err
}

func (r *GormFolderRepository) GetRootByUser(_ context.Context, tx *gorm.DB, userID uint) (models.Folder, error) {
	var folder models.Folder
	err := useTx(r.db, tx).Where("user_id = ? AND is_root = 1", userID).First(&folder).Error
	return folder, err
}

func (r *GormFolderRepository) Create(_ context.Context, tx *gorm.DB, folder *models.Folder) error {
	return useTx(r.db, tx).Create(folder).Error
}

func (r *GormFolderRepository) ListByParent(_ context.Context, tx *gorm.DB, userID uint, parentID uint, includeLegacyRoot bool) ([]models.Folder, error) {
	db := useTx(r.db, tx).Model(&models.Folder{}).Where("user_id = ?", userID)
	if includeLegacyRoot {
		db = db.Where("((parent_id = ?) OR (parent_id IS NULL AND (is_root IS NULL OR is_root = 0)))", parentID)
	} else {
		db = db.Where("parent_id = ?", parentID)
	}

	var folders []models.Folder
	err := db.Order("name ASC").Find(&folders).Error
	return folders, err
}

func (r *GormFolderRepository) CountByParentAndName(_ context.Context, tx *gorm.DB, userID uint, parentID uint, name string, excludeID uint) (int64, error) {
	db := useTx(r.db, tx).Model(&models.Folder{}).
		Where("user_id = ? AND parent_id = ? AND name = ?", userID, parentID, name)
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}
	var count int64
	err := db.Count(&count).Error
	return count, err
}

func (r *GormFolderRepository) UpdateByID(_ context.Context, tx *gorm.DB, folderID uint, updates map[string]interface{}) error {
	return useTx(r.db, tx).Model(&models.Folder{}).Where("id = ?", folderID).Updates(updates).Error
}

func (r *GormFolderRepository) UpdateByIDUnscoped(_ context.Context, tx *gorm.DB, folderID uint, updates map[string]interface{}) error {
	return useTx(r.db, tx).Unscoped().Model(&models.Folder{}).Where("id = ?", folderID).Updates(updates).Error
}

func (r *GormFolderRepository) ListByPathPrefix(_ context.Context, tx *gorm.DB, userID uint, rootID uint, rootPath string, unscoped bool) ([]models.Folder, error) {
	db := useTx(r.db, tx)
	if unscoped {
		db = db.Unscoped()
	}

	var folders []models.Folder
	err := db.Where("user_id = ? AND (id = ? OR path LIKE ?)", userID, rootID, rootPath+"/%").Find(&folders).Error
	return folders, err
}

func (r *GormFolderRepository) PluckIDsByPathPrefix(_ context.Context, tx *gorm.DB, userID uint, rootID uint, rootPath string) ([]uint, error) {
	var ids []uint
	err := useTx(r.db, tx).Model(&models.Folder{}).
		Where("user_id = ? AND (id = ? OR path LIKE ?)", userID, rootID, rootPath+"/%").
		Pluck("id", &ids).Error
	return ids, err
}

func (r *GormFolderRepository) SoftDeleteByPathPrefix(_ context.Context, tx *gorm.DB, userID uint, rootID uint, rootPath string) error {
	return useTx(r.db, tx).Where("user_id = ? AND (id = ? OR path LIKE ?)", userID, rootID, rootPath+"/%").Delete(&models.Folder{}).Error
}

func (r *GormFolderRepository) UnscopedDeleteByIDs(_ context.Context, tx *gorm.DB, folderIDs []uint) error {
	if len(folderIDs) == 0 {
		return nil
	}
	return useTx(r.db, tx).Unscoped().Where("id IN ?", folderIDs).Delete(&models.Folder{}).Error
}
