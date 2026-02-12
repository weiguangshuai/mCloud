package services

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	"mcloud/config"
	"mcloud/models"
	"mcloud/repositories"

	"gorm.io/gorm"
)

type CleanupService interface {
	StartWorkers()
}

type cleanupService struct {
	txManager   TxManager
	users       repositories.UserRepository
	folders     repositories.FolderRepository
	files       repositories.FileRepository
	fileObjects repositories.FileObjectRepository
	uploadTasks repositories.UploadTaskRepository
	recycle     repositories.RecycleBinRepository
}

var defaultCleanupService CleanupService

func NewCleanupService(
	txManager TxManager,
	users repositories.UserRepository,
	folders repositories.FolderRepository,
	files repositories.FileRepository,
	fileObjects repositories.FileObjectRepository,
	uploadTasks repositories.UploadTaskRepository,
	recycle repositories.RecycleBinRepository,
) CleanupService {
	return &cleanupService{
		txManager:   txManager,
		users:       users,
		folders:     folders,
		files:       files,
		fileObjects: fileObjects,
		uploadTasks: uploadTasks,
		recycle:     recycle,
	}
}

func SetCleanupService(svc CleanupService) {
	defaultCleanupService = svc
}

// StartCleanupWorkers starts background workers for expiring upload tasks and recycle-bin items.
func StartCleanupWorkers() {
	if defaultCleanupService == nil {
		return
	}
	defaultCleanupService.StartWorkers()
}

func (s *cleanupService) StartWorkers() {
	go s.tempFileCleanupLoop()
	go s.recycleBinCleanupLoop()
}

func (s *cleanupService) tempFileCleanupLoop() {
	interval := time.Duration(config.AppConfig.Storage.TempFileCleanupInterval) * time.Second
	if interval <= 0 {
		interval = time.Hour
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanExpiredUploadTasks(context.Background())
	}
}

func (s *cleanupService) cleanExpiredUploadTasks(ctx context.Context) {
	tasks, err := s.uploadTasks.ListExpiredAndUncompleted(ctx, nil, time.Now())
	if err != nil {
		log.Printf("查询过期上传任务失败: %v", err)
		return
	}

	for _, task := range tasks {
		if task.TempDir != "" {
			_ = os.RemoveAll(task.TempDir)
		}
		if err := s.uploadTasks.DeleteByID(ctx, nil, task.ID); err != nil {
			log.Printf("删除过期上传任务失败 %s: %v", task.UploadID, err)
		}
	}

	if len(tasks) > 0 {
		log.Printf("已清理 %d 个过期上传任务", len(tasks))
	}
}

func (s *cleanupService) recycleBinCleanupLoop() {
	if !config.AppConfig.RecycleBin.Enabled {
		return
	}

	interval := time.Duration(config.AppConfig.RecycleBin.CleanupInterval) * time.Second
	if interval <= 0 {
		interval = 24 * time.Hour
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanExpiredRecycleBinItems(context.Background())
	}
}

func (s *cleanupService) cleanExpiredRecycleBinItems(ctx context.Context) {
	items, err := s.recycle.ListExpired(ctx, nil, time.Now())
	if err != nil {
		log.Printf("查询过期回收站项目失败: %v", err)
		return
	}

	for i := range items {
		item := &items[i]
		err := s.txManager.WithTransaction(ctx, func(tx *gorm.DB) error {
			if item.OriginalType == "file" {
				if err := s.cleanupPermanentDeleteFile(ctx, tx, item); err != nil {
					return err
				}
			} else {
				if err := s.cleanupPermanentDeleteFolder(ctx, tx, item); err != nil {
					return err
				}
			}
			return s.recycle.DeleteByID(ctx, tx, item.ID)
		})
		if err != nil {
			log.Printf("清理回收站项目失败(ID=%d): %v", item.ID, err)
		}
	}

	if len(items) > 0 {
		log.Printf("已清理 %d 个过期回收站项目", len(items))
	}
}

func (s *cleanupService) cleanupPermanentDeleteFile(ctx context.Context, tx *gorm.DB, item *models.RecycleBinItem) error {
	userID := item.UserID
	file, err := s.files.GetByIDAndUserUnscoped(ctx, tx, item.OriginalID, userID, true)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	fileObjectID := uint(0)
	fileSize := int64(0)
	if err == nil {
		fileObjectID = file.FileObjectID
		fileSize = file.FileObject.FileSize
	} else {
		if item.FileObjectID != nil {
			fileObjectID = *item.FileObjectID
		}
		if item.FileSize != nil {
			fileSize = *item.FileSize
		}
	}

	if err := s.files.UnscopedDeleteByIDAndUser(ctx, tx, item.OriginalID, userID); err != nil {
		return err
	}
	if fileSize > 0 {
		if err := s.users.SubStorageUsed(ctx, tx, userID, fileSize); err != nil {
			return err
		}
	}
	if fileObjectID > 0 {
		if err := s.cleanupDecrementFileObjectRef(ctx, tx, fileObjectID); err != nil {
			return err
		}
	}
	return nil
}

func (s *cleanupService) cleanupPermanentDeleteFolder(ctx context.Context, tx *gorm.DB, item *models.RecycleBinItem) error {
	userID := item.UserID
	rootFolder, err := s.folders.GetByIDAndUserUnscoped(ctx, tx, item.OriginalID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if rootFolder.IsRoot != nil && *rootFolder.IsRoot {
		return nil
	}

	folders, err := s.folders.ListByPathPrefix(ctx, tx, userID, rootFolder.ID, rootFolder.Path, true)
	if err != nil {
		return err
	}
	if len(folders) == 0 {
		return nil
	}

	folderIDs := make([]uint, 0, len(folders))
	for _, f := range folders {
		folderIDs = append(folderIDs, f.ID)
	}

	files, err := s.files.ListByFolderIDs(ctx, tx, userID, folderIDs, true, true)
	if err != nil {
		return err
	}
	fileIDs := make([]uint, 0, len(files))
	for i := range files {
		fileIDs = append(fileIDs, files[i].ID)
		size := files[i].FileObject.FileSize
		tmp := &models.RecycleBinItem{UserID: userID, OriginalID: files[i].ID, FileObjectID: &files[i].FileObjectID, FileSize: &size}
		if err := s.cleanupPermanentDeleteFile(ctx, tx, tmp); err != nil {
			return err
		}
	}

	if err := s.folders.UnscopedDeleteByIDs(ctx, tx, folderIDs); err != nil {
		return err
	}
	if err := s.recycle.DeleteByOriginalIDs(ctx, tx, userID, "file", fileIDs); err != nil {
		return err
	}
	if err := s.recycle.DeleteByOriginalIDs(ctx, tx, userID, "folder", folderIDs); err != nil {
		return err
	}
	return nil
}

func (s *cleanupService) cleanupDecrementFileObjectRef(ctx context.Context, tx *gorm.DB, fileObjectID uint) error {
	fileObj, err := s.fileObjects.GetByID(ctx, tx, fileObjectID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	if fileObj.RefCount <= 1 {
		_ = os.Remove(filepath.Join(config.AppConfig.Storage.BasePath, fileObj.FilePath))
		if fileObj.ThumbnailPath != "" {
			_ = os.Remove(filepath.Join(config.AppConfig.Storage.BasePath, fileObj.ThumbnailPath))
		}
		return s.fileObjects.DeleteByID(ctx, tx, fileObj.ID)
	}

	return s.fileObjects.DecrementRefCount(ctx, tx, fileObj.ID)
}
