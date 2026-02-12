package services

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"mcloud/config"
	"mcloud/models"
	"mcloud/repositories"
	"mcloud/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FileListOutput struct {
	Files      []models.File        `json:"files"`
	Pagination utils.PaginationData `json:"pagination"`
}

type InitChunkedUploadInput struct {
	FileName string
	FileSize int64
	FileMD5  string
	FolderID uint
}

type InitChunkedUploadOutput struct {
	UploadID    string `json:"upload_id,omitempty"`
	ChunkSize   int64  `json:"chunk_size,omitempty"`
	TotalChunks int    `json:"total_chunks,omitempty"`
	Status      string `json:"status,omitempty"`
	FileID      uint   `json:"file_id,omitempty"`
}

type UploadChunkOutput struct {
	ChunkIndex     int    `json:"chunk_index"`
	UploadedChunks int64  `json:"uploaded_chunks"`
	TotalChunks    int    `json:"total_chunks"`
	Message        string `json:"message,omitempty"`
}

type UploadStatusOutput struct {
	UploadID       string `json:"upload_id"`
	FileName       string `json:"file_name"`
	FileSize       int64  `json:"file_size"`
	TotalChunks    int    `json:"total_chunks"`
	UploadedChunks []int  `json:"uploaded_chunks"`
	Status         string `json:"status"`
}

type FileAccessOutput struct {
	File         models.File
	AbsPath      string
	ContentType  string
	DownloadName string
}

type ThumbnailBatchOutput struct {
	Items []map[string]interface{} `json:"items"`
}

type FileService interface {
	ListFiles(ctx context.Context, userID uint, folderID uint, page int, pageSize int, sortBy string, order string) (FileListOutput, error)
	UploadFile(ctx context.Context, userID uint, folderID uint, file multipart.File, header *multipart.FileHeader) (models.File, error)
	InitChunkedUpload(ctx context.Context, userID uint, in InitChunkedUploadInput) (InitChunkedUploadOutput, error)
	UploadChunk(ctx context.Context, userID uint, uploadID string, chunkIndex int, chunk multipart.File) (UploadChunkOutput, error)
	GetUploadStatus(ctx context.Context, userID uint, uploadID string) (UploadStatusOutput, error)
	CompleteUpload(ctx context.Context, userID uint, uploadID string) (models.File, error)
	GetDownloadInfo(ctx context.Context, userID uint, fileID uint) (FileAccessOutput, error)
	GetPreviewInfo(ctx context.Context, userID uint, fileID uint) (FileAccessOutput, error)
	GetThumbnailInfo(ctx context.Context, userID uint, fileID uint) (FileAccessOutput, error)
	DeleteFile(ctx context.Context, userID uint, fileID uint) error
	RenameFile(ctx context.Context, userID uint, fileID uint, name string) (models.File, error)
	MoveFile(ctx context.Context, userID uint, fileID uint, folderID uint) error
	BatchDeleteFiles(ctx context.Context, userID uint, fileIDs []uint) error
	BatchMoveFiles(ctx context.Context, userID uint, fileIDs []uint, folderID uint) error
	BatchGetThumbnails(ctx context.Context, userID uint, fileIDs []uint) (ThumbnailBatchOutput, error)
}

type fileService struct {
	txManager      TxManager
	users          repositories.UserRepository
	folders        repositories.FolderRepository
	files          repositories.FileRepository
	fileObjects    repositories.FileObjectRepository
	uploadTasks    repositories.UploadTaskRepository
	recycle        repositories.RecycleBinRepository
	uploadProgress repositories.UploadProgressRepository
	resolver       folderResolver
}

func NewFileService(
	txManager TxManager,
	users repositories.UserRepository,
	folders repositories.FolderRepository,
	files repositories.FileRepository,
	fileObjects repositories.FileObjectRepository,
	uploadTasks repositories.UploadTaskRepository,
	recycle repositories.RecycleBinRepository,
	uploadProgress repositories.UploadProgressRepository,
) FileService {
	return &fileService{
		txManager:      txManager,
		users:          users,
		folders:        folders,
		files:          files,
		fileObjects:    fileObjects,
		uploadTasks:    uploadTasks,
		recycle:        recycle,
		uploadProgress: uploadProgress,
		resolver:       folderResolver{folders: folders},
	}
}

