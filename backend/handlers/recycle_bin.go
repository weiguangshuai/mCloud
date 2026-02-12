package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mcloud/config"
	"mcloud/database"
	"mcloud/models"
	"mcloud/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ListRecycleBin 获取回收站列表
func ListRecycleBin(c *gin.Context) {
	userID := c.GetUint("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var total int64
	database.DB.Model(&models.RecycleBinItem{}).Where("user_id = ?", userID).Count(&total)

	var items []models.RecycleBinItem
	database.DB.Where("user_id = ?", userID).
		Order("deleted_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items)

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	utils.Success(c, gin.H{
		"items": items,
		"pagination": utils.PaginationData{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
			HasNext:    page < totalPages,
			HasPrev:    page > 1,
		},
	})
}

// RestoreItem 恢复文件/文件夹
func RestoreItem(c *gin.Context) {
	userID := c.GetUint("user_id")
	itemID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var item models.RecycleBinItem
	if err := database.DB.Where("id = ? AND user_id = ?", itemID, userID).First(&item).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "回收站项目不存在")
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if item.OriginalType == "file" {
			if err := restoreFileItem(tx, userID, &item); err != nil {
				return err
			}
		} else {
			if err := restoreFolderItem(tx, userID, &item); err != nil {
				return err
			}
		}
		return tx.Delete(&item).Error
	})

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Error(c, http.StatusNotFound, "待恢复对象不存在")
			return
		}
		utils.Error(c, http.StatusInternalServerError, "恢复失败")
		return
	}

	utils.SuccessWithMessage(c, "恢复成功", nil)
}

// PermanentDelete 永久删除
func PermanentDelete(c *gin.Context) {
	userID := c.GetUint("user_id")
	itemID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var item models.RecycleBinItem
	if err := database.DB.Where("id = ? AND user_id = ?", itemID, userID).First(&item).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "回收站项目不存在")
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if item.OriginalType == "file" {
			if err := permanentDeleteFile(tx, &item, userID); err != nil {
				return err
			}
		} else {
			if err := permanentDeleteFolder(tx, &item, userID); err != nil {
				return err
			}
		}
		return tx.Delete(&item).Error
	})

	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "永久删除失败")
		return
	}

	utils.SuccessWithMessage(c, "永久删除成功", nil)
}

// EmptyRecycleBin 清空回收站
func EmptyRecycleBin(c *gin.Context) {
	userID := c.GetUint("user_id")

	var items []models.RecycleBinItem
	database.DB.Where("user_id = ?", userID).Find(&items)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		for i := range items {
			if items[i].OriginalType == "file" {
				if err := permanentDeleteFile(tx, &items[i], userID); err != nil {
					return err
				}
			} else {
				if err := permanentDeleteFolder(tx, &items[i], userID); err != nil {
					return err
				}
			}
		}
		return tx.Where("user_id = ?", userID).Delete(&models.RecycleBinItem{}).Error
	})

	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "清空回收站失败")
		return
	}

	utils.SuccessWithMessage(c, "回收站已清空", nil)
}

func restoreFileItem(tx *gorm.DB, userID uint, item *models.RecycleBinItem) error {
	var file models.File
	if err := tx.Unscoped().Where("id = ? AND user_id = ?", item.OriginalID, userID).First(&file).Error; err != nil {
		return err
	}

	folderID := file.FolderID
	if item.OriginalFolderID != nil {
		folderID = *item.OriginalFolderID
	}
	folderID = ensureActiveFolderOrRoot(tx, userID, folderID)

	var count int64
	tx.Model(&models.File{}).
		Where("user_id = ? AND folder_id = ? AND original_name = ? AND id <> ?", userID, folderID, item.OriginalName, item.OriginalID).
		Count(&count)

	updates := map[string]interface{}{
		"deleted_at": nil,
		"deleted_by": nil,
		"folder_id":  folderID,
	}
	if count > 0 {
		updates["original_name"] = fmt.Sprintf("%s(restored)", item.OriginalName)
	}

	return tx.Unscoped().Model(&models.File{}).
		Where("id = ? AND user_id = ?", item.OriginalID, userID).
		Updates(updates).Error
}

