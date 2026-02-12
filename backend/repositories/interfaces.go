package repositories

import (
	"context"
	"time"

	"mcloud/models"

	"gorm.io/gorm"
)

type TxManager interface {
	WithTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error
}

type UserRepository interface {
	CountByUsername(ctx context.Context, username string) (int64, error)
	Create(ctx context.Context, tx *gorm.DB, user *models.User) error
	GetByUsername(ctx context.Context, tx *gorm.DB, username string) (models.User, error)
	GetByID(ctx context.Context, tx *gorm.DB, userID uint) (models.User, error)
	AddStorageUsed(ctx context.Context, tx *gorm.DB, userID uint, delta int64) error
	SubStorageUsed(ctx context.Context, tx *gorm.DB, userID uint, delta int64) error
}

type FolderRepository interface {
	GetByIDAndUser(ctx context.Context, tx *gorm.DB, folderID uint, userID uint) (models.Folder, error)
	GetByIDAndUserUnscoped(ctx context.Context, tx *gorm.DB, folderID uint, userID uint) (models.Folder, error)
	GetRootByUser(ctx context.Context, tx *gorm.DB, userID uint) (models.Folder, error)
	Create(ctx context.Context, tx *gorm.DB, folder *models.Folder) error
	ListByParent(ctx context.Context, tx *gorm.DB, userID uint, parentID uint, includeLegacyRoot bool) ([]models.Folder, error)
	CountByParentAndName(ctx context.Context, tx *gorm.DB, userID uint, parentID uint, name string, excludeID uint) (int64, error)
	UpdateByID(ctx context.Context, tx *gorm.DB, folderID uint, updates map[string]interface{}) error
	UpdateByIDUnscoped(ctx context.Context, tx *gorm.DB, folderID uint, updates map[string]interface{}) error
	ListByPathPrefix(ctx context.Context, tx *gorm.DB, userID uint, rootID uint, rootPath string, unscoped bool) ([]models.Folder, error)
	PluckIDsByPathPrefix(ctx context.Context, tx *gorm.DB, userID uint, rootID uint, rootPath string) ([]uint, error)
	SoftDeleteByPathPrefix(ctx context.Context, tx *gorm.DB, userID uint, rootID uint, rootPath string) error
	UnscopedDeleteByIDs(ctx context.Context, tx *gorm.DB, folderIDs []uint) error
}

type ListFilesInput struct {
	UserID            uint
	FolderID          uint
	RootFolderID      uint
	IncludeLegacyRoot bool
	SortBy            string
	Order             string
	Offset            int
	Limit             int
}

type FileRepository interface {
	CountByFolder(ctx context.Context, tx *gorm.DB, userID uint, folderID uint, rootFolderID uint, includeLegacyRoot bool) (int64, error)
	CountByFolderAndOriginalName(ctx context.Context, tx *gorm.DB, userID uint, folderID uint, originalName string, excludeID uint, unscoped bool) (int64, error)
	ListByFolder(ctx context.Context, tx *gorm.DB, in ListFilesInput) ([]models.File, error)
	ListByFolderIDs(ctx context.Context, tx *gorm.DB, userID uint, folderIDs []uint, preloadObject bool, unscoped bool) ([]models.File, error)
	Create(ctx context.Context, tx *gorm.DB, file *models.File) error
	GetByIDAndUser(ctx context.Context, tx *gorm.DB, fileID uint, userID uint, preloadObject bool) (models.File, error)
	GetByIDAndUserUnscoped(ctx context.Context, tx *gorm.DB, fileID uint, userID uint, preloadObject bool) (models.File, error)
	GetByIDsAndUser(ctx context.Context, tx *gorm.DB, userID uint, fileIDs []uint, preloadObject bool) ([]models.File, error)
	UpdateByIDAndUser(ctx context.Context, tx *gorm.DB, fileID uint, userID uint, updates map[string]interface{}) error
	UpdateByIDsAndUser(ctx context.Context, tx *gorm.DB, fileIDs []uint, userID uint, updates map[string]interface{}) error
	SoftDeleteByIDAndUser(ctx context.Context, tx *gorm.DB, fileID uint, userID uint) error
	SoftDeleteByFolderIDs(ctx context.Context, tx *gorm.DB, userID uint, folderIDs []uint) error
	UnscopedDeleteByIDAndUser(ctx context.Context, tx *gorm.DB, fileID uint, userID uint) error
	UnscopedRestoreByIDAndUser(ctx context.Context, tx *gorm.DB, fileID uint, userID uint, updates map[string]interface{}) error
	UnscopedRestoreByFolderIDs(ctx context.Context, tx *gorm.DB, userID uint, folderIDs []uint, updates map[string]interface{}) error
	FindByUserAndMD5(ctx context.Context, tx *gorm.DB, userID uint, md5 string) (models.FileObject, error)
}

