package auth

import (
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/jwt"
	"CheckHealthDO/internal/pkg/logger"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// AuthRegistrar registers authentication routes
type AuthRegistrar struct{}

// Register implements the RouteRegistrar interface
func (r *AuthRegistrar) Register(engine *gin.Engine, config *config.Config) error {
	// Create auth group for API authentication endpoints
	authGroup := engine.Group("/api/auth")
	{
		// Login endpoint
		authGroup.POST("/login", func(c *gin.Context) {
			var credentials struct {
				Username string `json:"username"`
				Password string `json:"password"`
			}

			if err := c.BindJSON(&credentials); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
				return
			}

			if credentials.Username == config.Agent.Auth.User && credentials.Password == config.Agent.Auth.Pass {
				// Generate JWT token
				tokenExpiration := 24 * time.Hour
				if config.API.Auth.JWTExpiration > 0 {
					tokenExpiration = time.Duration(config.API.Auth.JWTExpiration) * time.Second
				}

				token, err := jwt.GenerateToken(credentials.Username, config.API.Auth.JWTSecret, tokenExpiration)
				if err != nil {
					logger.Error("Failed to generate token", logger.String("error", err.Error()))
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"status":     "success",
					"token":      token,
					"expires_in": tokenExpiration.Seconds(),
				})
			} else {
				logger.Warn("Failed authentication attempt",
					logger.String("username", credentials.Username),
					logger.String("ip", c.ClientIP()))

				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid credentials",
				})
			}
		})
	}

	return nil
}
