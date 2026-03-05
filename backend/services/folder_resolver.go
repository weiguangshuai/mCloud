package services

import (
	"context"
	"errors"
	"strings"

	"mcloud/models"
	"mcloud/repositories"

	"gorm.io/gorm"
)

// folderResolver 负责目录 ID 归一化与根目录兜底创建。
type folderResolver struct {
	folders repositories.FolderRepository
}

// getOrCreateUserRootFolder 获取用户根目录；历史数据缺失时自动创建。
func (r folderResolver) getOrCreateUserRootFolder(ctx context.Context, tx *gorm.DB, userID uint) (models.Folder, error) {
	root, err := r.folders.GetRootByUser(ctx, tx, userID)
	if err == nil {
		return root, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Folder{}, err
	}

	// 根目录固定使用 "/" 路径，便于子目录路径拼接。
	isRoot := true
	root = models.Folder{
		Name:   "root",
		UserID: userID,
		IsRoot: &isRoot,
		Path:   "/",
	}
	if err := r.folders.Create(ctx, tx, &root); err != nil {
		return models.Folder{}, err
	}
	return root, nil
}

// resolveFolderIDForUser 将 folderID 归一化为当前用户可访问的真实目录 ID。
// 约定 folderID=0 代表“用户根目录”。
func (r folderResolver) resolveFolderIDForUser(ctx context.Context, tx *gorm.DB, userID uint, folderID uint) (uint, error) {
	if folderID == 0 {
		root, err := r.getOrCreateUserRootFolder(ctx, tx, userID)
		if err != nil {
			return 0, err
		}
		return root.ID, nil
	}
	folder, err := r.folders.GetByIDAndUser(ctx, tx, folderID, userID)
	if err != nil {
		return 0, err
	}
	return folder.ID, nil
}

// buildChildFolderPath 生成子目录完整路径，并兼容根路径拼接场景。
func buildChildFolderPath(parentPath, childName string) string {
	if parentPath == "" || parentPath == "/" {
		return "/" + childName
	}
	return strings.TrimRight(parentPath, "/") + "/" + childName
}