func (s *fileService) ListFiles(ctx context.Context, userID uint, folderID uint, page int, pageSize int, sortBy string, order string) (FileListOutput, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > config.AppConfig.Pagination.MaxPageSize {
		pageSize = config.AppConfig.Pagination.DefaultPageSize
	}

	allowedSortFields := map[string]bool{"name": true, "created_at": true, "file_size": true}
	if !allowedSortFields[sortBy] {
		sortBy = "created_at"
	}
	if order != "asc" && order != "desc" {
		order = "desc"
	}

	resolvedFolderID, err := s.resolver.resolveFolderIDForUser(ctx, nil, userID, folderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return FileListOutput{}, newAppError(http.StatusNotFound, "目标文件夹不存在", nil)
		}
		return FileListOutput{}, newAppError(http.StatusInternalServerError, "校验目标文件夹失败", err)
	}

	rootFolder, err := s.resolver.getOrCreateUserRootFolder(ctx, nil, userID)
	if err != nil {
		return FileListOutput{}, newAppError(http.StatusInternalServerError, "获取根目录失败", err)
	}

	includeLegacyRoot := resolvedFolderID == rootFolder.ID
	total, err := s.files.CountByFolder(ctx, nil, userID, resolvedFolderID, rootFolder.ID, includeLegacyRoot)
	if err != nil {
		return FileListOutput{}, newAppError(http.StatusInternalServerError, "查询文件总数失败", err)
	}

	list, err := s.files.ListByFolder(ctx, nil, repositories.ListFilesInput{
		UserID:            userID,
		FolderID:          resolvedFolderID,
		RootFolderID:      rootFolder.ID,
		IncludeLegacyRoot: includeLegacyRoot,
		SortBy:            sortBy,
		Order:             order,
		Offset:            (page - 1) * pageSize,
		Limit:             pageSize,
	})
	if err != nil {
		return FileListOutput{}, newAppError(http.StatusInternalServerError, "查询文件列表失败", err)
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	return FileListOutput{
		Files: list,
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

func (s *fileService) UploadFile(ctx context.Context, userID uint, folderID uint, file multipart.File, header *multipart.FileHeader) (models.File, error) {
	if header.Size > config.AppConfig.Storage.MaxFileSize {
		return models.File{}, newAppError(http.StatusBadRequest, "文件大小超出限制", nil)
	}
	if !isFileExtensionAllowed(header.Filename) {
		return models.File{}, newAppError(http.StatusBadRequest, "不支持的文件类型", nil)
	}

	resolvedFolderID, err := s.resolver.resolveFolderIDForUser(ctx, nil, userID, folderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.File{}, newAppError(http.StatusNotFound, "目标文件夹不存在", nil)
		}
		return models.File{}, newAppError(http.StatusInternalServerError, "校验目标文件夹失败", err)
	}

	user, err := s.users.GetByID(ctx, nil, userID)
	if err != nil {
		return models.File{}, newAppError(http.StatusInternalServerError, "查询用户失败", err)
	}
	if user.StorageUsed+header.Size > user.StorageQuota {
		return models.File{}, newAppErrorWithData(http.StatusBadRequest, "存储空间不足", map[string]interface{}{
			"storage_quota":   user.StorageQuota,
			"storage_used":    user.StorageUsed,
			"available_space": user.StorageQuota - user.StorageUsed,
			"required_space":  header.Size,
		}, nil)
	}

	hasher := md5.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return models.File{}, newAppError(http.StatusInternalServerError, "计算文件MD5失败", err)
	}
	fileMD5 := hex.EncodeToString(hasher.Sum(nil))

	seeker, ok := file.(io.Seeker)
	if !ok {
		return models.File{}, newAppError(http.StatusInternalServerError, "文件流不支持重置", nil)
	}
	if _, err := seeker.Seek(0, io.SeekStart); err != nil {
		return models.File{}, newAppError(http.StatusInternalServerError, "重置文件流失败", err)
	}

	now := time.Now()
	fileUUID := uuid.New().String()
	storageName := fileUUID + "_" + sanitizeFilename(header.Filename)
	relDir := filepath.Join("files", fmt.Sprintf("%d", userID), now.Format("2006"), now.Format("01"))
	absDir := filepath.Join(config.AppConfig.Storage.BasePath, relDir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return models.File{}, newAppError(http.StatusInternalServerError, "创建存储目录失败", err)
	}

	absPath := filepath.Join(absDir, storageName)
	dst, err := os.Create(absPath)
	if err != nil {
		return models.File{}, newAppError(http.StatusInternalServerError, "创建文件失败", err)
	}
	if _, err := io.Copy(dst, file); err != nil {
		dst.Close()
		_ = os.Remove(absPath)
		return models.File{}, newAppError(http.StatusInternalServerError, "保存文件失败", err)
	}
	_ = dst.Close()

	isImage := IsImageFile(header.Filename)
	var thumbnailPath string
	var width, height int
	if isImage {
		w, h, dimErr := GetImageDimensions(absPath)
		if dimErr == nil {
			width, height = w, h
		}
		thumbName := fileUUID + "_thumb.jpg"
		thumbRelDir := filepath.Join("thumbnails", fmt.Sprintf("%d", userID), now.Format("2006"), now.Format("01"))
		thumbAbsDir := filepath.Join(config.AppConfig.Storage.BasePath, thumbRelDir)
		thumbAbsPath := filepath.Join(thumbAbsDir, thumbName)
		if err := GenerateThumbnail(absPath, thumbAbsPath); err == nil {
			thumbnailPath = filepath.Join(thumbRelDir, thumbName)
		}
	}

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	fileObj := models.FileObject{
		FilePath:      filepath.Join(relDir, storageName),
		ThumbnailPath: thumbnailPath,
		FileSize:      header.Size,
		MimeType:      mimeType,
		IsImage:       isImage,
		Width:         width,
		Height:        height,
		FileMD5:       fileMD5,
		RefCount:      1,
	}
	fileRecord := models.File{
		Name:         storageName,
		OriginalName: header.Filename,
		FolderID:     resolvedFolderID,
		UserID:       userID,
	}

	err = s.txManager.WithTransaction(ctx, func(tx *gorm.DB) error {
		if err := s.fileObjects.Create(ctx, tx, &fileObj); err != nil {
			return err
		}
		fileRecord.FileObjectID = fileObj.ID
		if err := s.files.Create(ctx, tx, &fileRecord); err != nil {
			return err
		}
		return s.users.AddStorageUsed(ctx, tx, userID, header.Size)
	})
	if err != nil {
		_ = os.Remove(absPath)
		if thumbnailPath != "" {
			_ = os.Remove(filepath.Join(config.AppConfig.Storage.BasePath, thumbnailPath))
		}
		return models.File{}, newAppError(http.StatusInternalServerError, "保存文件记录失败", err)
	}

	fileRecord.FileObject = fileObj
	return fileRecord, nil
}

