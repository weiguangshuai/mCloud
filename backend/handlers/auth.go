package handlers

import (
	"net/http"

	"mcloud/services"
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
		utils.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	result, err := getServices().Auth.Register(c.Request.Context(), services.RegisterInput{
		Username: req.Username,
		Password: req.Password,
		Nickname: req.Nickname,
	})
	if respondServiceError(c, err) {
		return
	}

	utils.Success(c, gin.H{
		"token": result.Token,
		"user":  result.User,
	})
}

func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	result, err := getServices().Auth.Login(c.Request.Context(), services.LoginInput{
		Username: req.Username,
		Password: req.Password,
	})
	if respondServiceError(c, err) {
		return
	}

	utils.Success(c, gin.H{
		"token": result.Token,
		"user":  result.User,
	})
}

func GetProfile(c *gin.Context) {
	userID := c.GetUint("user_id")
	profile, err := getServices().Auth.GetProfile(c.Request.Context(), userID)
	if respondServiceError(c, err) {
		return
	}
	utils.Success(c, profile)
}
