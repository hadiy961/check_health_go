package sysinfo

import (
	"CheckHealthDO/internal/pkg/logger"
	"CheckHealthDO/internal/websocket"

	"github.com/gin-gonic/gin"
)

// WebSocketHandler handles WebSocket connections for CPU monitoring
func (m *Monitor) WebSocketHandler(c *gin.Context) {
	// Initialize the WebSocket registry if needed
	registry := websocket.GetRegistry()
	handler := registry.GetSysHandler()

	// Create a new handler if one doesn't exist
	if handler == nil {
		handler = websocket.NewHandler()
		registry.RegisterSysHandler(handler)
	}

	// Force an immediate CPU check to get fresh data
	m.checkSysInfo()

	// Let the central registry handle the WebSocket connection
	handler.ServeHTTP(c.Writer, c.Request)

	logger.Info("New WebSocket client connected for CPU monitoring",
		logger.String("client_ip", c.ClientIP()))
}