func (s *fileService) InitChunkedUpload(ctx context.Context, userID uint, in InitChunkedUploadInput) (InitChunkedUploadOutput, error) {
	if !isFileExtensionAllowed(in.FileName) {
		return InitChunkedUploadOutput{}, newAppError(http.StatusBadRequest, "不支持的文件类型", nil)
	}

	resolvedFolderID, err := s.resolver.resolveFolderIDForUser(ctx, nil, userID, in.FolderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return InitChunkedUploadOutput{}, newAppError(http.StatusNotFound, "目标文件夹不存在", nil)
		}
		return InitChunkedUploadOutput{}, newAppError(http.StatusInternalServerError, "校验目标文件夹失败", err)
	}

	user, err := s.users.GetByID(ctx, nil, userID)
	if err != nil {
		return InitChunkedUploadOutput{}, newAppError(http.StatusInternalServerError, "查询用户失败", err)
	}
	if user.StorageUsed+in.FileSize > user.StorageQuota {
		return InitChunkedUploadOutput{}, newAppErrorWithData(http.StatusBadRequest, "存储空间不足", map[string]interface{}{
			"storage_quota":   user.StorageQuota,
			"storage_used":    user.StorageUsed,
			"available_space": user.StorageQuota - user.StorageUsed,
			"required_space":  in.FileSize,
		}, nil)
	}

	existingObj, err := s.files.FindByUserAndMD5(ctx, nil, userID, in.FileMD5)
	if err == nil {
		var newFile models.File
		err = s.txManager.WithTransaction(ctx, func(tx *gorm.DB) error {
			if err := s.fileObjects.IncrementRefCount(ctx, tx, existingObj.ID); err != nil {
				return err
			}
			newFile = models.File{
				Name:         filepath.Base(existingObj.FilePath),
				OriginalName: in.FileName,
				FolderID:     resolvedFolderID,
				UserID:       userID,
				FileObjectID: existingObj.ID,
			}
			if err := s.files.Create(ctx, tx, &newFile); err != nil {
				return err
			}
			return s.users.AddStorageUsed(ctx, tx, userID, existingObj.FileSize)
		})
		if err != nil {
			return InitChunkedUploadOutput{}, newAppError(http.StatusInternalServerError, "秒传失败", err)
		}
		return InitChunkedUploadOutput{Status: "instant_upload", FileID: newFile.ID}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return InitChunkedUploadOutput{}, newAppError(http.StatusInternalServerError, "秒传检查失败", err)
	}

	chunkSize := config.AppConfig.Storage.ChunkSize
	totalChunks := int(math.Ceil(float64(in.FileSize) / float64(chunkSize)))
	uploadID := uuid.New().String()
	tempDir := filepath.Join(config.AppConfig.Storage.BasePath, "temp", uploadID)
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return InitChunkedUploadOutput{}, newAppError(http.StatusInternalServerError, "创建临时目录失败", err)
	}

	task := models.UploadTask{
		UploadID:    uploadID,
		UserID:      userID,
		FolderID:    resolvedFolderID,
		FileName:    in.FileName,
		FileSize:    in.FileSize,
		FileMD5:     in.FileMD5,
		TotalChunks: totalChunks,
		Status:      "uploading",
		TempDir:     tempDir,
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}
	if err := s.uploadTasks.Create(ctx, nil, &task); err != nil {
		return InitChunkedUploadOutput{}, newAppError(http.StatusInternalServerError, "创建上传任务失败", err)
	}

	return InitChunkedUploadOutput{UploadID: uploadID, ChunkSize: chunkSize, TotalChunks: totalChunks}, nil
}

