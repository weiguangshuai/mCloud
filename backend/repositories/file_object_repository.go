package repositories

import (
	"context"

	"mcloud/models"

	"gorm.io/gorm"
)

type GormFileObjectRepository struct {
	db *gorm.DB
}

func NewGormFileObjectRepository(db *gorm.DB) *GormFileObjectRepository {
	return &GormFileObjectRepository{db: db}
}

func (r *GormFileObjectRepository) Create(_ context.Context, tx *gorm.DB, fileObject *models.FileObject) error {
	return useTx(r.db, tx).Create(fileObject).Error
}

func (r *GormFileObjectRepository) GetByID(_ context.Context, tx *gorm.DB, fileObjectID uint) (models.FileObject, error) {
	var obj models.FileObject
	err := useTx(r.db, tx).First(&obj, fileObjectID).Error
	return obj, err
}

func (r *GormFileObjectRepository) IncrementRefCount(_ context.Context, tx *gorm.DB, fileObjectID uint) error {
	return useTx(r.db, tx).Model(&models.FileObject{}).
		Where("id = ?", fileObjectID).
		Update("ref_count", gorm.Expr("ref_count + 1")).Error
}

func (r *GormFileObjectRepository) DecrementRefCount(_ context.Context, tx *gorm.DB, fileObjectID uint) error {
	return useTx(r.db, tx).Model(&models.FileObject{}).
		Where("id = ?", fileObjectID).
		Update("ref_count", gorm.Expr("ref_count - 1")).Error
}

func (r *GormFileObjectRepository) DeleteByID(_ context.Context, tx *gorm.DB, fileObjectID uint) error {
	return useTx(r.db, tx).Delete(&models.FileObject{}, fileObjectID).Error
}
