package handlers

import (
	"net/http"

	"mcloud/config"
	"mcloud/database"
	"mcloud/models"
	"mcloud/utils"

	"github.com/gin-gonic/gin"
)

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6"`
	Nickname string `json:"nickname"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	// 检查用户名是否已存在
	var count int64
	database.DB.Model(&models.User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		utils.Error(c, http.StatusBadRequest, "用户名已存在")
		return
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "密码加密失败")
		return
	}

	user := models.User{
		Username:     req.Username,
		Password:     hashedPassword,
		Nickname:     req.Nickname,
		StorageQuota: config.AppConfig.Storage.DefaultUserQuota,
	}

	if err := database.DB.Create(&user).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "创建用户失败")
		return
	}

	utils.Success(c, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"nickname": user.Nickname,
	})
}

func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	var user models.User
	if err := database.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		utils.Error(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	if !utils.CheckPassword(req.Password, user.Password) {
		utils.Error(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	token, err := utils.GenerateToken(user.ID)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "生成令牌失败")
		return
	}

	utils.Success(c, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"nickname": user.Nickname,
		},
	})
}

func GetProfile(c *gin.Context) {
	userID := c.GetUint("user_id")

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "用户不存在")
		return
	}

	utils.Success(c, gin.H{
		"id":            user.ID,
		"username":      user.Username,
		"nickname":      user.Nickname,
		"avatar":        user.Avatar,
		"storage_quota": user.StorageQuota,
		"storage_used":  user.StorageUsed,
		"created_at":    user.CreatedAt,
	})
}