func (s *fileService) UploadChunk(ctx context.Context, userID uint, uploadID string, chunkIndex int, chunk multipart.File) (UploadChunkOutput, error) {
	task, err := s.uploadTasks.GetByUploadID(ctx, nil, uploadID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return UploadChunkOutput{}, newAppError(http.StatusNotFound, "上传任务不存在", nil)
		}
		return UploadChunkOutput{}, newAppError(http.StatusInternalServerError, "查询上传任务失败", err)
	}
	if task.UserID != userID {
		return UploadChunkOutput{}, newAppError(http.StatusForbidden, "无权操作此上传任务", nil)
	}

	uploaded, err := s.uploadProgress.IsChunkUploaded(ctx, uploadID, chunkIndex)
	if err != nil {
		return UploadChunkOutput{}, newAppError(http.StatusInternalServerError, "检查分片状态失败", err)
	}
	if uploaded {
		uploadedCount, _ := s.uploadProgress.UploadedCount(ctx, uploadID)
		return UploadChunkOutput{
			ChunkIndex:     chunkIndex,
			UploadedChunks: uploadedCount,
			TotalChunks:    task.TotalChunks,
			Message:        "分片已存在",
		}, nil
	}

	chunkPath := filepath.Join(task.TempDir, fmt.Sprintf("chunk_%d", chunkIndex))
	dst, err := os.Create(chunkPath)
	if err != nil {
		return UploadChunkOutput{}, newAppError(http.StatusInternalServerError, "保存分片失败", err)
	}
	if _, err := io.Copy(dst, chunk); err != nil {
		dst.Close()
		_ = os.Remove(chunkPath)
		return UploadChunkOutput{}, newAppError(http.StatusInternalServerError, "写入分片失败", err)
	}
	_ = dst.Close()

	if err := s.uploadProgress.AddChunk(ctx, uploadID, chunkIndex, config.AppConfig.Redis.UploadTaskExpire); err != nil {
		return UploadChunkOutput{}, newAppError(http.StatusInternalServerError, "记录分片进度失败", err)
	}

	uploadedCount, err := s.uploadProgress.UploadedCount(ctx, uploadID)
	if err != nil {
		return UploadChunkOutput{}, newAppError(http.StatusInternalServerError, "查询分片进度失败", err)
	}

	return UploadChunkOutput{ChunkIndex: chunkIndex, UploadedChunks: uploadedCount, TotalChunks: task.TotalChunks}, nil
}

