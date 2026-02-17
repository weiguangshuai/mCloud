package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"mcloud/logger"
	"mcloud/services"
	"mcloud/utils"

	"github.com/gin-gonic/gin"
)

func ListFiles(c *gin.Context) {
	userID := c.GetUint("user_id")
	folderID, err := strconv.ParseUint(c.DefaultQuery("folder_id", "0"), 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid folder_id")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	sortBy := c.DefaultQuery("sort_by", "created_at")
	order := c.DefaultQuery("order", "desc")

	result, err := getServices().File.ListFiles(c.Request.Context(), userID, uint(folderID), page, pageSize, sortBy, order)
	if respondServiceError(c, err) {
		return
	}

	utils.Success(c, result)
}

func UploadFile(c *gin.Context) {
	userID := c.GetUint("user_id")

	folderIDStr := c.PostForm("folder_id")
	folderID := uint64(0)
	if folderIDStr != "" {
		parsed, err := strconv.ParseUint(folderIDStr, 10, 32)
		if err != nil {
			utils.Error(c, http.StatusBadRequest, "invalid folder_id")
			return
		}
		folderID = parsed
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "failed to read upload file")
		return
	}
	defer file.Close()

	record, err := getServices().File.UploadFile(c.Request.Context(), userID, uint(folderID), file, header)
	if respondServiceError(c, err) {
		return
	}

	utils.Success(c, record)
}

func InitChunkedUpload(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req struct {
		FileName string `json:"file_name" binding:"required"`
		FileSize int64  `json:"file_size" binding:"required"`
		FileMD5  string `json:"file_md5" binding:"required"`
		FolderID uint   `json:"folder_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	result, err := getServices().File.InitChunkedUpload(c.Request.Context(), userID, services.InitChunkedUploadInput{
		FileName: req.FileName,
		FileSize: req.FileSize,
		FileMD5:  req.FileMD5,
		FolderID: req.FolderID,
	})
	if respondServiceError(c, err) {
		logger.Debugf("[upload] init failed user=%d file=%q size=%d err=%v", userID, req.FileName, req.FileSize, err)
		return
	}

	if result.Status == "instant_upload" {
		logger.Debugf("[upload] instant success user=%d file=%q size=%d file_id=%d", userID, req.FileName, req.FileSize, result.FileID)
		utils.SuccessWithMessage(c, "instant upload success", gin.H{"status": result.Status, "file_id": result.FileID})
		return
	}
	logger.Debugf("[upload] init success user=%d upload_id=%s file=%q size=%d chunks=%d chunk_size=%d", userID, result.UploadID, req.FileName, req.FileSize, result.TotalChunks, result.ChunkSize)

	utils.Success(c, gin.H{
		"upload_id":    result.UploadID,
		"chunk_size":   result.ChunkSize,
		"total_chunks": result.TotalChunks,
	})
}

func UploadChunk(c *gin.Context) {
	userID := c.GetUint("user_id")
	start := time.Now()
	uploadID := c.PostForm("upload_id")
	chunkIndex, err := strconv.Atoi(c.PostForm("chunk_index"))
	if err != nil {
		logger.Debugf("[upload] chunk invalid index user=%d upload_id=%s value=%q", userID, uploadID, c.PostForm("chunk_index"))
		utils.Error(c, http.StatusBadRequest, "invalid chunk_index")
		return
	}

	chunk, header, err := c.Request.FormFile("chunk")
	if err != nil {
		logger.Debugf("[upload] chunk read failed user=%d upload_id=%s chunk=%d err=%v", userID, uploadID, chunkIndex, err)
		utils.Error(c, http.StatusBadRequest, "failed to read chunk")
		return
	}
	defer chunk.Close()

	result, err := getServices().File.UploadChunk(c.Request.Context(), userID, uploadID, chunkIndex, chunk)
	if respondServiceError(c, err) {
		logger.Debugf("[upload] chunk save failed user=%d upload_id=%s chunk=%d size=%d cost=%s err=%v", userID, uploadID, chunkIndex, header.Size, time.Since(start), err)
		return
	}
	logger.Debugf("[upload] chunk saved user=%d upload_id=%s chunk=%d size=%d uploaded=%d/%d cost=%s", userID, uploadID, chunkIndex, header.Size, result.UploadedChunks, result.TotalChunks, time.Since(start))
	utils.Success(c, result)
}

func GetUploadStatus(c *gin.Context) {
	uploadID := c.Param("upload_id")
	userID := c.GetUint("user_id")

	result, err := getServices().File.GetUploadStatus(c.Request.Context(), userID, uploadID)
	if respondServiceError(c, err) {
		return
	}
	utils.Success(c, result)
}

func CompleteUpload(c *gin.Context) {
	userID := c.GetUint("user_id")
	start := time.Now()

	var req struct {
		UploadID string `json:"upload_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request")
		return
	}

	record, err := getServices().File.CompleteUpload(c.Request.Context(), userID, req.UploadID)
	if respondServiceError(c, err) {
		logger.Debugf("[upload] complete failed user=%d upload_id=%s cost=%s err=%v", userID, req.UploadID, time.Since(start), err)
		return
	}
	logger.Debugf("[upload] complete success user=%d upload_id=%s file_id=%d cost=%s", userID, req.UploadID, record.ID, time.Since(start))

	utils.Success(c, record)
}

