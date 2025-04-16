package mariadb

import "github.com/gin-gonic/gin"

// InfoHandlerInterface defines the methods for MariaDB info handlers
type InfoHandlerInterface interface {
	GetInfo(c *gin.Context)
}

// ServiceHandlerInterface defines the methods for MariaDB service handlers
type ServiceHandlerInterface interface {
	StartService(c *gin.Context)
	StopService(c *gin.Context)
	RestartService(c *gin.Context)
}

// StatusHandlerInterface defines the methods for MariaDB status handlers
type StatusHandlerInterface interface {
	GetStatusDetails(c *gin.Context)
}

// HandlerInterface defines the methods for the main MariaDB handler
type HandlerInterface interface {
	GetInfo(c *gin.Context)
	StartService(c *gin.Context)
	StopService(c *gin.Context)
	RestartService(c *gin.Context)
	GetStatusDetails(c *gin.Context)
}
