package handlers

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mcloud/config"
	"mcloud/database"
	"mcloud/models"
	"mcloud/services"
	"mcloud/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func validateFolderOwnership(userID uint, folderID uint) error {
	if folderID == 0 {
		return nil
	}

	var folder models.Folder
	return database.DB.Where("id = ? AND user_id = ?", folderID, userID).First(&folder).Error
}

func respondFolderValidationError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		utils.Error(c, http.StatusNotFound, "目标文件夹不存在")
	} else {
		utils.Error(c, http.StatusInternalServerError, "校验目标文件夹失败")
	}
	return true
}

// ListFiles 获取文件列表（支持分页）
func ListFiles(c *gin.Context) {
	userID := c.GetUint("user_id")
	folderID, _ := strconv.ParseUint(c.DefaultQuery("folder_id", "0"), 10, 32)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	sortBy := c.DefaultQuery("sort_by", config.AppConfig.Pagination.DefaultSortBy)
	order := c.DefaultQuery("order", config.AppConfig.Pagination.DefaultOrder)

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

	var total int64
	database.DB.Model(&models.File{}).
		Where("user_id = ? AND folder_id = ?", userID, uint(folderID)).
		Count(&total)

	var files []models.File
	database.DB.Preload("FileObject").
		Where("user_id = ? AND folder_id = ?", userID, uint(folderID)).
		Order(sortBy + " " + order).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&files)

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	utils.Success(c, gin.H{
		"files": files,
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

// UploadFile 小文件直接上传（< 5MB）
func UploadFile(c *gin.Context) {
	userID := c.GetUint("user_id")
	folderIDStr := c.PostForm("folder_id")
	folderID, _ := strconv.ParseUint(folderIDStr, 10, 32)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "获取上传文件失败")
		return
	}
	defer file.Close()

	// 检查文件大小
	if header.Size > config.AppConfig.Storage.MaxFileSize {
		utils.Error(c, http.StatusBadRequest, "文件大小超出限制")
		return
	}

	// 检查存储配额
	var user models.User
	database.DB.First(&user, userID)
	if user.StorageUsed+header.Size > user.StorageQuota {
		utils.ErrorWithData(c, http.StatusBadRequest, "存储空间不足", gin.H{
			"storage_quota":   user.StorageQuota,
			"storage_used":    user.StorageUsed,
			"available_space": user.StorageQuota - user.StorageUsed,
			"required_space":  header.Size,
		})
		return
	}

	// 计算 MD5
	hasher := md5.New()
	if _, err := io.Copy(hasher, file); err != nil {
		utils.Error(c, http.StatusInternalServerError, "计算文件MD5失败")
		return
	}
	fileMD5 := hex.EncodeToString(hasher.Sum(nil))
	file.Seek(0, 0)

	// 生成存储路径
	now := time.Now()
	fileUUID := uuid.New().String()
	storageName := fileUUID + "_" + sanitizeFilename(header.Filename)
	relDir := filepath.Join("files", fmt.Sprintf("%d", userID), now.Format("2006"), now.Format("01"))
	absDir := filepath.Join(config.AppConfig.Storage.BasePath, relDir)

	if err := os.MkdirAll(absDir, 0755); err != nil {
		utils.Error(c, http.StatusInternalServerError, "创建存储目录失败")
		return
	}

	absPath := filepath.Join(absDir, storageName)
	dst, err := os.Create(absPath)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "创建文件失败")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		utils.Error(c, http.StatusInternalServerError, "保存文件失败")
		return
	}

	// 判断是否为图片
	isImage := services.IsImageFile(header.Filename)
	var thumbnailPath string
	var width, height int

	if isImage {
		w, h, err := services.GetImageDimensions(absPath)
		if err == nil {
			width, height = w, h
		}

		thumbName := fileUUID + "_thumb.jpg"
		thumbRelDir := filepath.Join("thumbnails", fmt.Sprintf("%d", userID), now.Format("2006"), now.Format("01"))
		thumbAbsDir := filepath.Join(config.AppConfig.Storage.BasePath, thumbRelDir)
		thumbAbsPath := filepath.Join(thumbAbsDir, thumbName)

		if err := services.GenerateThumbnail(absPath, thumbAbsPath); err == nil {
			thumbnailPath = filepath.Join(thumbRelDir, thumbName)
		}
	}

	// 获取 MIME 类型
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// 创建物理文件对象
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
	if err := database.DB.Create(&fileObj).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "保存文件对象失败")
		return
	}

	// 创建逻辑文件记录
	fileRecord := models.File{
		Name:         storageName,
		OriginalName: header.Filename,
		FolderID:     uint(folderID),
		UserID:       userID,
		FileObjectID: fileObj.ID,
	}
	if err := database.DB.Create(&fileRecord).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "保存文件记录失败")
		return
	}

	// 更新用户存储使用量
	database.DB.Model(&user).Update("storage_used", user.StorageUsed+header.Size)

	fileRecord.FileObject = fileObj
	utils.Success(c, fileRecord)
}

