package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"mcloud/config"
	"mcloud/models"
	"mcloud/repositories"
	"mcloud/utils"

	"gorm.io/gorm"
)

type RecycleBinListOutput struct {
	Items      []models.RecycleBinItem `json:"items"`
	Pagination utils.PaginationData    `json:"pagination"`
}

type RecycleBinService interface {
	ListRecycleBin(ctx context.Context, userID uint, page int, pageSize int) (RecycleBinListOutput, error)
	RestoreItem(ctx context.Context, userID uint, itemID uint) error
	PermanentDelete(ctx context.Context, userID uint, itemID uint) error
	EmptyRecycleBin(ctx context.Context, userID uint) error
}

type recycleBinService struct {
	txManager   TxManager
	users       repositories.UserRepository
	folders     repositories.FolderRepository
	files       repositories.FileRepository
	fileObjects repositories.FileObjectRepository
	recycle     repositories.RecycleBinRepository
	resolver    folderResolver
}

func NewRecycleBinService(
	txManager TxManager,
	users repositories.UserRepository,
	folders repositories.FolderRepository,
	files repositories.FileRepository,
	fileObjects repositories.FileObjectRepository,
	recycle repositories.RecycleBinRepository,
) RecycleBinService {
	return &recycleBinService{
		txManager:   txManager,
		users:       users,
		folders:     folders,
		files:       files,
		fileObjects: fileObjects,
		recycle:     recycle,
		resolver:    folderResolver{folders: folders},
	}
}

func (s *recycleBinService) ListRecycleBin(ctx context.Context, userID uint, page int, pageSize int) (RecycleBinListOutput, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	total, err := s.recycle.CountByUser(ctx, nil, userID)
	if err != nil {
		return RecycleBinListOutput{}, newAppError(http.StatusInternalServerError, "查询回收站总数失败", err)
	}

	items, err := s.recycle.ListByUser(ctx, nil, repositories.RecycleBinListInput{
		UserID:  userID,
		SortSQL: "deleted_at DESC",
		Offset:  (page - 1) * pageSize,
		Limit:   pageSize,
	})
	if err != nil {
		return RecycleBinListOutput{}, newAppError(http.StatusInternalServerError, "查询回收站列表失败", err)
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if totalPages == 0 {
		totalPages = 1
	}

	return RecycleBinListOutput{
		Items: items,
		Pagination: utils.PaginationData{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
			HasNext:    page < totalPages,
			HasPrev:    page > 1,
		},
	}, nil
}

func (s *recycleBinService) RestoreItem(ctx context.Context, userID uint, itemID uint) error {
	item, err := s.recycle.GetByIDAndUser(ctx, nil, itemID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return newAppError(http.StatusNotFound, "回收站项目不存在", nil)
		}
		return newAppError(http.StatusInternalServerError, "查询回收站项目失败", err)
	}

	err = s.txManager.WithTransaction(ctx, func(tx *gorm.DB) error {
		if item.OriginalType == "file" {
			if err := s.restoreFileItem(ctx, tx, userID, &item); err != nil {
				return err
			}
		} else {
			if err := s.restoreFolderItem(ctx, tx, userID, &item); err != nil {
				return err
			}
		}
		return s.recycle.DeleteByID(ctx, tx, item.ID)
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return newAppError(http.StatusNotFound, "待恢复对象不存在", nil)
		}
		return newAppError(http.StatusInternalServerError, "恢复失败", err)
	}

	return nil
}

func (s *recycleBinService) PermanentDelete(ctx context.Context, userID uint, itemID uint) error {
	item, err := s.recycle.GetByIDAndUser(ctx, nil, itemID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return newAppError(http.StatusNotFound, "回收站项目不存在", nil)
		}
		return newAppError(http.StatusInternalServerError, "查询回收站项目失败", err)
	}

	err = s.txManager.WithTransaction(ctx, func(tx *gorm.DB) error {
		if item.OriginalType == "file" {
			if err := s.permanentDeleteFile(ctx, tx, &item, userID); err != nil {
				return err
			}
		} else {
			if err := s.permanentDeleteFolder(ctx, tx, &item, userID); err != nil {
				return err
			}
		}
		return s.recycle.DeleteByID(ctx, tx, item.ID)
	})
	if err != nil {
		return newAppError(http.StatusInternalServerError, "永久删除失败", err)
	}
	return nil
}

