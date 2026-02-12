package handlers

import (
	"mcloud/services"
	"mcloud/utils"

	"github.com/gin-gonic/gin"
)

var appServices *services.Container

func SetServices(container *services.Container) {
	appServices = container
}

func getServices() *services.Container {
	if appServices == nil {
		panic("services container is not initialized")
	}
	return appServices
}

func respondServiceError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if appErr, ok := err.(*services.AppError); ok {
		if appErr.Data != nil {
			utils.ErrorWithData(c, appErr.HTTPCode, appErr.Message, appErr.Data)
		} else {
			utils.Error(c, appErr.HTTPCode, appErr.Message)
		}
		return true
	}
	utils.Error(c, 500, "internal error")
	return true
}