func (s *fileService) GetUploadStatus(ctx context.Context, userID uint, uploadID string) (UploadStatusOutput, error) {
	task, err := s.uploadTasks.GetByUploadIDAndUser(ctx, nil, uploadID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return UploadStatusOutput{}, newAppError(http.StatusNotFound, "上传任务不存在", nil)
		}
		return UploadStatusOutput{}, newAppError(http.StatusInternalServerError, "查询上传任务失败", err)
	}

	uploadedChunks, err := s.uploadProgress.UploadedChunks(ctx, uploadID)
	if err != nil {
		return UploadStatusOutput{}, newAppError(http.StatusInternalServerError, "查询分片进度失败", err)
	}
	sort.Ints(uploadedChunks)

	return UploadStatusOutput{
		UploadID:       uploadID,
		FileName:       task.FileName,
		FileSize:       task.FileSize,
		TotalChunks:    task.TotalChunks,
		UploadedChunks: uploadedChunks,
		Status:         task.Status,
	}, nil
}

func (s *fileService) CompleteUpload(ctx context.Context, userID uint, uploadID string) (models.File, error) {
	task, err := s.uploadTasks.GetByUploadIDAndUser(ctx, nil, uploadID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.File{}, newAppError(http.StatusNotFound, "上传任务不存在", nil)
		}
		return models.File{}, newAppError(http.StatusInternalServerError, "查询上传任务失败", err)
	}

	resolvedFolderID, err := s.resolver.resolveFolderIDForUser(ctx, nil, userID, task.FolderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.File{}, newAppError(http.StatusNotFound, "目标文件夹不存在", nil)
		}
		return models.File{}, newAppError(http.StatusInternalServerError, "校验目标文件夹失败", err)
	}

	uploadedCount, err := s.uploadProgress.UploadedCount(ctx, uploadID)
	if err != nil {
		return models.File{}, newAppError(http.StatusInternalServerError, "查询分片进度失败", err)
	}
	if int(uploadedCount) < task.TotalChunks {
		return models.File{}, newAppError(http.StatusBadRequest, fmt.Sprintf("分片未全部上传，已上传 %d/%d", uploadedCount, task.TotalChunks), nil)
	}

	now := time.Now()
	fileUUID := uuid.New().String()
	storageName := fileUUID + "_" + sanitizeFilename(task.FileName)
	relDir := filepath.Join("files", fmt.Sprintf("%d", userID), now.Format("2006"), now.Format("01"))
	absDir := filepath.Join(config.AppConfig.Storage.BasePath, relDir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return models.File{}, newAppError(http.StatusInternalServerError, "创建存储目录失败", err)
	}

	finalPath := filepath.Join(absDir, storageName)
	finalFile, err := os.Create(finalPath)
	if err != nil {
		return models.File{}, newAppError(http.StatusInternalServerError, "创建目标文件失败", err)
	}

	for i := 0; i < task.TotalChunks; i++ {
		chunkPath := filepath.Join(task.TempDir, fmt.Sprintf("chunk_%d", i))
		chunkData, err := os.ReadFile(chunkPath)
		if err != nil {
			finalFile.Close()
			_ = os.Remove(finalPath)
			return models.File{}, newAppError(http.StatusInternalServerError, fmt.Sprintf("读取分片 %d 失败", i), err)
		}
		if _, err := finalFile.Write(chunkData); err != nil {
			finalFile.Close()
			_ = os.Remove(finalPath)
			return models.File{}, newAppError(http.StatusInternalServerError, "合并文件失败", err)
		}
	}

	if _, err := finalFile.Seek(0, io.SeekStart); err != nil {
		finalFile.Close()
		_ = os.Remove(finalPath)
		return models.File{}, newAppError(http.StatusInternalServerError, "重置文件游标失败", err)
	}
	hasher := md5.New()
	if _, err := io.Copy(hasher, finalFile); err != nil {
		finalFile.Close()
		_ = os.Remove(finalPath)
		return models.File{}, newAppError(http.StatusInternalServerError, "校验文件MD5失败", err)
	}
	_ = finalFile.Close()

	actualMD5 := hex.EncodeToString(hasher.Sum(nil))
	if actualMD5 != task.FileMD5 {
		_ = os.Remove(finalPath)
		return models.File{}, newAppError(http.StatusBadRequest, "文件完整性校验失败，MD5不匹配", nil)
	}

	isImage := IsImageFile(task.FileName)
	var thumbnailPath string
	var width, height int
	if isImage {
		w, h, dimErr := GetImageDimensions(finalPath)
		if dimErr == nil {
			width, height = w, h
		}
		thumbName := fileUUID + "_thumb.jpg"
		thumbRelDir := filepath.Join("thumbnails", fmt.Sprintf("%d", userID), now.Format("2006"), now.Format("01"))
		thumbAbsPath := filepath.Join(config.AppConfig.Storage.BasePath, thumbRelDir, thumbName)
		if err := GenerateThumbnail(finalPath, thumbAbsPath); err == nil {
			thumbnailPath = filepath.Join(thumbRelDir, thumbName)
		}
	}

	fileObj := models.FileObject{
		FilePath:      filepath.Join(relDir, storageName),
		ThumbnailPath: thumbnailPath,
		FileSize:      task.FileSize,
		MimeType:      getMimeType(filepath.Ext(task.FileName)),
		IsImage:       isImage,
		Width:         width,
		Height:        height,
		FileMD5:       task.FileMD5,
		RefCount:      1,
	}
	fileRecord := models.File{
		Name:         storageName,
		OriginalName: task.FileName,
		FolderID:     resolvedFolderID,
		UserID:       userID,
	}

	err = s.txManager.WithTransaction(ctx, func(tx *gorm.DB) error {
		if err := s.fileObjects.Create(ctx, tx, &fileObj); err != nil {
			return err
		}
		fileRecord.FileObjectID = fileObj.ID
		if err := s.files.Create(ctx, tx, &fileRecord); err != nil {
			return err
		}
		if err := s.users.AddStorageUsed(ctx, tx, userID, task.FileSize); err != nil {
			return err
		}
		return s.uploadTasks.UpdateStatus(ctx, tx, uploadID, "completed")
	})
	if err != nil {
		_ = os.Remove(finalPath)
		if thumbnailPath != "" {
			_ = os.Remove(filepath.Join(config.AppConfig.Storage.BasePath, thumbnailPath))
		}
		return models.File{}, newAppError(http.StatusInternalServerError, "保存文件记录失败", err)
	}

	_ = os.RemoveAll(task.TempDir)
	_ = s.uploadProgress.Clear(ctx, uploadID)
	fileRecord.FileObject = fileObj
	return fileRecord, nil
}

