package handlers

import (
	"mcloud/utils"

	"github.com/gin-gonic/gin"
)

func GetStorageQuota(c *gin.Context) {
	userID := c.GetUint("user_id")
	quota, err := getServices().User.GetStorageQuota(c.Request.Context(), userID)
	if respondServiceError(c, err) {
		return
	}
	utils.Success(c, quota)
}
