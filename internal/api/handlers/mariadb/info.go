package mariadb

import (
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"CheckHealthDO/internal/services/mariadb"
	"net/http"

	"github.com/gin-gonic/gin"
)

// InfoHandler handles MariaDB information requests
type InfoHandler struct {
	config *config.Config
}

// NewInfoHandler creates a new MariaDB info handler
func NewInfoHandler(cfg *config.Config) *InfoHandler {
	return &InfoHandler{
		config: cfg,
	}
}

// GetInfo handles the MariaDB information endpoint
func (h *Handler) GetInfo(c *gin.Context) {
	h.info.GetInfo(c)
}

// GetInfo handles the MariaDB information endpoint
func (h *InfoHandler) GetInfo(c *gin.Context) {
	info, err := mariadb.GetMariaDBInfo(h.config)
	if err != nil {
		logger.Error("API error: failed to get MariaDB info",
			logger.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to retrieve MariaDB information",
			"error":   err.Error(),
			"info":    nil,
		})
		return
	}

	// Provide a consistent response format regardless of status
	var message string
	if info.Status == "running" {
		message = "MariaDB service is running"
	} else {
		message = "MariaDB service is currently stopped"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  info.Status,
		"message": message,
		"info":    info,
	})
}