func (s *recycleBinService) EmptyRecycleBin(ctx context.Context, userID uint) error {
	items, err := s.recycle.ListAllByUser(ctx, nil, userID)
	if err != nil {
		return newAppError(http.StatusInternalServerError, "查询回收站列表失败", err)
	}

	err = s.txManager.WithTransaction(ctx, func(tx *gorm.DB) error {
		for i := range items {
			if items[i].OriginalType == "file" {
				if err := s.permanentDeleteFile(ctx, tx, &items[i], userID); err != nil {
					return err
				}
			} else {
				if err := s.permanentDeleteFolder(ctx, tx, &items[i], userID); err != nil {
					return err
				}
			}
		}
		return s.recycle.DeleteByUser(ctx, tx, userID)
	})
	if err != nil {
		return newAppError(http.StatusInternalServerError, "清空回收站失败", err)
	}
	return nil
}

func (s *recycleBinService) restoreFileItem(ctx context.Context, tx *gorm.DB, userID uint, item *models.RecycleBinItem) error {
	file, err := s.files.GetByIDAndUserUnscoped(ctx, tx, item.OriginalID, userID, false)
	if err != nil {
		return err
	}

	folderID := file.FolderID
	if item.OriginalFolderID != nil {
		folderID = *item.OriginalFolderID
	}
	folderID = s.ensureActiveFolderOrRoot(ctx, tx, userID, folderID)

	count, err := s.files.CountByFolderAndOriginalName(ctx, tx, userID, folderID, item.OriginalName, item.OriginalID, false)
	if err != nil {
		return err
	}

	updates := map[string]interface{}{
		"deleted_at": nil,
		"deleted_by": nil,
		"folder_id":  folderID,
	}
	if count > 0 {
		updates["original_name"] = fmt.Sprintf("%s(restored)", item.OriginalName)
	}

	return s.files.UnscopedRestoreByIDAndUser(ctx, tx, item.OriginalID, userID, updates)
}

func (s *recycleBinService) restoreFolderItem(ctx context.Context, tx *gorm.DB, userID uint, item *models.RecycleBinItem) error {
	folder, err := s.folders.GetByIDAndUserUnscoped(ctx, tx, item.OriginalID, userID)
	if err != nil {
		return err
	}
	if folder.IsRoot != nil && *folder.IsRoot {
		return fmt.Errorf("root folder cannot be restored")
	}

	restoreParentID := uint(0)
	if folder.ParentID != nil {
		restoreParentID = *folder.ParentID
	} else if item.OriginalFolderID != nil {
		restoreParentID = *item.OriginalFolderID
	}
	restoreParentID = s.ensureActiveFolderOrRoot(ctx, tx, userID, restoreParentID)

	parent, err := s.folders.GetByIDAndUser(ctx, tx, restoreParentID, userID)
	if err != nil {
		return err
	}

	restoredName := item.OriginalName
	if restoredName == "" {
		restoredName = folder.Name
	}

	count, err := s.folders.CountByParentAndName(ctx, tx, userID, restoreParentID, restoredName, folder.ID)
	if err != nil {
		return err
	}
	if count > 0 {
		restoredName = fmt.Sprintf("%s(restored)", restoredName)
	}

	oldPath := folder.Path
	newPath := buildChildFolderPath(parent.Path, restoredName)
	affectedFolders, err := s.folders.ListByPathPrefix(ctx, tx, userID, folder.ID, oldPath, true)
	if err != nil {
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

		if err := s.folders.UpdateByIDUnscoped(ctx, tx, affectedFolders[i].ID, updates); err != nil {
			return err
		}
	}

	return s.files.UnscopedRestoreByFolderIDs(ctx, tx, userID, folderIDs, map[string]interface{}{"deleted_at": nil, "deleted_by": nil})
}

