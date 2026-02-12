package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"mcloud/config"
	"mcloud/models"
	"mcloud/repositories"

	"gorm.io/gorm"
)

type FolderService interface {
	GetOrCreateRootFolder(ctx context.Context, userID uint) (models.Folder, error)
	ResolveFolderID(ctx context.Context, userID uint, folderID uint) (uint, error)
	ListFolders(ctx context.Context, userID uint, parentID *uint) ([]models.Folder, error)
	CreateFolder(ctx context.Context, userID uint, name string, parentID uint) (models.Folder, error)
	RenameFolder(ctx context.Context, userID uint, folderID uint, name string) (models.Folder, error)
	DeleteFolder(ctx context.Context, userID uint, folderID uint) error
}

type folderService struct {
	txManager TxManager
	folders   repositories.FolderRepository
	files     repositories.FileRepository
	recycle   repositories.RecycleBinRepository
	resolver  folderResolver
}

func NewFolderService(
	txManager TxManager,
	folders repositories.FolderRepository,
	files repositories.FileRepository,
	recycle repositories.RecycleBinRepository,
) FolderService {
	return &folderService{
		txManager: txManager,
		folders:   folders,
		files:     files,
		recycle:   recycle,
		resolver:  folderResolver{folders: folders},
	}
}

func (s *folderService) GetOrCreateRootFolder(ctx context.Context, userID uint) (models.Folder, error) {
	root, err := s.resolver.getOrCreateUserRootFolder(ctx, nil, userID)
	if err != nil {
		return models.Folder{}, newAppError(http.StatusInternalServerError, "获取根目录失败", err)
	}
	return root, nil
}

func (s *folderService) ResolveFolderID(ctx context.Context, userID uint, folderID uint) (uint, error) {
	resolved, err := s.resolver.resolveFolderIDForUser(ctx, nil, userID, folderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, newAppError(http.StatusNotFound, "目标文件夹不存在", nil)
		}
		return 0, newAppError(http.StatusInternalServerError, "校验目标文件夹失败", err)
	}
	return resolved, nil
}

func (s *folderService) ListFolders(ctx context.Context, userID uint, parentID *uint) ([]models.Folder, error) {
	rootFolder, err := s.resolver.getOrCreateUserRootFolder(ctx, nil, userID)
	if err != nil {
		return nil, newAppError(http.StatusInternalServerError, "获取根目录失败", err)
	}

	targetParentID := rootFolder.ID
	if parentID != nil {
		resolvedParentID, err := s.resolver.resolveFolderIDForUser(ctx, nil, userID, *parentID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, newAppError(http.StatusNotFound, "父文件夹不存在", nil)
			}
			return nil, newAppError(http.StatusInternalServerError, "校验父文件夹失败", err)
		}
		targetParentID = resolvedParentID
	}

	includeLegacyRoot := targetParentID == rootFolder.ID
	list, err := s.folders.ListByParent(ctx, nil, userID, targetParentID, includeLegacyRoot)
	if err != nil {
		return nil, newAppError(http.StatusInternalServerError, "获取文件夹列表失败", err)
	}
	return list, nil
}

func (s *folderService) CreateFolder(ctx context.Context, userID uint, name string, parentID uint) (models.Folder, error) {
	resolvedParentID, err := s.resolver.resolveFolderIDForUser(ctx, nil, userID, parentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Folder{}, newAppError(http.StatusNotFound, "父文件夹不存在", nil)
		}
		return models.Folder{}, newAppError(http.StatusInternalServerError, "校验父文件夹失败", err)
	}

	parent, err := s.folders.GetByIDAndUser(ctx, nil, resolvedParentID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Folder{}, newAppError(http.StatusNotFound, "父文件夹不存在", nil)
		}
		return models.Folder{}, newAppError(http.StatusInternalServerError, "查询父文件夹失败", err)
	}

	count, err := s.folders.CountByParentAndName(ctx, nil, userID, resolvedParentID, name, 0)
	if err != nil {
		return models.Folder{}, newAppError(http.StatusInternalServerError, "检查文件夹重名失败", err)
	}
	if count > 0 {
		return models.Folder{}, newAppError(http.StatusBadRequest, "同名文件夹已存在", nil)
	}

	parentIDPtr := resolvedParentID
	folder := models.Folder{
		Name:     name,
		ParentID: &parentIDPtr,
		UserID:   userID,
		Path:     buildChildFolderPath(parent.Path, name),
	}
	if err := s.folders.Create(ctx, nil, &folder); err != nil {
		return models.Folder{}, newAppError(http.StatusInternalServerError, "创建文件夹失败", err)
	}

	return folder, nil
}