func restoreFolderItem(tx *gorm.DB, userID uint, item *models.RecycleBinItem) error {
	var folder models.Folder
	if err := tx.Unscoped().Where("id = ? AND user_id = ?", item.OriginalID, userID).First(&folder).Error; err != nil {
		return err
	}
	if folder.IsRoot != nil && *folder.IsRoot {
		return fmt.Errorf("根目录不可恢复")
	}

	restoreParentID := uint(0)
	if folder.ParentID != nil {
		restoreParentID = *folder.ParentID
	} else if item.OriginalFolderID != nil {
		restoreParentID = *item.OriginalFolderID
	}
	restoreParentID = ensureActiveFolderOrRoot(tx, userID, restoreParentID)

	var parent models.Folder
	if err := tx.Where("id = ? AND user_id = ?", restoreParentID, userID).First(&parent).Error; err != nil {
		return err
	}

	restoredName := item.OriginalName
	if restoredName == "" {
		restoredName = folder.Name
	}

	var count int64
	tx.Model(&models.Folder{}).
		Where("user_id = ? AND parent_id = ? AND name = ? AND id <> ?", userID, restoreParentID, restoredName, folder.ID).
		Count(&count)
	if count > 0 {
		restoredName = fmt.Sprintf("%s(restored)", restoredName)
	}

	oldPath := folder.Path
	newPath := buildChildFolderPath(parent.Path, restoredName)

	var affectedFolders []models.Folder
	if err := tx.Unscoped().
		Where("user_id = ? AND (id = ? OR path LIKE ?)", userID, folder.ID, oldPath+"/%").
		Find(&affectedFolders).Error; err != nil {
		return err
	}
	if len(affectedFolders) == 0 {
		return gorm.ErrRecordNotFound
	}

	folderIDs := make([]uint, 0, len(affectedFolders))
	for i := range affectedFolders {
		folderIDs = append(folderIDs, affectedFolders[i].ID)

		updates := map[string]interface{}{"deleted_at": nil}
		if affectedFolders[i].ID == folder.ID {
			updates["name"] = restoredName
			updates["path"] = newPath
			updates["parent_id"] = restoreParentID
		} else {
			updates["path"] = strings.Replace(affectedFolders[i].Path, oldPath, newPath, 1)
		}

		if err := tx.Unscoped().Model(&models.Folder{}).
			Where("id = ?", affectedFolders[i].ID).
			Updates(updates).Error; err != nil {
			return err
		}
	}

	return tx.Unscoped().Model(&models.File{}).
		Where("user_id = ? AND folder_id IN ?", userID, folderIDs).
		Updates(map[string]interface{}{"deleted_at": nil, "deleted_by": nil}).Error
}

func ensureActiveFolderOrRoot(tx *gorm.DB, userID uint, folderID uint) uint {
	if folderID > 0 {
		var folder models.Folder
		if err := tx.Where("id = ? AND user_id = ?", folderID, userID).First(&folder).Error; err == nil {
			return folderID
		}
	}
	root, err := getOrCreateUserRootFolder(userID)
	if err != nil {
		return folderID
	}
	return root.ID
}

// permanentDeleteFile 永久删除文件，处理 FileObject ref_count
func permanentDeleteFile(tx *gorm.DB, item *models.RecycleBinItem, userID uint) error {
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
		if err := decrementFileObjectRef(tx, fileObjectID); err != nil {
			return err
		}
	}

	return nil
}

func permanentDeleteFolder(tx *gorm.DB, item *models.RecycleBinItem, userID uint) error {
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
		return fmt.Errorf("根目录不可删除")
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
		tmpItem := models.RecycleBinItem{
			OriginalID:   files[i].ID,
			FileObjectID: &files[i].FileObjectID,
			FileSize:     &size,
		}
		if err := permanentDeleteFile(tx, &tmpItem, userID); err != nil {
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

func decrementFileObjectRef(tx *gorm.DB, fileObjectID uint) error {
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
