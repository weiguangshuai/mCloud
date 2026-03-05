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

// FolderService 定义目录管理能力。
type FolderService interface {
	// GetOrCreateRootFolder 获取或初始化用户根目录。
	GetOrCreateRootFolder(ctx context.Context, userID uint) (models.Folder, error)
	// ResolveFolderID 解析目标目录，folderID=0 时返回根目录。
	ResolveFolderID(ctx context.Context, userID uint, folderID uint) (uint, error)
	// ListFolders 查询某个父目录下的子目录列表。
	ListFolders(ctx context.Context, userID uint, parentID *uint) ([]models.Folder, error)
	// CreateFolder 在指定父目录下创建子目录。
	CreateFolder(ctx context.Context, userID uint, name string, parentID uint) (models.Folder, error)
	// RenameFolder 重命名目录并同步更新全部后代路径。
	RenameFolder(ctx context.Context, userID uint, folderID uint, name string) (models.Folder, error)
	// DeleteFolder 删除目录（开启回收站时为软删除）。
	DeleteFolder(ctx context.Context, userID uint, folderID uint) error
}

// folderService 为 FolderService 的默认实现。
type folderService struct {
	txManager TxManager
	folders   repositories.FolderRepository
	files     repositories.FileRepository
	recycle   repositories.RecycleBinRepository
	resolver  folderResolver
}

// NewFolderService 创建目录服务实例。
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

// GetOrCreateRootFolder 获取用户根目录，不存在时自动补建。
func (s *folderService) GetOrCreateRootFolder(ctx context.Context, userID uint) (models.Folder, error) {
	root, err := s.resolver.getOrCreateUserRootFolder(ctx, nil, userID)
	if err != nil {
		return models.Folder{}, newAppError(http.StatusInternalServerError, "获取根目录失败", err)
	}
	return root, nil
}

// ResolveFolderID 将输入目录 ID 解析为当前用户可访问目录。
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

// ListFolders 查询父目录下的子目录列表。
func (s *folderService) ListFolders(ctx context.Context, userID uint, parentID *uint) ([]models.Folder, error) {
	rootFolder, err := s.resolver.getOrCreateUserRootFolder(ctx, nil, userID)
	if err != nil {
		return nil, newAppError(http.StatusInternalServerError, "获取根目录失败", err)
	}

	// 未传 parentID 时默认列根目录下内容。
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

	// 兼容历史数据：根目录下可能存在 legacy root 标记数据。
	includeLegacyRoot := targetParentID == rootFolder.ID
	list, err := s.folders.ListByParent(ctx, nil, userID, targetParentID, includeLegacyRoot)
	if err != nil {
		return nil, newAppError(http.StatusInternalServerError, "获取文件夹列表失败", err)
	}
	return list, nil
}

// CreateFolder 在指定父目录下创建新目录。
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

	// 同父目录下不允许重名。
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

// RenameFolder 重命名目录并同步更新后代路径。
func (s *folderService) RenameFolder(ctx context.Context, userID uint, folderID uint, name string) (models.Folder, error) {
	folder, err := s.folders.GetByIDAndUser(ctx, nil, folderID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Folder{}, newAppError(http.StatusNotFound, "文件夹不存在", nil)
		}
		return models.Folder{}, newAppError(http.StatusInternalServerError, "查询文件夹失败", err)
	}
	// 根目录名称固定，不允许改名。
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

	// 先更新当前目录路径，再级联更新全部后代路径前缀。
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

// DeleteFolder 删除目录；开启回收站时会保留恢复所需快照。
func (s *folderService) DeleteFolder(ctx context.Context, userID uint, folderID uint) error {
	folder, err := s.folders.GetByIDAndUser(ctx, nil, folderID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return newAppError(http.StatusNotFound, "文件夹不存在", nil)
		}
		return newAppError(http.StatusInternalServerError, "查询文件夹失败", err)
	}
	// 根目录是租户隔离锚点，禁止删除。
	if folder.IsRoot != nil && *folder.IsRoot {
		return newAppError(http.StatusBadRequest, "根目录不允许删除", nil)
	}

	err = s.txManager.WithTransaction(ctx, func(tx *gorm.DB) error {
		if config.AppConfig.RecycleBin.Enabled {
			// 开启回收站时先写入回收记录，再统一软删除目录及其子孙文件。
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