type FileObjectRepository interface {
	Create(ctx context.Context, tx *gorm.DB, fileObject *models.FileObject) error
	GetByID(ctx context.Context, tx *gorm.DB, fileObjectID uint) (models.FileObject, error)
	IncrementRefCount(ctx context.Context, tx *gorm.DB, fileObjectID uint) error
	DecrementRefCount(ctx context.Context, tx *gorm.DB, fileObjectID uint) error
	DeleteByID(ctx context.Context, tx *gorm.DB, fileObjectID uint) error
}

type UploadTaskRepository interface {
	Create(ctx context.Context, tx *gorm.DB, task *models.UploadTask) error
	GetByUploadID(ctx context.Context, tx *gorm.DB, uploadID string) (models.UploadTask, error)
	GetByUploadIDAndUser(ctx context.Context, tx *gorm.DB, uploadID string, userID uint) (models.UploadTask, error)
	UpdateStatus(ctx context.Context, tx *gorm.DB, uploadID string, status string) error
	DeleteByID(ctx context.Context, tx *gorm.DB, id uint) error
	ListExpiredAndUncompleted(ctx context.Context, tx *gorm.DB, now time.Time) ([]models.UploadTask, error)
}

type RecycleBinListInput struct {
	UserID  uint
	Offset  int
	Limit   int
	SortSQL string
}

type RecycleBinRepository interface {
	CountByUser(ctx context.Context, tx *gorm.DB, userID uint) (int64, error)
	ListByUser(ctx context.Context, tx *gorm.DB, in RecycleBinListInput) ([]models.RecycleBinItem, error)
	ListAllByUser(ctx context.Context, tx *gorm.DB, userID uint) ([]models.RecycleBinItem, error)
	ListExpired(ctx context.Context, tx *gorm.DB, now time.Time) ([]models.RecycleBinItem, error)
	GetByIDAndUser(ctx context.Context, tx *gorm.DB, itemID uint, userID uint) (models.RecycleBinItem, error)
	Create(ctx context.Context, tx *gorm.DB, item *models.RecycleBinItem) error
	DeleteByID(ctx context.Context, tx *gorm.DB, itemID uint) error
	DeleteByUser(ctx context.Context, tx *gorm.DB, userID uint) error
	DeleteByOriginalIDs(ctx context.Context, tx *gorm.DB, userID uint, originalType string, originalIDs []uint) error
}

type UploadProgressRepository interface {
	IsChunkUploaded(ctx context.Context, uploadID string, chunkIndex int) (bool, error)
	AddChunk(ctx context.Context, uploadID string, chunkIndex int, expireSeconds int) error
	UploadedCount(ctx context.Context, uploadID string) (int64, error)
	UploadedChunks(ctx context.Context, uploadID string) ([]int, error)
	Clear(ctx context.Context, uploadID string) error
}

type Container struct {
	TxManager      TxManager
	Users          UserRepository
	Folders        FolderRepository
	Files          FileRepository
	FileObjects    FileObjectRepository
	UploadTasks    UploadTaskRepository
	RecycleBin     RecycleBinRepository
	UploadProgress UploadProgressRepository
}
