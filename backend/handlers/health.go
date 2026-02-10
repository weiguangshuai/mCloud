package handlers

import (
	"mcloud/utils"

	"github.com/gin-gonic/gin"
)

func HealthCheck(c *gin.Context) {
	utils.Success(c, gin.H{
		"status":  "ok",
		"service": "mcloud",
	})
}
