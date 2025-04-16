package memory

import (
	"CheckHealthDO/internal/pkg/logger"
	"CheckHealthDO/internal/websocket"

	"github.com/gin-gonic/gin"
)

// Add WebSocket handler for memory monitoring
func (m *Monitor) WebSocketHandler(c *gin.Context) {
	// Initialize the WebSocket registry if needed
	registry := websocket.GetRegistry()
	handler := registry.GetMemoryHandler()
	if handler == nil {
		handler = websocket.NewHandler()
		registry.RegisterMemoryHandler(handler)
	}

	// Force an immediate memory check to get fresh data
	m.checkMemory()

	// Let the central registry handle the WebSocket connection
	handler.ServeHTTP(c.Writer, c.Request)

	logger.Info("New WebSocket client connected for Memory monitoring",
		logger.String("client_ip", c.ClientIP()))
}