func (s *fileService) getFileAccessInfo(ctx context.Context, userID uint, fileID uint) (FileAccessOutput, error) {
	file, err := s.files.GetByIDAndUser(ctx, nil, fileID, userID, true)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return FileAccessOutput{}, newAppError(http.StatusNotFound, "文件不存在", nil)
		}
		return FileAccessOutput{}, newAppError(http.StatusInternalServerError, "查询文件失败", err)
	}

	absPath := filepath.Join(config.AppConfig.Storage.BasePath, file.FileObject.FilePath)
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return FileAccessOutput{}, newAppError(http.StatusNotFound, "文件不存在于存储中", nil)
	}

	return FileAccessOutput{File: file, AbsPath: absPath, ContentType: file.FileObject.MimeType, DownloadName: file.OriginalName}, nil
}

func (s *fileService) GetDownloadInfo(ctx context.Context, userID uint, fileID uint) (FileAccessOutput, error) {
	return s.getFileAccessInfo(ctx, userID, fileID)
}

func (s *fileService) GetPreviewInfo(ctx context.Context, userID uint, fileID uint) (FileAccessOutput, error) {
	return s.getFileAccessInfo(ctx, userID, fileID)
}

func (s *fileService) GetThumbnailInfo(ctx context.Context, userID uint, fileID uint) (FileAccessOutput, error) {
	file, err := s.files.GetByIDAndUser(ctx, nil, fileID, userID, true)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return FileAccessOutput{}, newAppError(http.StatusNotFound, "文件不存在", nil)
		}
		return FileAccessOutput{}, newAppError(http.StatusInternalServerError, "查询文件失败", err)
	}
	if file.FileObject.ThumbnailPath == "" {
		return FileAccessOutput{}, newAppError(http.StatusNotFound, "缩略图不存在", nil)
	}
	absPath := filepath.Join(config.AppConfig.Storage.BasePath, file.FileObject.ThumbnailPath)
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return FileAccessOutput{}, newAppError(http.StatusNotFound, "缩略图文件不存在", nil)
	}
	return FileAccessOutput{File: file, AbsPath: absPath, ContentType: "image/jpeg"}, nil
}