func sanitizeFilename(name string) string {
	// 移除路径分隔符，保留文件名
	name = filepath.Base(name)
	// 替换不安全字符
	replacer := strings.NewReplacer(
		"..", "_", "/", "_", "\\", "_",
	)
	return replacer.Replace(name)
}

// getMimeType 根据扩展名获取 MIME 类型
func getMimeType(ext string) string {
	mimeTypes := map[string]string{
		".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png",
		".gif": "image/gif", ".bmp": "image/bmp", ".webp": "image/webp",
		".pdf": "application/pdf", ".txt": "text/plain",
		".mp4": "video/mp4", ".mp3": "audio/mpeg",
		".zip": "application/zip", ".doc": "application/msword",
	}
	if mt, ok := mimeTypes[strings.ToLower(ext)]; ok {
		return mt
	}
	return "application/octet-stream"
}

// InitChunkedUpload 初始化分片上传
func InitChunkedUpload(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req struct {
		FileName string `json:"file_name" binding:"required"`
		FileSize int64  `json:"file_size" binding:"required"`
		FileMD5  string `json:"file_md5" binding:"required"`
		FolderID uint   `json:"folder_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if respondFolderValidationError(c, validateFolderOwnership(userID, req.FolderID)) {
		return
	}

	// 检查存储配额
	var user models.User
	database.DB.First(&user, userID)
	if user.StorageUsed+req.FileSize > user.StorageQuota {
		utils.ErrorWithData(c, http.StatusBadRequest, "存储空间不足", gin.H{
			"storage_quota":   user.StorageQuota,
			"storage_used":    user.StorageUsed,
			"available_space": user.StorageQuota - user.StorageUsed,
			"required_space":  req.FileSize,
		})
		return
	}

	// 秒传检查（用户范围内，通过 file_objects JOIN files 查询）
	var existingFileObj models.FileObject
	err := database.DB.
		Joins("JOIN files ON files.file_object_id = file_objects.id").
		Where("files.user_id = ? AND file_objects.file_md5 = ? AND files.deleted_at IS NULL", userID, req.FileMD5).
		First(&existingFileObj).Error
	if err == nil {
		// 秒传：复用 FileObject，ref_count +1
		database.DB.Model(&existingFileObj).Update("ref_count", existingFileObj.RefCount+1)
		newFile := models.File{
			Name:         existingFileObj.FilePath[strings.LastIndex(existingFileObj.FilePath, string(os.PathSeparator))+1:],
			OriginalName: req.FileName,
			FolderID:     req.FolderID,
			UserID:       userID,
			FileObjectID: existingFileObj.ID,
		}
		database.DB.Create(&newFile)
		database.DB.Model(&user).Update("storage_used", user.StorageUsed+existingFileObj.FileSize)

		utils.SuccessWithMessage(c, "秒传成功", gin.H{
			"status":  "instant_upload",
			"file_id": newFile.ID,
		})
		return
	}

	// 创建上传任务
	chunkSize := config.AppConfig.Storage.ChunkSize
	totalChunks := int(math.Ceil(float64(req.FileSize) / float64(chunkSize)))
	uploadID := uuid.New().String()

	tempDir := filepath.Join(config.AppConfig.Storage.BasePath, "temp", uploadID)
	os.MkdirAll(tempDir, 0755)

	task := models.UploadTask{
		UploadID:    uploadID,
		UserID:      userID,
		FolderID:    req.FolderID,
		FileName:    req.FileName,
		FileSize:    req.FileSize,
		FileMD5:     req.FileMD5,
		TotalChunks: totalChunks,
		Status:      "uploading",
		TempDir:     tempDir,
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}
	database.DB.Create(&task)

	utils.Success(c, gin.H{
		"upload_id":    uploadID,
		"chunk_size":   chunkSize,
		"total_chunks": totalChunks,
	})
}

// UploadChunk 上传文件分片
func UploadChunk(c *gin.Context) {
	uploadID := c.PostForm("upload_id")
	chunkIndex, _ := strconv.Atoi(c.PostForm("chunk_index"))

	var task models.UploadTask
	if err := database.DB.Where("upload_id = ?", uploadID).First(&task).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "上传任务不存在")
		return
	}

	userID := c.GetUint("user_id")
	if task.UserID != userID {
		utils.Error(c, http.StatusForbidden, "无权操作此上传任务")
		return
	}

	// 检查分片是否已上传（Redis）
	ctx := context.Background()
	chunkKey := fmt.Sprintf("upload:%s:chunks", uploadID)
	isMember, _ := database.RedisClient.SIsMember(ctx, chunkKey, chunkIndex).Result()
	if isMember {
		utils.Success(c, gin.H{"message": "分片已存在", "chunk_index": chunkIndex})
		return
	}

	file, _, err := c.Request.FormFile("chunk")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "获取分片数据失败")
		return
	}
	defer file.Close()

	chunkPath := filepath.Join(task.TempDir, fmt.Sprintf("chunk_%d", chunkIndex))
	dst, err := os.Create(chunkPath)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "保存分片失败")
		return
	}
	defer dst.Close()
	io.Copy(dst, file)

	// 记录到 Redis
	database.RedisClient.SAdd(ctx, chunkKey, chunkIndex)
	database.RedisClient.Expire(ctx, chunkKey, time.Duration(config.AppConfig.Redis.UploadTaskExpire)*time.Second)

	uploaded, _ := database.RedisClient.SCard(ctx, chunkKey).Result()

	utils.Success(c, gin.H{
		"chunk_index":     chunkIndex,
		"uploaded_chunks": uploaded,
		"total_chunks":    task.TotalChunks,
	})
}

// GetUploadStatus 获取上传进度
func GetUploadStatus(c *gin.Context) {
	uploadID := c.Param("upload_id")
	userID := c.GetUint("user_id")

	var task models.UploadTask
	if err := database.DB.Where("upload_id = ? AND user_id = ?", uploadID, userID).First(&task).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "上传任务不存在")
		return
	}

	ctx := context.Background()
	chunkKey := fmt.Sprintf("upload:%s:chunks", uploadID)
	members, _ := database.RedisClient.SMembers(ctx, chunkKey).Result()

	uploadedChunks := make([]int, 0, len(members))
	for _, m := range members {
		idx, _ := strconv.Atoi(m)
		uploadedChunks = append(uploadedChunks, idx)
	}

	utils.Success(c, gin.H{
		"upload_id":       uploadID,
		"file_name":       task.FileName,
		"file_size":       task.FileSize,
		"total_chunks":    task.TotalChunks,
		"uploaded_chunks": uploadedChunks,
		"status":          task.Status,
	})
}

// CompleteUpload 完成分片上传，合并文件
func CompleteUpload(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req struct {
		UploadID string `json:"upload_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	var task models.UploadTask
	if err := database.DB.Where("upload_id = ? AND user_id = ?", req.UploadID, userID).First(&task).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "上传任务不存在")
		return
	}
	if respondFolderValidationError(c, validateFolderOwnership(userID, task.FolderID)) {
		return
	}

	// 检查所有分片是否已上传
	ctx := context.Background()
	chunkKey := fmt.Sprintf("upload:%s:chunks", req.UploadID)
	uploaded, _ := database.RedisClient.SCard(ctx, chunkKey).Result()
	if int(uploaded) < task.TotalChunks {
		utils.Error(c, http.StatusBadRequest, fmt.Sprintf("分片未全部上传，已上传 %d/%d", uploaded, task.TotalChunks))
		return
	}

	// 合并分片
	now := time.Now()
	fileUUID := uuid.New().String()
	storageName := fileUUID + "_" + sanitizeFilename(task.FileName)
	relDir := filepath.Join("files", fmt.Sprintf("%d", userID), now.Format("2006"), now.Format("01"))
	absDir := filepath.Join(config.AppConfig.Storage.BasePath, relDir)
	os.MkdirAll(absDir, 0755)

	finalPath := filepath.Join(absDir, storageName)
	finalFile, err := os.Create(finalPath)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "创建目标文件失败")
		return
	}
	defer finalFile.Close()

	for i := 0; i < task.TotalChunks; i++ {
		chunkPath := filepath.Join(task.TempDir, fmt.Sprintf("chunk_%d", i))
		chunkData, err := os.ReadFile(chunkPath)
		if err != nil {
			utils.Error(c, http.StatusInternalServerError, fmt.Sprintf("读取分片 %d 失败", i))
			return
		}
		finalFile.Write(chunkData)
	}

	// 验证 MD5
	finalFile.Seek(0, 0)
	hasher := md5.New()
	io.Copy(hasher, finalFile)
	actualMD5 := hex.EncodeToString(hasher.Sum(nil))
	if actualMD5 != task.FileMD5 {
		os.Remove(finalPath)
		utils.Error(c, http.StatusBadRequest, "文件完整性验证失败，MD5不匹配")
		return
	}

	// 生成缩略图
	isImage := services.IsImageFile(task.FileName)
	var thumbnailPath string
	var width, height int

	if isImage {
		w, h, err := services.GetImageDimensions(finalPath)
		if err == nil {
			width, height = w, h
		}
		thumbName := fileUUID + "_thumb.jpg"
		thumbRelDir := filepath.Join("thumbnails", fmt.Sprintf("%d", userID), now.Format("2006"), now.Format("01"))
		thumbAbsPath := filepath.Join(config.AppConfig.Storage.BasePath, thumbRelDir, thumbName)
		if err := services.GenerateThumbnail(finalPath, thumbAbsPath); err == nil {
			thumbnailPath = filepath.Join(thumbRelDir, thumbName)
		}
	}

	ext := filepath.Ext(task.FileName)
	mimeType := getMimeType(ext)

	// 创建物理文件对象
	fileObj := models.FileObject{
		FilePath:      filepath.Join(relDir, storageName),
		ThumbnailPath: thumbnailPath,
		FileSize:      task.FileSize,
		MimeType:      mimeType,
		IsImage:       isImage,
		Width:         width,
		Height:        height,
		FileMD5:       task.FileMD5,
		RefCount:      1,
	}
	database.DB.Create(&fileObj)

	// 创建逻辑文件记录
	fileRecord := models.File{
		Name:         storageName,
		OriginalName: task.FileName,
		FolderID:     task.FolderID,
		UserID:       userID,
		FileObjectID: fileObj.ID,
	}
	database.DB.Create(&fileRecord)

	// 更新用户存储使用量
	var user models.User
	database.DB.First(&user, userID)
	database.DB.Model(&user).Update("storage_used", user.StorageUsed+task.FileSize)

	// 清理临时文件和 Redis
	os.RemoveAll(task.TempDir)
	database.RedisClient.Del(ctx, chunkKey)
	database.DB.Model(&task).Update("status", "completed")

	utils.Success(c, fileRecord)
}