func (s *folderService) RenameFolder(ctx context.Context, userID uint, folderID uint, name string) (models.Folder, error) {
	folder, err := s.folders.GetByIDAndUser(ctx, nil, folderID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Folder{}, newAppError(http.StatusNotFound, "文件夹不存在", nil)
		}
		return models.Folder{}, newAppError(http.StatusInternalServerError, "查询文件夹失败", err)
	}
	if folder.IsRoot != nil && *folder.IsRoot {
		return models.Folder{}, newAppError(http.StatusBadRequest, "根目录不允许重命名", nil)
	}

	parentID := uint(0)
	if folder.ParentID != nil {
		parentID = *folder.ParentID
	}

	duplicateCount, err := s.folders.CountByParentAndName(ctx, nil, userID, parentID, name, folder.ID)
	if err != nil {
		return models.Folder{}, newAppError(http.StatusInternalServerError, "检查重名失败", err)
	}
	if duplicateCount > 0 {
		return models.Folder{}, newAppError(http.StatusBadRequest, "同名文件夹已存在", nil)
	}

	oldPath := folder.Path
	parentPath := "/"
	if folder.ParentID != nil {
		parent, err := s.folders.GetByIDAndUser(ctx, nil, *folder.ParentID, userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return models.Folder{}, newAppError(http.StatusNotFound, "父文件夹不存在", nil)
			}
			return models.Folder{}, newAppError(http.StatusInternalServerError, "查询父文件夹失败", err)
		}
		parentPath = parent.Path
	}
	newPath := buildChildFolderPath(parentPath, name)

	if err := s.folders.UpdateByID(ctx, nil, folder.ID, map[string]interface{}{"name": name, "path": newPath}); err != nil {
		return models.Folder{}, newAppError(http.StatusInternalServerError, "重命名失败", err)
	}

	children, err := s.folders.ListByPathPrefix(ctx, nil, userID, folder.ID, oldPath, false)
	if err != nil {
		return models.Folder{}, newAppError(http.StatusInternalServerError, "更新子目录路径失败", err)
	}
	for i := range children {
		if children[i].ID == folder.ID {
			continue
		}
		newChildPath := strings.Replace(children[i].Path, oldPath, newPath, 1)
		if err := s.folders.UpdateByID(ctx, nil, children[i].ID, map[string]interface{}{"path": newChildPath}); err != nil {
			return models.Folder{}, newAppError(http.StatusInternalServerError, "更新子目录路径失败", err)
		}
	}

	folder.Name = name
	folder.Path = newPath
	return folder, nil
}

func (s *folderService) DeleteFolder(ctx context.Context, userID uint, folderID uint) error {
	folder, err := s.folders.GetByIDAndUser(ctx, nil, folderID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return newAppError(http.StatusNotFound, "文件夹不存在", nil)
		}
		return newAppError(http.StatusInternalServerError, "查询文件夹失败", err)
	}
	if folder.IsRoot != nil && *folder.IsRoot {
		return newAppError(http.StatusBadRequest, "根目录不允许删除", nil)
	}

	err = s.txManager.WithTransaction(ctx, func(tx *gorm.DB) error {
		if config.AppConfig.RecycleBin.Enabled {
			parentIDVal := uint(0)
			if folder.ParentID != nil {
				parentIDVal = *folder.ParentID
			}
			recycleItem := models.RecycleBinItem{
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
			if err := s.recycle.Create(ctx, tx, &recycleItem); err != nil {
				return err
			}
		}

		affectedFolderIDs, err := s.folders.PluckIDsByPathPrefix(ctx, tx, userID, folderID, folder.Path)
		if err != nil {
			return err
		}

		if err := s.folders.SoftDeleteByPathPrefix(ctx, tx, userID, folderID, folder.Path); err != nil {
			return err
		}

		if len(affectedFolderIDs) > 0 {
			if err := s.files.SoftDeleteByFolderIDs(ctx, tx, userID, affectedFolderIDs); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return newAppError(http.StatusInternalServerError, "删除文件夹失败", err)
	}

	return nil
}