func (s *fileService) DeleteFile(ctx context.Context, userID uint, fileID uint) error {
	file, err := s.files.GetByIDAndUser(ctx, nil, fileID, userID, true)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return newAppError(http.StatusNotFound, "文件不存在", nil)
		}
		return newAppError(http.StatusInternalServerError, "查询文件失败", err)
	}

	err = s.txManager.WithTransaction(ctx, func(tx *gorm.DB) error {
		if config.AppConfig.RecycleBin.Enabled {
			metadata, _ := json.Marshal(map[string]interface{}{
				"mime_type":      file.FileObject.MimeType,
				"thumbnail_path": file.FileObject.ThumbnailPath,
				"is_image":       file.FileObject.IsImage,
				"width":          file.FileObject.Width,
				"height":         file.FileObject.Height,
				"file_md5":       file.FileObject.FileMD5,
				"file_object_id": file.FileObjectID,
			})
			fileSize := file.FileObject.FileSize
			item := models.RecycleBinItem{
				UserID:           userID,
				OriginalID:       file.ID,
				OriginalType:     "file",
				OriginalName:     file.OriginalName,
				OriginalPath:     file.FileObject.FilePath,
				OriginalFolderID: &file.FolderID,
				FileObjectID:     &file.FileObjectID,
				FileSize:         &fileSize,
				ExpiresAt:        time.Now().AddDate(0, 0, config.AppConfig.RecycleBin.RetentionDays),
				Metadata:         string(metadata),
			}
			if err := s.recycle.Create(ctx, tx, &item); err != nil {
				return err
			}
		}
		return s.files.SoftDeleteByIDAndUser(ctx, tx, file.ID, userID)
	})
	if err != nil {
		return newAppError(http.StatusInternalServerError, "删除文件失败", err)
	}
	return nil
}

func (s *fileService) RenameFile(ctx context.Context, userID uint, fileID uint, name string) (models.File, error) {
	file, err := s.files.GetByIDAndUser(ctx, nil, fileID, userID, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.File{}, newAppError(http.StatusNotFound, "文件不存在", nil)
		}
		return models.File{}, newAppError(http.StatusInternalServerError, "查询文件失败", err)
	}
	if err := s.files.UpdateByIDAndUser(ctx, nil, fileID, userID, map[string]interface{}{"original_name": name}); err != nil {
		return models.File{}, newAppError(http.StatusInternalServerError, "重命名文件失败", err)
	}
	file.OriginalName = name
	return file, nil
}

