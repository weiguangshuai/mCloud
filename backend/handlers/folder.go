package handlers

import (
	"net/http"
	"strconv"

	"mcloud/utils"

	"github.com/gin-gonic/gin"
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

	var parentID *uint
	if parentIDStr, exists := c.GetQuery("parent_id"); exists {
		parsedParentID, err := strconv.ParseUint(parentIDStr, 10, 32)
		if err != nil {
			utils.Error(c, http.StatusBadRequest, "invalid parent_id")
			return
		}
		tmp := uint(parsedParentID)
		parentID = &tmp
	}

	folders, err := getServices().Folder.ListFolders(c.Request.Context(), userID, parentID)
	if respondServiceError(c, err) {
		return
	}

	utils.Success(c, folders)
}

func CreateFolder(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req CreateFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	folder, err := getServices().Folder.CreateFolder(c.Request.Context(), userID, req.Name, req.ParentID)
	if respondServiceError(c, err) {
		return
	}

	utils.Success(c, folder)
}

func RenameFolder(c *gin.Context) {
	userID := c.GetUint("user_id")
	folderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid folder id")
		return
	}

	var req RenameFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	folder, err := getServices().Folder.RenameFolder(c.Request.Context(), userID, uint(folderID), req.Name)
	if respondServiceError(c, err) {
		return
	}

	utils.Success(c, folder)
}

func DeleteFolder(c *gin.Context) {
	userID := c.GetUint("user_id")
	folderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid folder id")
		return
	}

	if err := getServices().Folder.DeleteFolder(c.Request.Context(), userID, uint(folderID)); respondServiceError(c, err) {
		return
	}

	utils.SuccessWithMessage(c, "folder deleted", nil)
}
