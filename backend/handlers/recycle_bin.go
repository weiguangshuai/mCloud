package handlers

import (
	"net/http"
	"strconv"

	"mcloud/utils"

	"github.com/gin-gonic/gin"
)

func ListRecycleBin(c *gin.Context) {
	userID := c.GetUint("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := getServices().RecycleBin.ListRecycleBin(c.Request.Context(), userID, page, pageSize)
	if respondServiceError(c, err) {
		return
	}

	utils.Success(c, result)
}

func RestoreItem(c *gin.Context) {
	userID := c.GetUint("user_id")
	itemID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid recycle-bin item id")
		return
	}

	if err := getServices().RecycleBin.RestoreItem(c.Request.Context(), userID, uint(itemID)); respondServiceError(c, err) {
		return
	}

	utils.SuccessWithMessage(c, "restored", nil)
}

func PermanentDelete(c *gin.Context) {
	userID := c.GetUint("user_id")
	itemID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid recycle-bin item id")
		return
	}

	if err := getServices().RecycleBin.PermanentDelete(c.Request.Context(), userID, uint(itemID)); respondServiceError(c, err) {
		return
	}

	utils.SuccessWithMessage(c, "permanently deleted", nil)
}

func EmptyRecycleBin(c *gin.Context) {
	userID := c.GetUint("user_id")
	if err := getServices().RecycleBin.EmptyRecycleBin(c.Request.Context(), userID); respondServiceError(c, err) {
		return
	}
	utils.SuccessWithMessage(c, "recycle bin emptied", nil)
}
