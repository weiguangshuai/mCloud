package handlers

import (
	"net/http"

	"mcloud/database"
	"mcloud/models"
	"mcloud/utils"

	"github.com/gin-gonic/gin"
)

// GetStorageQuota 获取存储配额和使用情况
func GetStorageQuota(c *gin.Context) {
	userID := c.GetUint("user_id")

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "用户不存在")
		return
	}

	utils.Success(c, gin.H{
		"storage_quota":   user.StorageQuota,
		"storage_used":    user.StorageUsed,
		"available_space": user.StorageQuota - user.StorageUsed,
		"usage_percent":   float64(user.StorageUsed) / float64(user.StorageQuota) * 100,
	})
}
