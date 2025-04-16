package mariadb

import (
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"CheckHealthDO/internal/services/mariadb"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// StatusHandler handles MariaDB status operations
type StatusHandler struct {
	config *config.Config
}

// NewStatusHandler creates a new MariaDB status handler
func NewStatusHandler(cfg *config.Config) *StatusHandler {
	return &StatusHandler{
		config: cfg,
	}
}

// GetStatusDetails handles the MariaDB status details endpoint
func (h *Handler) GetStatusDetails(c *gin.Context) {
	h.status.GetStatusDetails(c)
}

// GetStatusDetails provides detailed status information with logs and diagnostics
func (h *StatusHandler) GetStatusDetails(c *gin.Context) {
	serviceName := h.config.Monitoring.MariaDB.ServiceName
	logPath := h.config.Monitoring.MariaDB.LogPath

	// Check if the service is running with the improved check
	isRunning, err := mariadb.CheckServiceStatus(serviceName, h.config)
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

	// Create base response
	var status string
	if isRunning {
		status = "running"
	} else {
		status = "stopped"
	}

	response := gin.H{
		"status":       status,
		"service_name": serviceName,
		"timestamp":    time.Now().Format(time.RFC3339),
	}

	if isRunning {
		// Get additional runtime information for running service
		dbConfig := mariadb.GetDBConfigFromConfig(h.config)

		// Get uptime
		uptime, err := mariadb.GetUptime(dbConfig)
		if err == nil {
			response["uptime"] = mariadb.FormatUptime(uptime)
			response["uptime_seconds"] = uptime
		}

		// Get version
		version, err := mariadb.GetVersion(dbConfig)
		if err == nil {
			response["version"] = version
		}

		// Get active connections
		connections, err := mariadb.GetActiveConnections(dbConfig)
		if err == nil {
			response["connections_active"] = connections
		}

		// Get status message
		response["message"] = "MariaDB service is running normally"
		response["diagnosis"] = []string{
			"Service is responding to connection requests",
			"Database is operational",
		}
	} else {
		// For stopped service, get log information and diagnose possible issues
		errorLogs, logErr := mariadb.GetLatestMariaDBLogs(logPath, 10) // Get last 10 log entries

		// Create diagnostic information based on service being down
		possibleCauses := []string{
			"Service might have been stopped manually",
			"Service could have crashed due to resource constraints",
			"There might be configuration issues",
			"Database files might be corrupted",
		}

		// Provide more specific error message based on the error type
		var logMessage string
		logSamples := []string{}

		if logErr != nil {
			// Check what type of error occurred
			if os.IsNotExist(logErr) {
				logMessage = fmt.Sprintf("Log file not found at: %s", logPath)
			} else if os.IsPermission(logErr) {
				logMessage = fmt.Sprintf("Permission denied accessing log file: %s", logPath)
			} else {
				logMessage = fmt.Sprintf("Error reading log file: %s", logErr.Error())
			}
			logSamples = append(logSamples, logMessage)

			// If MariaDB logs aren't available, try to get systemd logs as fallback
			systemLogs, _ := mariadb.GetSystemdServiceLogs(serviceName, 5)
			if len(systemLogs) > 0 {
				logSamples = append(logSamples, "System logs:")
				logSamples = append(logSamples, systemLogs...)
			}
		} else if len(errorLogs) == 0 {
			logSamples = append(logSamples, "Log file exists but contains no relevant entries")
		} else {
			logSamples = errorLogs

			// Use the log analyzer from mariadb package
			additionalCauses := mariadb.AnalyzeMariaDBLogs(errorLogs)
			possibleCauses = append(possibleCauses, additionalCauses...)
		}

		response["message"] = "MariaDB service is currently stopped"
		response["logs"] = logSamples
		response["diagnosis"] = possibleCauses
		response["log_path"] = logPath // Add log path to help troubleshoot
		response["recommended_action"] = "Check the MariaDB logs for more details and try to start the service"
	}

	c.JSON(http.StatusOK, response)
}
