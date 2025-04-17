package middleware

import (
	"CheckHealthDO/internal/pkg/jwt"
	"CheckHealthDO/internal/pkg/logger"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// JWTAuthMiddleware creates a middleware to validate JWT tokens
func JWTAuthMiddleware(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Paths that don't require auth
		excludedPaths := []string{
			"/api/auth/login",
			"/", // Root health check endpoint
		}

		// Check if the current path is excluded from auth
		currentPath := c.Request.URL.Path
		for _, path := range excludedPaths {
			if currentPath == path {
				c.Next()
				return
			}
		}

		// For WebSocket connections, verify token from query param or header
		if c.Request.Header.Get("Upgrade") == "websocket" {
			// Try to get token from query param first (useful for WebSocket connections)
			token := c.Query("token")

			// If not in query, check header as fallback
			if token == "" {
				authHeader := c.GetHeader("Authorization")
				if authHeader == "" {
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
					return
				}

				// Extract bearer token
				parts := strings.Split(authHeader, " ")
				if len(parts) != 2 || parts[0] != "Bearer" {
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format"})
					return
				}
				token = parts[1]
			}

			// Validate token
			claims, err := jwt.ValidateToken(token, jwtSecret)
			if err != nil {
				logger.Warn("Invalid JWT token for WebSocket",
					logger.String("error", err.Error()),
					logger.String("path", currentPath))
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
				return
			}

			// Store username in context for future use
			c.Set("username", claims.Username)
			c.Next()
			return
		}

		// Regular HTTP request auth flow
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			return
		}

		// Extract bearer token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format"})
			return
		}

		// Validate token
		tokenString := parts[1]
		claims, err := jwt.ValidateToken(tokenString, jwtSecret)
		if err != nil {
			logger.Warn("Invalid JWT token", logger.String("error", err.Error()))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		// Store username in context for future use
		c.Set("username", claims.Username)
		c.Next()
	}
}
