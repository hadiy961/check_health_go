package handlers

import (
	"CheckHealthDO/internal/api/handlers/mariadb"
	"CheckHealthDO/internal/pkg/config"

	"github.com/gin-gonic/gin"
)

// DatabaseHandler contains handlers for database-related endpoints
type DatabaseHandler struct {
	mariadbHandler *mariadb.Handler
}

// NewDatabaseHandler creates a new database handler
func NewDatabaseHandler(cfg *config.Config) *DatabaseHandler {
	return &DatabaseHandler{
		mariadbHandler: mariadb.NewHandler(cfg),
	}
}

// GetMariaDBInfo handles the MariaDB information endpoint
func (h *DatabaseHandler) GetMariaDBInfo(c *gin.Context) {
	h.mariadbHandler.GetInfo(c)
}

// StartMariaDBService handles starting the MariaDB service
func (h *DatabaseHandler) StartMariaDBService(c *gin.Context) {
	h.mariadbHandler.StartService(c)
}

// StopMariaDBService handles stopping the MariaDB service
func (h *DatabaseHandler) StopMariaDBService(c *gin.Context) {
	h.mariadbHandler.StopService(c)
}

// RestartMariaDBService handles restarting the MariaDB service
func (h *DatabaseHandler) RestartMariaDBService(c *gin.Context) {
	h.mariadbHandler.RestartService(c)
}

// GetMariaDBStatusDetails provides detailed status information with logs and diagnostics
func (h *DatabaseHandler) GetMariaDBStatusDetails(c *gin.Context) {
	h.mariadbHandler.GetStatusDetails(c)
}
