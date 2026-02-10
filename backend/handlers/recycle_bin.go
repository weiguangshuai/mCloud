package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"mcloud/config"
	"mcloud/database"
	"mcloud/models"
	"mcloud/utils"

	"github.com/gin-gonic/gin"
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

	if item.OriginalType == "file" {
		// 恢复文件（取消软删除）
		database.DB.Unscoped().Model(&models.File{}).
			Where("id = ?", item.OriginalID).
			Updates(map[string]interface{}{"deleted_at": nil})
	} else {
		// 恢复文件夹
		database.DB.Unscoped().Model(&models.Folder{}).
			Where("id = ?", item.OriginalID).
			Updates(map[string]interface{}{"deleted_at": nil})
	}

	database.DB.Delete(&item)
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

	if item.OriginalType == "file" {
		// 删除物理文件
		absPath := filepath.Join(config.AppConfig.Storage.BasePath, item.OriginalPath)
		os.Remove(absPath)

		// 永久删除数据库记录
		database.DB.Unscoped().Where("id = ?", item.OriginalID).Delete(&models.File{})

		// 更新存储配额
		if item.FileSize != nil {
			database.DB.Model(&models.User{}).Where("id = ?", userID).
				UpdateColumn("storage_used", database.DB.Raw("storage_used - ?", *item.FileSize))
		}
	} else {
		database.DB.Unscoped().Where("id = ?", item.OriginalID).Delete(&models.Folder{})
	}

	database.DB.Delete(&item)
	utils.SuccessWithMessage(c, "永久删除成功", nil)
}

// EmptyRecycleBin 清空回收站
func EmptyRecycleBin(c *gin.Context) {
	userID := c.GetUint("user_id")

	var items []models.RecycleBinItem
	database.DB.Where("user_id = ?", userID).Find(&items)

	for _, item := range items {
		if item.OriginalType == "file" {
			absPath := filepath.Join(config.AppConfig.Storage.BasePath, item.OriginalPath)
			os.Remove(absPath)
			database.DB.Unscoped().Where("id = ?", item.OriginalID).Delete(&models.File{})
			if item.FileSize != nil {
				database.DB.Model(&models.User{}).Where("id = ?", userID).
					UpdateColumn("storage_used", database.DB.Raw("storage_used - ?", *item.FileSize))
			}
		} else {
			database.DB.Unscoped().Where("id = ?", item.OriginalID).Delete(&models.Folder{})
		}
	}

	database.DB.Where("user_id = ?", userID).Delete(&models.RecycleBinItem{})
	utils.SuccessWithMessage(c, "回收站已清空", nil)
}
