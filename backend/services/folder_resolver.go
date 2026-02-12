package services

import (
	"context"
	"errors"
	"strings"

	"mcloud/models"
	"mcloud/repositories"

	"gorm.io/gorm"
)

type folderResolver struct {
	folders repositories.FolderRepository
}

func (r folderResolver) getOrCreateUserRootFolder(ctx context.Context, tx *gorm.DB, userID uint) (models.Folder, error) {
	root, err := r.folders.GetRootByUser(ctx, tx, userID)
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
	if err := r.folders.Create(ctx, tx, &root); err != nil {
		return models.Folder{}, err
	}
	return root, nil
}

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

func buildChildFolderPath(parentPath, childName string) string {
	if parentPath == "" || parentPath == "/" {
		return "/" + childName
	}
	return strings.TrimRight(parentPath, "/") + "/" + childName
}
