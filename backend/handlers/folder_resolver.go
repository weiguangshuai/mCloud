package handlers

import (
	"errors"
	"strings"

	"mcloud/database"
	"mcloud/models"

	"gorm.io/gorm"
)

// getOrCreateUserRootFolder 获取用户根目录；若历史数据缺失则自动补齐。
func getOrCreateUserRootFolder(userID uint) (models.Folder, error) {
	var root models.Folder
	err := database.DB.Where("user_id = ? AND is_root = 1", userID).First(&root).Error
	if err == nil {
		return root, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Folder{}, err
	}

	isRoot := true
	root = models.Folder{
		Name:   "root",
		UserID: userID,
		IsRoot: &isRoot,
		Path:   "/",
	}
	if err := database.DB.Create(&root).Error; err != nil {
		return models.Folder{}, err
	}
	return root, nil
}

// resolveFolderIDForUser 解析并校验目标文件夹，兼容 folder_id=0（自动映射到用户根目录）。
func resolveFolderIDForUser(userID uint, folderID uint) (uint, error) {
	if folderID == 0 {
		root, err := getOrCreateUserRootFolder(userID)
		if err != nil {
			return 0, err
		}
		return root.ID, nil
	}

	var folder models.Folder
	if err := database.DB.Where("id = ? AND user_id = ?", folderID, userID).First(&folder).Error; err != nil {
		return 0, err
	}
	return folder.ID, nil
}

func buildChildFolderPath(parentPath, childName string) string {
	if parentPath == "" || parentPath == "/" {
		return "/" + childName
	}
	return strings.TrimRight(parentPath, "/") + "/" + childName
}