// DownloadFile 下载文件（支持 Range 断点续传）
func DownloadFile(c *gin.Context) {
	userID := c.GetUint("user_id")
	fileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的文件ID")
		return
	}

	var file models.File
	if err := database.DB.Preload("FileObject").Where("id = ? AND user_id = ?", fileID, userID).First(&file).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "文件不存在")
		return
	}

	absPath := filepath.Join(config.AppConfig.Storage.BasePath, file.FileObject.FilePath)
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		utils.Error(c, http.StatusNotFound, "文件不存在于存储中")
		return
	}

	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.OriginalName))
	http.ServeFile(c.Writer, c.Request, absPath)
}

// DownloadFileHead 获取文件元信息
func DownloadFileHead(c *gin.Context) {
	userID := c.GetUint("user_id")
	fileID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var file models.File
	if err := database.DB.Preload("FileObject").Where("id = ? AND user_id = ?", fileID, userID).First(&file).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Length", fmt.Sprintf("%d", file.FileObject.FileSize))
	c.Header("Content-Type", file.FileObject.MimeType)
	c.Status(http.StatusOK)
}

// PreviewFile 预览原图
func PreviewFile(c *gin.Context) {
	userID := c.GetUint("user_id")
	fileID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var file models.File
	if err := database.DB.Preload("FileObject").Where("id = ? AND user_id = ?", fileID, userID).First(&file).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "文件不存在")
		return
	}

	absPath := filepath.Join(config.AppConfig.Storage.BasePath, file.FileObject.FilePath)
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		utils.Error(c, http.StatusNotFound, "文件不存在于存储中")
		return
	}

	c.Header("Content-Type", file.FileObject.MimeType)
	c.File(absPath)
}

