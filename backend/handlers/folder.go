package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mcloud/config"
	"mcloud/database"
	"mcloud/models"
	"mcloud/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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

	rootFolder, err := getOrCreateUserRootFolder(userID)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "获取根目录失败")
		return
	}

	targetParentID := rootFolder.ID
	if parentIDStr, exists := c.GetQuery("parent_id"); exists {
		parsedParentID, err := strconv.ParseUint(parentIDStr, 10, 32)
		if err != nil {
			utils.Error(c, http.StatusBadRequest, "无效的 parent_id")
			return
		}
		resolvedParentID, err := resolveFolderIDForUser(userID, uint(parsedParentID))
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				utils.Error(c, http.StatusNotFound, "父文件夹不存在")
			} else {
				utils.Error(c, http.StatusInternalServerError, "校验父文件夹失败")
			}
			return
		}
		targetParentID = resolvedParentID
	}

	isRootRequest := targetParentID == rootFolder.ID
	query := database.DB.Model(&models.Folder{}).Where("user_id = ?", userID)
	if isRootRequest {
		query = query.Where("((parent_id = ?) OR (parent_id IS NULL AND (is_root IS NULL OR is_root = 0)))", rootFolder.ID)
	} else {
		query = query.Where("parent_id = ?", targetParentID)
	}

	var folders []models.Folder
	if err := query.Order("name ASC").Find(&folders).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "获取文件夹列表失败")
		return
	}

	utils.Success(c, folders)
}

func CreateFolder(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req CreateFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	resolvedParentID, err := resolveFolderIDForUser(userID, req.ParentID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.Error(c, http.StatusNotFound, "父文件夹不存在")
		} else {
			utils.Error(c, http.StatusInternalServerError, "校验父文件夹失败")
		}
		return
	}

	var parent models.Folder
	if err := database.DB.Where("id = ? AND user_id = ?", resolvedParentID, userID).First(&parent).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "父文件夹不存在")
		return
	}
	path := buildChildFolderPath(parent.Path, req.Name)

	// 检查同名文件夹
	var count int64
	if err := database.DB.Model(&models.Folder{}).
		Where("user_id = ? AND parent_id = ? AND name = ?", userID, resolvedParentID, req.Name).
		Count(&count).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "检查文件夹重名失败")
		return
	}
	if count > 0 {
		utils.Error(c, http.StatusBadRequest, "同名文件夹已存在")
		return
	}

	parentIDPtr := resolvedParentID
	folder := models.Folder{
		Name:     req.Name,
		ParentID: &parentIDPtr,
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
	if folder.IsRoot != nil && *folder.IsRoot {
		utils.Error(c, http.StatusBadRequest, "根目录不允许重命名")
		return
	}

	var duplicateCount int64
	if err := database.DB.Model(&models.Folder{}).
		Where("user_id = ? AND parent_id = ? AND name = ? AND id <> ?", userID, folder.ParentID, req.Name, folder.ID).
		Count(&duplicateCount).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "检查重名失败")
		return
	}
	if duplicateCount > 0 {
		utils.Error(c, http.StatusBadRequest, "同名文件夹已存在")
		return
	}

	oldPath := folder.Path
	parentPath := "/"
	if folder.ParentID != nil {
		var parent models.Folder
		if err := database.DB.Where("id = ? AND user_id = ?", *folder.ParentID, userID).First(&parent).Error; err != nil {
			utils.Error(c, http.StatusNotFound, "父文件夹不存在")
			return
		}
		parentPath = parent.Path
	}
	newPath := buildChildFolderPath(parentPath, req.Name)

	if err := database.DB.Model(&folder).Updates(map[string]interface{}{
		"name": req.Name,
		"path": newPath,
	}).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "重命名失败")
		return
	}

	// 递归更新子文件夹路径
	var children []models.Folder
	if err := database.DB.Where("user_id = ? AND path LIKE ?", userID, oldPath+"/%").Find(&children).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "更新子目录路径失败")
		return
	}
	for i := range children {
		newChildPath := strings.Replace(children[i].Path, oldPath, newPath, 1)
		if err := database.DB.Model(&children[i]).Update("path", newChildPath).Error; err != nil {
			utils.Error(c, http.StatusInternalServerError, "更新子目录路径失败")
			return
		}
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
	if folder.IsRoot != nil && *folder.IsRoot {
		utils.Error(c, http.StatusBadRequest, "根目录不允许删除")
		return
	}

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		// 写入回收站（文件夹树仅保留顶层入口）
		if config.AppConfig.RecycleBin.Enabled {
			parentIDVal := uint(0)
			if folder.ParentID != nil {
				parentIDVal = *folder.ParentID
			}
			recycleBinItem := models.RecycleBinItem{
				UserID:           userID,
				OriginalID:       folder.ID,
				OriginalType:     "folder",
				OriginalName:     folder.Name,
				OriginalPath:     folder.Path,
				OriginalFullPath: folder.Path,
				OriginalFolderID: folder.ParentID,
				ExpiresAt:        time.Now().AddDate(0, 0, config.AppConfig.RecycleBin.RetentionDays),
				Metadata:         fmt.Sprintf(`{"parent_id":%d}`, parentIDVal),
			}
			if err := tx.Create(&recycleBinItem).Error; err != nil {
				return err
			}
		}

		// 收集所有受影响的文件夹 ID（当前文件夹 + 所有子文件夹）
		var affectedFolderIDs []uint
		if err := tx.Model(&models.Folder{}).
			Where("user_id = ? AND (id = ? OR path LIKE ?)", userID, folderID, folder.Path+"/%").
			Pluck("id", &affectedFolderIDs).Error; err != nil {
			return err
		}

		// 软删除文件夹及其子文件夹
		if err := tx.Where("user_id = ? AND (id = ? OR path LIKE ?)", userID, folderID, folder.Path+"/%").
			Delete(&models.Folder{}).Error; err != nil {
			return err
		}

		// 软删除所有受影响文件夹内的文件
		if len(affectedFolderIDs) > 0 {
			if err := tx.Where("user_id = ? AND folder_id IN ?", userID, affectedFolderIDs).
				Delete(&models.File{}).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "删除文件夹失败")
		return
	}

	utils.SuccessWithMessage(c, "文件夹已删除", nil)
}