func (s *fileService) MoveFile(ctx context.Context, userID uint, fileID uint, folderID uint) error {
	if _, err := s.files.GetByIDAndUser(ctx, nil, fileID, userID, false); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return newAppError(http.StatusNotFound, "文件不存在", nil)
		}
		return newAppError(http.StatusInternalServerError, "查询文件失败", err)
	}

	resolvedFolderID, err := s.resolver.resolveFolderIDForUser(ctx, nil, userID, folderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return newAppError(http.StatusNotFound, "目标文件夹不存在", nil)
		}
		return newAppError(http.StatusInternalServerError, "校验目标文件夹失败", err)
	}

	if err := s.files.UpdateByIDAndUser(ctx, nil, fileID, userID, map[string]interface{}{"folder_id": resolvedFolderID}); err != nil {
		return newAppError(http.StatusInternalServerError, "移动文件失败", err)
	}
	return nil
}

func (s *fileService) BatchDeleteFiles(ctx context.Context, userID uint, fileIDs []uint) error {
	err := s.txManager.WithTransaction(ctx, func(tx *gorm.DB) error {
		for _, fileID := range fileIDs {
			file, err := s.files.GetByIDAndUser(ctx, tx, fileID, userID, true)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return err
			}
			if config.AppConfig.RecycleBin.Enabled {
				metadata, _ := json.Marshal(map[string]interface{}{
					"mime_type":      file.FileObject.MimeType,
					"is_image":       file.FileObject.IsImage,
					"file_md5":       file.FileObject.FileMD5,
					"file_object_id": file.FileObjectID,
				})
				fileSize := file.FileObject.FileSize
				item := models.RecycleBinItem{
					UserID:           userID,
					OriginalID:       file.ID,
					OriginalType:     "file",
					OriginalName:     file.OriginalName,
					OriginalPath:     file.FileObject.FilePath,
					OriginalFolderID: &file.FolderID,
					FileObjectID:     &file.FileObjectID,
					FileSize:         &fileSize,
					ExpiresAt:        time.Now().AddDate(0, 0, config.AppConfig.RecycleBin.RetentionDays),
					Metadata:         string(metadata),
				}
				if err := s.recycle.Create(ctx, tx, &item); err != nil {
					return err
				}
			}
			if err := s.files.SoftDeleteByIDAndUser(ctx, tx, file.ID, userID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return newAppError(http.StatusInternalServerError, "批量删除失败", err)
	}
	return nil
}

func (s *fileService) BatchMoveFiles(ctx context.Context, userID uint, fileIDs []uint, folderID uint) error {
	resolvedFolderID, err := s.resolver.resolveFolderIDForUser(ctx, nil, userID, folderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return newAppError(http.StatusNotFound, "目标文件夹不存在", nil)
		}
		return newAppError(http.StatusInternalServerError, "校验目标文件夹失败", err)
	}

	if err := s.files.UpdateByIDsAndUser(ctx, nil, fileIDs, userID, map[string]interface{}{"folder_id": resolvedFolderID}); err != nil {
		return newAppError(http.StatusInternalServerError, "批量移动失败", err)
	}
	return nil
}

func (s *fileService) BatchGetThumbnails(ctx context.Context, userID uint, fileIDs []uint) (ThumbnailBatchOutput, error) {
	fileRecords, err := s.files.GetByIDsAndUser(ctx, nil, userID, fileIDs, true)
	if err != nil {
		return ThumbnailBatchOutput{}, newAppError(http.StatusInternalServerError, "查询缩略图信息失败", err)
	}

	fileMap := make(map[uint]models.File, len(fileRecords))
	for _, f := range fileRecords {
		fileMap[f.ID] = f
	}

	items := make([]map[string]interface{}, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		f, ok := fileMap[fileID]
		if !ok {
			items = append(items, map[string]interface{}{
				"file_id":       fileID,
				"exists":        false,
				"has_thumbnail": false,
			})
			continue
		}
		hasThumb := f.FileObject.ThumbnailPath != ""
		items = append(items, map[string]interface{}{
			"file_id":       fileID,
			"exists":        true,
			"has_thumbnail": hasThumb,
			"thumbnail_url": fmt.Sprintf("/api/files/%d/thumbnail", fileID),
		})
	}
	return ThumbnailBatchOutput{Items: items}, nil
}
