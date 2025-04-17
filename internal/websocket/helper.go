package websocket

import (
	"CheckHealthDO/internal/pkg/logger"
	"net/http"
)

// GetUsernameFromContext retrieves username from request context
// This is set by the JWT middleware after successful authentication
func GetUsernameFromContext(r *http.Request) string {
	// The username will be set in the context by the middleware
	if username, ok := r.Context().Value("username").(string); ok {
		return username
	}
	return ""
}

// LogWebSocketConnection logs a new WebSocket connection with authentication info
func LogWebSocketConnection(clientIP, endpoint, username string) {
	if username != "" {
		logger.Info("New authenticated WebSocket client connected",
			logger.String("endpoint", endpoint),
			logger.String("client_ip", clientIP),
			logger.String("username", username))
	} else {
		logger.Warn("WebSocket client connected without authentication",
			logger.String("endpoint", endpoint),
			logger.String("client_ip", clientIP))
	}
}
