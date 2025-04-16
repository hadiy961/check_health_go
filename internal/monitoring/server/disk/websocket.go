package disk

import (
	"CheckHealthDO/internal/pkg/logger"
	"CheckHealthDO/internal/websocket"

	"github.com/gin-gonic/gin"
)

// WebSocketHandler creates a handler function for disk info WebSocket
func (m *Monitor) WebSocketHandler(c *gin.Context) {
	// Ensure monitor is properly initialized
	if m == nil {
		logger.Error("Disk monitor is nil in WebSocketHandler")
		c.String(500, "Internal server error: disk monitor not initialized")
		return
	}

	// Initialize the WebSocket registry if needed
	registry := websocket.GetRegistry()
	handler := registry.GetDiskHandler()

	// Create a new handler if one doesn't exist
	if handler == nil {
		handler = websocket.NewHandler()
		registry.RegisterDiskHandler(handler)
	}

	// Force an immediate status check to get fresh data
	m.checkStorageInfo()

	// Let the central registry handle the WebSocket connection
	handler.ServeHTTP(c.Writer, c.Request)

	logger.Info("New WebSocket client connected for Disk monitoring",
		logger.String("client_ip", c.ClientIP()))
}