// GetThumbnail 获取缩略图
func GetThumbnail(c *gin.Context) {
	userID := c.GetUint("user_id")
	fileID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var file models.File
	if err := database.DB.Preload("FileObject").Where("id = ? AND user_id = ?", fileID, userID).First(&file).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "文件不存在")
		return
	}

	if file.FileObject.ThumbnailPath == "" {
		utils.Error(c, http.StatusNotFound, "缩略图不存在")
		return
	}

	absPath := filepath.Join(config.AppConfig.Storage.BasePath, file.FileObject.ThumbnailPath)
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		utils.Error(c, http.StatusNotFound, "缩略图文件不存在")
		return
	}

	c.Header("Content-Type", "image/jpeg")
	c.Header("Cache-Control", "public, max-age=86400")
	c.File(absPath)
}

// DeleteFile 软删除文件（移至回收站）
func DeleteFile(c *gin.Context) {
	userID := c.GetUint("user_id")
	fileID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var file models.File
	if err := database.DB.Preload("FileObject").Where("id = ? AND user_id = ?", fileID, userID).First(&file).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "文件不存在")
		return
	}

	if config.AppConfig.RecycleBin.Enabled {
		metadata, _ := json.Marshal(gin.H{
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
		database.DB.Create(&item)
	}

	database.DB.Delete(&file)
	utils.SuccessWithMessage(c, "文件已删除", nil)
}

// RenameFile 重命名文件
func RenameFile(c *gin.Context) {
	userID := c.GetUint("user_id")
	fileID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var req struct {
		Name string `json:"name" binding:"required,max=255"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	var file models.File
	if err := database.DB.Where("id = ? AND user_id = ?", fileID, userID).First(&file).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "文件不存在")
		return
	}

	database.DB.Model(&file).Update("original_name", req.Name)
	file.OriginalName = req.Name
	utils.Success(c, file)
}

// MoveFile 移动文件到其他文件夹
func MoveFile(c *gin.Context) {
	userID := c.GetUint("user_id")
	fileID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var req struct {
		FolderID uint `json:"folder_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	var file models.File
	if err := database.DB.Where("id = ? AND user_id = ?", fileID, userID).First(&file).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "文件不存在")
		return
	}

	// 验证目标文件夹存在
	if req.FolderID > 0 {
		var folder models.Folder
		if err := database.DB.Where("id = ? AND user_id = ?", req.FolderID, userID).First(&folder).Error; err != nil {
			utils.Error(c, http.StatusNotFound, "目标文件夹不存在")
			return
		}
	}

	database.DB.Model(&file).Update("folder_id", req.FolderID)
	utils.SuccessWithMessage(c, "文件已移动", file)
}

