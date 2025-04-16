package server

import (
	"CheckHealthDO/internal/api/handlers"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all server monitoring routes
func RegisterRoutes(engine *gin.Engine, serverHandler *handlers.ServerHandler) {
	serverGroup := engine.Group("/api/server")
	{
		// General server information
		serverGroup.GET("/info", serverHandler.GetServerInfo)

		// Specific monitoring endpoints
		serverGroup.GET("/cpu", serverHandler.GetCPUInfo)
		serverGroup.GET("/memory", serverHandler.GetMemoryInfo)
		serverGroup.GET("/disk", serverHandler.GetDiskInfo)
		serverGroup.GET("/sysinfo", serverHandler.GetSystemInfoHandler)
	}
}
