package auth

import (
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AuthRegistrar registers authentication routes
type AuthRegistrar struct{}

// Register implements the RouteRegistrar interface
func (r *AuthRegistrar) Register(engine *gin.Engine, config *config.Config) error {
	// Create auth group for API authentication endpoints
	authGroup := engine.Group("/api/auth")
	{
		// Token endpoint
		authGroup.POST("/token", func(c *gin.Context) {
			var credentials struct {
				Username string `json:"username"`
				Password string `json:"password"`
			}

			if err := c.BindJSON(&credentials); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
				return
			}

			if credentials.Username == config.Agent.Auth.User && credentials.Password == config.Agent.Auth.Pass {
				c.JSON(http.StatusOK, gin.H{
					"status": "success",
					"token":  "sample-token", // Replace with actual token generation
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
