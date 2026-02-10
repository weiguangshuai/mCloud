package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"mcloud/config"
	"mcloud/database"
	"mcloud/models"
	"mcloud/utils"

	"github.com/gin-gonic/gin"
)

type CreateFolderRequest struct {
	Name     string `json:"name" binding:"required,max=255"`
	ParentID uint   `json:"parent_id"`
}

type RenameFolderRequest struct {
	Name string `json:"name" binding:"required,max=255"`
}

func ListFolders(c *gin.Context) {
	userID := c.GetUint("user_id")
	parentIDStr := c.DefaultQuery("parent_id", "0")
	parentID, _ := strconv.ParseUint(parentIDStr, 10, 32)

	var folders []models.Folder
	database.DB.Where("user_id = ? AND parent_id = ?", userID, uint(parentID)).
		Order("name ASC").Find(&folders)

	utils.Success(c, folders)
}

func CreateFolder(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req CreateFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	// 计算路径
	path := "/" + req.Name
	if req.ParentID > 0 {
		var parent models.Folder
		if err := database.DB.Where("id = ? AND user_id = ?", req.ParentID, userID).First(&parent).Error; err != nil {
			utils.Error(c, http.StatusNotFound, "父文件夹不存在")
			return
		}
		path = parent.Path + "/" + req.Name
	}

	// 检查同名文件夹
	var count int64
	database.DB.Model(&models.Folder{}).
		Where("user_id = ? AND parent_id = ? AND name = ?", userID, req.ParentID, req.Name).
		Count(&count)
	if count > 0 {
		utils.Error(c, http.StatusBadRequest, "同名文件夹已存在")
		return
	}

	folder := models.Folder{
		Name:     req.Name,
		ParentID: req.ParentID,
		UserID:   userID,
		Path:     path,
	}

	if err := database.DB.Create(&folder).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "创建文件夹失败")
		return
	}

	utils.Success(c, folder)
}

func RenameFolder(c *gin.Context) {
	userID := c.GetUint("user_id")
	folderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的文件夹ID")
		return
	}

	var req RenameFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	var folder models.Folder
	if err := database.DB.Where("id = ? AND user_id = ?", folderID, userID).First(&folder).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "文件夹不存在")
		return
	}

	oldPath := folder.Path
	// 计算新路径
	newPath := "/" + req.Name
	if folder.ParentID > 0 {
		var parent models.Folder
		database.DB.First(&parent, folder.ParentID)
		newPath = parent.Path + "/" + req.Name
	}

	// 更新当前文件夹
	database.DB.Model(&folder).Updates(map[string]interface{}{
		"name": req.Name,
		"path": newPath,
	})

	// 递归更新子文件夹路径
	var children []models.Folder
	database.DB.Where("user_id = ? AND path LIKE ?", userID, oldPath+"/%").Find(&children)
	for _, child := range children {
		newChildPath := newPath + child.Path[len(oldPath):]
		database.DB.Model(&child).Update("path", newChildPath)
	}

	folder.Name = req.Name
	folder.Path = newPath
	utils.Success(c, folder)
}

func DeleteFolder(c *gin.Context) {
	userID := c.GetUint("user_id")
	folderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的文件夹ID")
		return
	}

	var folder models.Folder
	if err := database.DB.Where("id = ? AND user_id = ?", folderID, userID).First(&folder).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "文件夹不存在")
		return
	}

	// 写入回收站
	if config.AppConfig.RecycleBin.Enabled {
		recycleBinItem := models.RecycleBinItem{
			UserID:           userID,
			OriginalID:       folder.ID,
			OriginalType:     "folder",
			OriginalName:     folder.Name,
			OriginalPath:     folder.Path,
			OriginalFolderID: &folder.ParentID,
			ExpiresAt:        time.Now().AddDate(0, 0, config.AppConfig.RecycleBin.RetentionDays),
			Metadata:         fmt.Sprintf(`{"parent_id":%d}`, folder.ParentID),
		}
		database.DB.Create(&recycleBinItem)
	}

	// 软删除文件夹及其子文件夹
	database.DB.Where("user_id = ? AND (id = ? OR path LIKE ?)", userID, folderID, folder.Path+"/%").
		Delete(&models.Folder{})

	// 软删除文件夹内的文件
	database.DB.Where("user_id = ? AND folder_id = ?", userID, folderID).
		Delete(&models.File{})

	utils.SuccessWithMessage(c, "文件夹已删除", nil)
}
