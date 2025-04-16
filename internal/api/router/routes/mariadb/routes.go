package mariadb

import (
	"CheckHealthDO/internal/api/handlers/mariadb"
	monitorMariadb "CheckHealthDO/internal/monitoring/services/mariadb"
	"CheckHealthDO/internal/pkg/config"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all MariaDB-related routes
func RegisterRoutes(engine *gin.Engine, config *config.Config, monitor *monitorMariadb.Monitor) {
	// Create MariaDB handler
	handler := mariadb.NewHandler(config)
	handler.SetMonitor(monitor)

	mariadbGroup := engine.Group("/api/mariadb")
	RegisterRoutesWithGroup(mariadbGroup, handler)
}

// RegisterRoutesWithGroup registers routes with a pre-configured group
func RegisterRoutesWithGroup(group *gin.RouterGroup, handler *mariadb.Handler) {
	// Service management endpoints
	group.POST("/start", handler.StartService)
	group.POST("/stop", handler.StopService)
	group.POST("/restart", handler.RestartService)

	// Status and information endpoints
	group.GET("/status", handler.GetStatusDetails)
	group.GET("/info", handler.GetInfo)
}
