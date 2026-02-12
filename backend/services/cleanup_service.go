package services

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	"mcloud/config"
	"mcloud/database"
	"mcloud/models"

	"gorm.io/gorm"
)

// StartCleanupWorkers 启动后台清理任务
func StartCleanupWorkers() {
	go tempFileCleanupLoop()
	go recycleBinCleanupLoop()
}

// tempFileCleanupLoop 定时清理过期上传任务和临时文件
func tempFileCleanupLoop() {
	interval := time.Duration(config.AppConfig.Storage.TempFileCleanupInterval) * time.Second
	if interval <= 0 {
		interval = time.Hour
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		cleanExpiredUploadTasks()
	}
}

func cleanExpiredUploadTasks() {
	var tasks []models.UploadTask
	database.DB.Where("expires_at < ? AND status != ?", time.Now(), "completed").Find(&tasks)

	for _, task := range tasks {
		// 删除临时目录
		if task.TempDir != "" {
			_ = os.RemoveAll(task.TempDir)
		}
		// 删除数据库记录
		database.DB.Delete(&task)
		log.Printf("清理过期上传任务: %s", task.UploadID)
	}

	if len(tasks) > 0 {
		log.Printf("共清理 %d 个过期上传任务", len(tasks))
	}
}

// recycleBinCleanupLoop 定时清理过期回收站项目
func recycleBinCleanupLoop() {
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
		cleanExpiredRecycleBinItems()
	}
}

func cleanExpiredRecycleBinItems() {
	var items []models.RecycleBinItem
	database.DB.Where("expires_at < ?", time.Now()).Find(&items)

	for i := range items {
		item := &items[i]
		err := database.DB.Transaction(func(tx *gorm.DB) error {
			if item.OriginalType == "file" {
				if err := cleanupPermanentDeleteFile(tx, item); err != nil {
					return err
				}
			} else {
				if err := cleanupPermanentDeleteFolder(tx, item); err != nil {
					return err
				}
			}

			return tx.Delete(item).Error
		})

		if err != nil {
			log.Printf("清理回收站项目失败 (ID: %d): %v", item.ID, err)
		}
	}

	if len(items) > 0 {
		log.Printf("共清理 %d 个过期回收站项目", len(items))
	}
}

func cleanupPermanentDeleteFile(tx *gorm.DB, item *models.RecycleBinItem) error {
	userID := item.UserID

	var file models.File
	err := tx.Unscoped().Preload("FileObject").
		Where("id = ? AND user_id = ?", item.OriginalID, userID).
		First(&file).Error
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

	if err := tx.Unscoped().
		Where("id = ? AND user_id = ?", item.OriginalID, userID).
		Delete(&models.File{}).Error; err != nil {
		return err
	}

	if fileSize > 0 {
		if err := tx.Model(&models.User{}).Where("id = ?", userID).
			UpdateColumn("storage_used", gorm.Expr("GREATEST(storage_used - ?, 0)", fileSize)).Error; err != nil {
			return err
		}
	}

	if fileObjectID > 0 {
		if err := cleanupDecrementFileObjectRef(tx, fileObjectID); err != nil {
			return err
		}
	}

	return nil
}

func cleanupPermanentDeleteFolder(tx *gorm.DB, item *models.RecycleBinItem) error {
	userID := item.UserID

	var rootFolder models.Folder
	if err := tx.Unscoped().
		Where("id = ? AND user_id = ?", item.OriginalID, userID).
		First(&rootFolder).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if rootFolder.IsRoot != nil && *rootFolder.IsRoot {
		return nil
	}

	var folders []models.Folder
	if err := tx.Unscoped().
		Where("user_id = ? AND (id = ? OR path LIKE ?)", userID, rootFolder.ID, rootFolder.Path+"/%").
		Find(&folders).Error; err != nil {
		return err
	}
	if len(folders) == 0 {
		return nil
	}

	folderIDs := make([]uint, 0, len(folders))
	for _, f := range folders {
		folderIDs = append(folderIDs, f.ID)
	}

	var files []models.File
	if err := tx.Unscoped().Preload("FileObject").
		Where("user_id = ? AND folder_id IN ?", userID, folderIDs).
		Find(&files).Error; err != nil {
		return err
	}

	fileIDs := make([]uint, 0, len(files))
	for i := range files {
		fileIDs = append(fileIDs, files[i].ID)
		size := files[i].FileObject.FileSize
		tmp := &models.RecycleBinItem{UserID: userID, OriginalID: files[i].ID, FileObjectID: &files[i].FileObjectID, FileSize: &size}
		if err := cleanupPermanentDeleteFile(tx, tmp); err != nil {
			return err
		}
	}

	if err := tx.Unscoped().Where("id IN ?", folderIDs).Delete(&models.Folder{}).Error; err != nil {
		return err
	}

	if len(fileIDs) > 0 {
		if err := tx.Where("user_id = ? AND original_type = 'file' AND original_id IN ?", userID, fileIDs).
			Delete(&models.RecycleBinItem{}).Error; err != nil {
			return err
		}
	}
	if len(folderIDs) > 0 {
		if err := tx.Where("user_id = ? AND original_type = 'folder' AND original_id IN ?", userID, folderIDs).
			Delete(&models.RecycleBinItem{}).Error; err != nil {
			return err
		}
	}

	return nil
}

func cleanupDecrementFileObjectRef(tx *gorm.DB, fileObjectID uint) error {
	var fileObj models.FileObject
	if err := tx.First(&fileObj, fileObjectID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	if fileObj.RefCount <= 1 {
		absPath := filepath.Join(config.AppConfig.Storage.BasePath, fileObj.FilePath)
		_ = os.Remove(absPath)
		if fileObj.ThumbnailPath != "" {
			thumbPath := filepath.Join(config.AppConfig.Storage.BasePath, fileObj.ThumbnailPath)
			_ = os.Remove(thumbPath)
		}
		return tx.Delete(&fileObj).Error
	}

	return tx.Model(&fileObj).Update("ref_count", gorm.Expr("ref_count - 1")).Error
}
