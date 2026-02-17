package middleware

import (
	"time"

	"mcloud/logger"

	"github.com/gin-gonic/gin"
)

// RequestLogger writes per-request logs at debug level.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !logger.IsDebugEnabled() {
			c.Next()
			return
		}

		start := time.Now()
		path := c.Request.URL.Path
		rawQuery := c.Request.URL.RawQuery

		c.Next()

		if rawQuery != "" {
			path = path + "?" + rawQuery
		}

		logger.Debugf(
			"%s | %d | %s | %s | %s",
			c.Request.Method,
			c.Writer.Status(),
			time.Since(start),
			c.ClientIP(),
			path,
		)
	}
}