func (s *recycleBinService) ensureActiveFolderOrRoot(ctx context.Context, tx *gorm.DB, userID uint, folderID uint) uint {
	if folderID > 0 {
		if _, err := s.folders.GetByIDAndUser(ctx, tx, folderID, userID); err == nil {
			return folderID
		}
	}

	root, err := s.resolver.getOrCreateUserRootFolder(ctx, tx, userID)
	if err != nil {
		return folderID
	}
	return root.ID
}

func (s *recycleBinService) permanentDeleteFile(ctx context.Context, tx *gorm.DB, item *models.RecycleBinItem, userID uint) error {
	file, err := s.files.GetByIDAndUserUnscoped(ctx, tx, item.OriginalID, userID, true)
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

	if err := s.files.UnscopedDeleteByIDAndUser(ctx, tx, item.OriginalID, userID); err != nil {
		return err
	}

	if fileSize > 0 {
		if err := s.users.SubStorageUsed(ctx, tx, userID, fileSize); err != nil {
			return err
		}
	}

	if fileObjectID > 0 {
		if err := s.decrementFileObjectRef(ctx, tx, fileObjectID); err != nil {
			return err
		}
	}

	return nil
}

func (s *recycleBinService) permanentDeleteFolder(ctx context.Context, tx *gorm.DB, item *models.RecycleBinItem, userID uint) error {
	rootFolder, err := s.folders.GetByIDAndUserUnscoped(ctx, tx, item.OriginalID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if rootFolder.IsRoot != nil && *rootFolder.IsRoot {
		return fmt.Errorf("root folder cannot be deleted")
	}

	folders, err := s.folders.ListByPathPrefix(ctx, tx, userID, rootFolder.ID, rootFolder.Path, true)
	if err != nil {
		return err
	}
	if len(folders) == 0 {
		return nil
	}

	folderIDs := make([]uint, 0, len(folders))
	for _, f := range folders {
		folderIDs = append(folderIDs, f.ID)
	}

	files, err := s.files.ListByFolderIDs(ctx, tx, userID, folderIDs, true, true)
	if err != nil {
		return err
	}

	fileIDs := make([]uint, 0, len(files))
	for i := range files {
		fileIDs = append(fileIDs, files[i].ID)
		size := files[i].FileObject.FileSize
		tmp := models.RecycleBinItem{OriginalID: files[i].ID, FileObjectID: &files[i].FileObjectID, FileSize: &size}
		if err := s.permanentDeleteFile(ctx, tx, &tmp, userID); err != nil {
			return err
		}
	}

	if err := s.folders.UnscopedDeleteByIDs(ctx, tx, folderIDs); err != nil {
		return err
	}
	if err := s.recycle.DeleteByOriginalIDs(ctx, tx, userID, "file", fileIDs); err != nil {
		return err
	}
	if err := s.recycle.DeleteByOriginalIDs(ctx, tx, userID, "folder", folderIDs); err != nil {
		return err
	}

	return nil
}

func (s *recycleBinService) decrementFileObjectRef(ctx context.Context, tx *gorm.DB, fileObjectID uint) error {
	fileObj, err := s.fileObjects.GetByID(ctx, tx, fileObjectID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	if fileObj.RefCount <= 1 {
		_ = os.Remove(filepath.Join(config.AppConfig.Storage.BasePath, fileObj.FilePath))
		if fileObj.ThumbnailPath != "" {
			_ = os.Remove(filepath.Join(config.AppConfig.Storage.BasePath, fileObj.ThumbnailPath))
		}
		return s.fileObjects.DeleteByID(ctx, tx, fileObj.ID)
	}

	return s.fileObjects.DecrementRefCount(ctx, tx, fileObj.ID)
}
