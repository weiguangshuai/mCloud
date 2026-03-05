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

// CleanupService 定义后台清理任务入口。
type CleanupService interface {
	// StartWorkers 启动后台清理协程。
	StartWorkers()
}

// cleanupService 聚合清理流程所需仓储依赖。
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

// NewCleanupService 创建后台清理服务实例。
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

// SetCleanupService 注册默认清理服务供全局启动入口使用。
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

// StartWorkers 启动临时文件和回收站两类清理循环。
func (s *cleanupService) StartWorkers() {
	// 两类清理任务相互独立，分别常驻轮询。
	go s.tempFileCleanupLoop()
	go s.recycleBinCleanupLoop()
}

// tempFileCleanupLoop 按配置周期清理过期上传任务。
func (s *cleanupService) tempFileCleanupLoop() {
	interval := time.Duration(config.AppConfig.Storage.TempFileCleanupInterval) * time.Second
	if interval <= 0 {
		interval = time.Hour
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		// 使用后台上下文执行定时清理，避免依赖外部请求生命周期。
		s.cleanExpiredUploadTasks(context.Background())
	}
}

// cleanExpiredUploadTasks 删除过期上传任务及其临时目录。
func (s *cleanupService) cleanExpiredUploadTasks(ctx context.Context) {
	tasks, err := s.uploadTasks.ListExpiredAndUncompleted(ctx, nil, time.Now())
	if err != nil {
		log.Printf("查询过期上传任务失败: %v", err)
		return
	}

	for _, task := range tasks {
		// 先清理临时分片目录，再删除任务元数据，避免磁盘残留。
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

// recycleBinCleanupLoop 按配置周期清理回收站过期条目。
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

// cleanExpiredRecycleBinItems 逐条彻删已过期回收站记录。
func (s *cleanupService) cleanExpiredRecycleBinItems(ctx context.Context) {
	items, err := s.recycle.ListExpired(ctx, nil, time.Now())
	if err != nil {
		log.Printf("查询过期回收站项目失败: %v", err)
		return
	}

	for i := range items {
		item := &items[i]
		// 每个回收站项独立事务处理，避免单个失败阻塞全部清理。
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

// cleanupPermanentDeleteFile 彻底删除单文件并回收空间与对象引用。
func (s *cleanupService) cleanupPermanentDeleteFile(ctx context.Context, tx *gorm.DB, item *models.RecycleBinItem) error {
	userID := item.UserID
	file, err := s.files.GetByIDAndUserUnscoped(ctx, tx, item.OriginalID, userID, true)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// 优先使用实时文件记录；若记录已不存在则回退到回收站快照字段。
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
		// 引用计数归零时需要同时回收物理文件与缩略图。
		if err := s.cleanupDecrementFileObjectRef(ctx, tx, fileObjectID); err != nil {
			return err
		}
	}
	return nil
}

// cleanupPermanentDeleteFolder 彻底删除目录树及其包含文件。
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

	// 按路径前缀拉出整个子树，保证“删目录”语义包含所有后代。
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
		// 目录内文件复用单文件永久删除流程，统一引用计数与空间回收逻辑。
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

// cleanupDecrementFileObjectRef 递减文件对象引用并在归零时删除物理文件。
func (s *cleanupService) cleanupDecrementFileObjectRef(ctx context.Context, tx *gorm.DB, fileObjectID uint) error {
	fileObj, err := s.fileObjects.GetByID(ctx, tx, fileObjectID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	if fileObj.RefCount <= 1 {
		// 最后一个引用被删除时，物理文件与缩略图都应清理。
		_ = os.Remove(filepath.Join(config.AppConfig.Storage.BasePath, fileObj.FilePath))
		if fileObj.ThumbnailPath != "" {
			_ = os.Remove(filepath.Join(config.AppConfig.Storage.BasePath, fileObj.ThumbnailPath))
		}
		return s.fileObjects.DeleteByID(ctx, tx, fileObj.ID)
	}

	return s.fileObjects.DecrementRefCount(ctx, tx, fileObj.ID)
}
