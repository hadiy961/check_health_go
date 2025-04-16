package mariadb

import (
	mariadbMonitor "CheckHealthDO/internal/monitoring/services/mariadb"
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"CheckHealthDO/internal/services/mariadb"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ServiceHandler handles MariaDB service operations
type ServiceHandler struct {
	config  *config.Config
	monitor *mariadbMonitor.Monitor
}

// NewServiceHandler creates a new MariaDB service handler
func NewServiceHandler(cfg *config.Config) *ServiceHandler {
	return &ServiceHandler{
		config: cfg,
	}
}

// SetMonitor sets the monitor reference
func (h *ServiceHandler) SetMonitor(monitor *mariadbMonitor.Monitor) {
	h.monitor = monitor
}

// StartService handles starting the MariaDB service
func (h *ServiceHandler) StartService(c *gin.Context) {
	serviceName := h.config.Monitoring.MariaDB.ServiceName

	// Check if the service is already running
	isRunning, err := mariadb.CheckServiceStatus(serviceName, nil)
	if err != nil {
		logger.Error("API error: failed to check MariaDB service status",
			logger.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to check MariaDB service status",
			"error":   err.Error(),
		})
		return
	}

	// If the service is already running, return a message
	if isRunning {
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "MariaDB service is already running",
		})
		return
	}

	// Mark this as an API-initiated action before attempting to start
	if h.monitor != nil {
		h.monitor.MarkAPIAction("start")
	}

	// Attempt to start the service
	err = mariadb.StartMariaDBService(serviceName)
	if err != nil {
		logger.Error("API error: failed to start MariaDB service",
			logger.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to start MariaDB service",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "MariaDB service started successfully",
	})
}

// StopService handles stopping the MariaDB service
func (h *ServiceHandler) StopService(c *gin.Context) {
	serviceName := h.config.Monitoring.MariaDB.ServiceName

	// Check if the service is already stopped
	isRunning, err := mariadb.CheckServiceStatus(serviceName, nil)
	if err != nil {
		logger.Error("API error: failed to check MariaDB service status",
			logger.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to check MariaDB service status",
			"error":   err.Error(),
		})
		return
	}

	// If the service is already stopped, return a message
	if !isRunning {
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "MariaDB service is already stopped",
		})
		return
	}

	// Mark this as an API-initiated action before attempting to stop
	if h.monitor != nil {
		h.monitor.MarkAPIAction("stop")
	}

	// Attempt to stop the service
	err = mariadb.StopMariaDBService(serviceName)
	if err != nil {
		logger.Error("API error: failed to stop MariaDB service",
			logger.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to stop MariaDB service",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "MariaDB service stopped successfully",
	})
}

// RestartService handles restarting the MariaDB service
func (h *ServiceHandler) RestartService(c *gin.Context) {
	serviceName := h.config.Monitoring.MariaDB.ServiceName

	// Check if the service is running
	isRunning, err := mariadb.CheckServiceStatus(serviceName, nil)
	if err != nil {
		logger.Error("API error: failed to check MariaDB service status",
			logger.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to check MariaDB service status",
			"error":   err.Error(),
		})
		return
	}

	// If the service is not running, start it first
	if !isRunning {
		logger.Info("MariaDB service is not running, starting it first before restart")

		err = mariadb.StartMariaDBService(serviceName)
		if err != nil {
			logger.Error("API error: failed to start MariaDB service before restart",
				logger.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to start MariaDB service before restart",
				"error":   err.Error(),
			})
			return
		}
	}

	// Mark this as an API-initiated restart action
	if h.monitor != nil {
		h.monitor.MarkAPIAction("restart")
	}

	// Now restart the service
	err = mariadb.RestartMariaDBService(serviceName)
	if err != nil {
		logger.Error("API error: failed to restart MariaDB service",
			logger.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to restart MariaDB service",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "MariaDB service restarted successfully",
	})
}
