package services

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"mcloud/config"
	"mcloud/database"
	"mcloud/models"
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
			os.RemoveAll(task.TempDir)
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

	for _, item := range items {
		if item.OriginalType == "file" {
			// 永久删除逻辑文件记录
			database.DB.Unscoped().Where("id = ?", item.OriginalID).Delete(&models.File{})

			// 更新存储配额
			if item.FileSize != nil {
				database.DB.Model(&models.User{}).Where("id = ?", item.UserID).
					UpdateColumn("storage_used", database.DB.Raw("GREATEST(storage_used - ?, 0)", *item.FileSize))
			}

			// 递减 FileObject ref_count
			if item.FileObjectID != nil {
				var fileObj models.FileObject
				if err := database.DB.First(&fileObj, *item.FileObjectID).Error; err == nil {
					newRefCount := fileObj.RefCount - 1
					if newRefCount <= 0 {
						absPath := filepath.Join(config.AppConfig.Storage.BasePath, fileObj.FilePath)
						os.Remove(absPath)
						if fileObj.ThumbnailPath != "" {
							thumbPath := filepath.Join(config.AppConfig.Storage.BasePath, fileObj.ThumbnailPath)
							os.Remove(thumbPath)
						}
						database.DB.Delete(&fileObj)
					} else {
						database.DB.Model(&fileObj).Update("ref_count", newRefCount)
					}
				}
			}
		} else {
			database.DB.Unscoped().Where("id = ?", item.OriginalID).Delete(&models.Folder{})
		}

		database.DB.Delete(&item)
	}

	if len(items) > 0 {
		log.Printf("共清理 %d 个过期回收站项目", len(items))
	}
}