// BatchDeleteFiles 批量删除文件
func BatchDeleteFiles(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req struct {
		FileIDs []uint `json:"file_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	for _, fileID := range req.FileIDs {
		var file models.File
		if err := database.DB.Preload("FileObject").Where("id = ? AND user_id = ?", fileID, userID).First(&file).Error; err == nil {
			if config.AppConfig.RecycleBin.Enabled {
				metadata, _ := json.Marshal(gin.H{
					"mime_type": file.FileObject.MimeType, "is_image": file.FileObject.IsImage,
					"file_md5": file.FileObject.FileMD5, "file_object_id": file.FileObjectID,
				})
				fileSize := file.FileObject.FileSize
				item := models.RecycleBinItem{
					UserID: userID, OriginalID: file.ID, OriginalType: "file",
					OriginalName: file.OriginalName, OriginalPath: file.FileObject.FilePath,
					OriginalFolderID: &file.FolderID, FileObjectID: &file.FileObjectID,
					FileSize:  &fileSize,
					ExpiresAt: time.Now().AddDate(0, 0, config.AppConfig.RecycleBin.RetentionDays),
					Metadata:  string(metadata),
				}
				database.DB.Create(&item)
			}
			database.DB.Delete(&file)
		}
	}

	utils.SuccessWithMessage(c, "批量删除成功", nil)
}

// BatchMoveFiles 批量移动文件
func BatchMoveFiles(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req struct {
		FileIDs  []uint `json:"file_ids" binding:"required"`
		FolderID uint   `json:"folder_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	if req.FolderID > 0 {
		var folder models.Folder
		if err := database.DB.Where("id = ? AND user_id = ?", req.FolderID, userID).First(&folder).Error; err != nil {
			utils.Error(c, http.StatusNotFound, "目标文件夹不存在")
			return
		}
	}

	database.DB.Model(&models.File{}).
		Where("id IN ? AND user_id = ?", req.FileIDs, userID).
		Update("folder_id", req.FolderID)

	utils.SuccessWithMessage(c, "批量移动成功", nil)
}
