package handlers

import (
	"CheckHealthDO/internal/pkg/logger"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleError provides a consistent way to handle errors in route handlers
func HandleError(c *gin.Context, err error) {
	logger.Error("API error",
		logger.String("path", c.Request.URL.Path),
		logger.String("error", err.Error()))
	c.JSON(http.StatusInternalServerError, gin.H{
		"error": err.Error(),
	})
}