func DownloadFile(c *gin.Context) {
	userID := c.GetUint("user_id")
	fileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid file id")
		return
	}

	info, err := getServices().File.GetDownloadInfo(c.Request.Context(), userID, uint(fileID))
	if respondServiceError(c, err) {
		return
	}

	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, info.DownloadName))
	http.ServeFile(c.Writer, c.Request, info.AbsPath)
}

func DownloadFileHead(c *gin.Context) {
	userID := c.GetUint("user_id")
	fileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	info, err := getServices().File.GetDownloadInfo(c.Request.Context(), userID, uint(fileID))
	if respondServiceError(c, err) {
		return
	}

	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Length", fmt.Sprintf("%d", info.File.FileObject.FileSize))
	c.Header("Content-Type", info.ContentType)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, info.DownloadName))
	c.Status(http.StatusOK)
}

func PreviewFile(c *gin.Context) {
	userID := c.GetUint("user_id")
	fileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid file id")
		return
	}

	info, err := getServices().File.GetPreviewInfo(c.Request.Context(), userID, uint(fileID))
	if respondServiceError(c, err) {
		return
	}

	if info.ContentType != "" {
		c.Header("Content-Type", info.ContentType)
	}
	c.File(info.AbsPath)
}

func GetThumbnail(c *gin.Context) {
	userID := c.GetUint("user_id")
	fileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid file id")
		return
	}

	info, err := getServices().File.GetThumbnailInfo(c.Request.Context(), userID, uint(fileID))
	if respondServiceError(c, err) {
		return
	}

	c.Header("Content-Type", info.ContentType)
	c.Header("Cache-Control", "public, max-age=86400")
	c.File(info.AbsPath)
}

func DeleteFile(c *gin.Context) {
	userID := c.GetUint("user_id")
	fileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid file id")
		return
	}

	if err := getServices().File.DeleteFile(c.Request.Context(), userID, uint(fileID)); respondServiceError(c, err) {
		return
	}

	utils.SuccessWithMessage(c, "file deleted", nil)
}

func RenameFile(c *gin.Context) {
	userID := c.GetUint("user_id")
	fileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid file id")
		return
	}

	var req struct {
		Name string `json:"name" binding:"required,max=255"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	file, err := getServices().File.RenameFile(c.Request.Context(), userID, uint(fileID), req.Name)
	if respondServiceError(c, err) {
		return
	}
	utils.Success(c, file)
}

func MoveFile(c *gin.Context) {
	userID := c.GetUint("user_id")
	fileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid file id")
		return
	}

	var req struct {
		FolderID uint `json:"folder_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request")
		return
	}

	if err := getServices().File.MoveFile(c.Request.Context(), userID, uint(fileID), req.FolderID); respondServiceError(c, err) {
		return
	}
	utils.SuccessWithMessage(c, "file moved", nil)
}

func BatchDeleteFiles(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req struct {
		FileIDs []uint `json:"file_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request")
		return
	}

	if err := getServices().File.BatchDeleteFiles(c.Request.Context(), userID, req.FileIDs); respondServiceError(c, err) {
		return
	}
	utils.SuccessWithMessage(c, "batch delete success", nil)
}

func BatchMoveFiles(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req struct {
		FileIDs  []uint `json:"file_ids" binding:"required"`
		FolderID uint   `json:"folder_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request")
		return
	}

	if err := getServices().File.BatchMoveFiles(c.Request.Context(), userID, req.FileIDs, req.FolderID); respondServiceError(c, err) {
		return
	}
	utils.SuccessWithMessage(c, "batch move success", nil)
}

func BatchGetThumbnails(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req struct {
		FileIDs []uint `json:"file_ids" binding:"required,min=1,max=200"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request")
		return
	}

	result, err := getServices().File.BatchGetThumbnails(c.Request.Context(), userID, req.FileIDs)
	if respondServiceError(c, err) {
		return
	}
	utils.Success(c, result)
}
