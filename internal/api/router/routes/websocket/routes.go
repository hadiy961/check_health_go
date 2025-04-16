package websocket

import (
	"CheckHealthDO/internal/monitoring/services/mariadb"
	"CheckHealthDO/internal/monitoring/server/cpu"
	"CheckHealthDO/internal/monitoring/server/disk"
	"CheckHealthDO/internal/monitoring/server/memory"
	"CheckHealthDO/internal/monitoring/server/sysinfo"

	"github.com/gin-gonic/gin"
)

// RegisterWebSocketRoutes registers the websocket routes
func RegisterWebSocketRoutes(router *gin.Engine, cpuMonitor *cpu.Monitor, mariaDBMonitor *mariadb.Monitor, memoryMonitor *memory.Monitor, sysInfoMonitor *sysinfo.Monitor, diskMonitor *disk.Monitor) {
	// CPU-specific websocket endpoint
	router.GET("/ws/cpu", func(c *gin.Context) {
		// Use the CPU monitor's WebSocketHandler directly
		cpuMonitor.WebSocketHandler(c)
	})

	// Memory-specific websocket endpoint
	router.GET("/ws/memory", func(c *gin.Context) {
		// Use the Memory monitor's WebSocketHandler directly
		memoryMonitor.WebSocketHandler(c)
	})

	// MariaDB-specific websocket endpoint
	router.GET("/ws/mariadb", func(c *gin.Context) {
		// Use the MariaDB monitor's WebSocketHandler directly
		mariaDBMonitor.WebSocketHandler(c)
	})

	// SysInfo-specific websocket endpoint
	router.GET("/ws/sysinfo", func(c *gin.Context) {
		// Use the SysInfo monitor's WebSocketHandler directly
		sysInfoMonitor.WebSocketHandler(c)
	})

	// DiskInfo-specific websocket endpoint
	router.GET("/ws/disk", func(c *gin.Context) {
		// Use the SysInfo monitor's WebSocketHandler directly
		diskMonitor.WebSocketHandler(c)
	})
}
