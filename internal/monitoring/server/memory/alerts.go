package memory

import (
	"CheckHealthDO/internal/alerts"
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"CheckHealthDO/internal/services/mariadb"
	"fmt"
)

// AlertHandler handles memory alerts
type AlertHandler struct {
	monitor *Monitor
	handler *alerts.Handler
}

// NewAlertHandler creates a new alert handler
func NewAlertHandler(monitor *Monitor) *AlertHandler {
	return &AlertHandler{
		monitor: monitor,
		handler: alerts.NewHandler(monitor, nil), // Use default styles
	}
}

// HandleWarningAlert handles warning level memory alerts
func (a *AlertHandler) HandleWarningAlert(info *MemoryInfo, statusChanged bool) {
	var counter *int = &a.handler.SuppressedWarningCount

	// For warning alert, check if we should throttle
	if a.handler.ShouldThrottleAlert(statusChanged, counter, alerts.AlertTypeWarning) {
		return
	}

	// Get server information using the common utility function
	serverInfo := alerts.GetServerInfoForAlert()

	// Create table content for memory info
	tableContent := a.createMemoryTableContent(info)

	// Additional content for warning
	additionalContent := `<p><b>Recommendation:</b> Please monitor the system closely if this condition persists.</p>`

	// Get style for this alert type
	style := a.handler.GetAlertStyle(alerts.AlertTypeWarning)

	// Generate HTML
	message := alerts.CreateAlertHTML(
		alerts.AlertTypeWarning,
		style,
		"MEMORY WARNING ALERT",
		statusChanged,
		tableContent,
		serverInfo,
		additionalContent,
	)

	// Send notification
	a.handler.SendNotifications("Memory Warning", message, "warning")
	a.monitor.UpdateLastAlertTime()
}

// HandleCriticalAlert handles critical level memory alerts
func (a *AlertHandler) HandleCriticalAlert(info *MemoryInfo, statusChanged bool) {
	var counter *int = &a.handler.SuppressedCriticalCount

	// For critical alert, check if we should throttle
	if a.handler.ShouldThrottleAlert(statusChanged, counter, alerts.AlertTypeCritical) {
		return
	}

	// Get server information using the common utility function
	serverInfo := alerts.GetServerInfoForAlert()

	// Create table content for memory info
	tableContent := a.createMemoryTableContent(info)

	// Get config with proper type assertion
	configInterface := a.monitor.GetConfig()
	cfg, ok := configInterface.(*config.Config)
	if !ok {
		logger.Error("Failed to convert config to *config.Config in HandleCriticalAlert")
		// Use a default notification without custom recovery actions
		additionalContent := `
		<div style="background-color: #d9534f; color: white; padding: 10px; text-align: center; margin: 20px 0;">
			<h3>IMMEDIATE ACTION REQUIRED!</h3>
		</div>`

		// Get style for this alert type
		style := a.handler.GetAlertStyle(alerts.AlertTypeCritical)

		// Generate HTML
		message := alerts.CreateAlertHTML(
			alerts.AlertTypeCritical,
			style,
			"CRITICAL MEMORY ALERT",
			statusChanged,
			tableContent,
			serverInfo,
			additionalContent,
		)

		// Send notification
		a.handler.SendNotifications("CRITICAL Memory Alert", message, "critical")
		a.monitor.UpdateLastAlertTime()
		return
	}

	// Prepare additional content for critical alerts
	additionalContent := `
	<div style="background-color: #d9534f; color: white; padding: 10px; text-align: center; margin: 20px 0;">
		<h3>IMMEDIATE ACTION REQUIRED!</h3>
	</div>`

	// Add recovery action message if MariaDB auto-restart is enabled
	if cfg.Monitoring.MariaDB.Enabled && cfg.Monitoring.MariaDB.RestartOnThreshold.Enabled {
		additionalContent += `
		<div style="background-color: #5bc0de; padding: 10px; margin: 20px 0; border-radius: 5px;">
			<h3 style="color: white; background-color: #31b0d5; padding: 5px; margin-top: 0;">AUTOMATIC RECOVERY ACTION</h3>
			<p>The system is attempting to automatically restart the MariaDB service 
			to free up memory resources and maintain stability.</p>
		</div>`
	}

	// Get style for this alert type
	style := a.handler.GetAlertStyle(alerts.AlertTypeCritical)

	// Generate HTML
	message := alerts.CreateAlertHTML(
		alerts.AlertTypeCritical,
		style,
		"CRITICAL MEMORY ALERT",
		statusChanged,
		tableContent,
		serverInfo,
		additionalContent,
	)

	// Send notification
	a.handler.SendNotifications("CRITICAL Memory Alert", message, "critical")
	a.monitor.UpdateLastAlertTime()

	// Perform recovery actions if configured to do so
	if cfg.Monitoring.MariaDB.RestartOnThreshold.Enabled {
		a.performRecoveryActions(info)
	}
}

// HandleNormalAlert handles notifications when memory returns to normal state
func (a *AlertHandler) HandleNormalAlert(info *MemoryInfo, statusChanged bool) {
	// Only send notification if the status has changed
	if !statusChanged {
		return
	}

	// Get server information using the common utility function
	serverInfo := alerts.GetServerInfoForAlert()

	// Create table content for memory info
	tableContent := a.createMemoryTableContent(info)

	// Additional content for normal
	additionalContent := `
	<div style="background-color: #dff0d8; color: #3c763d; padding: 10px; margin: 20px 0; text-align: center; border-radius: 5px;">
		<p>System is now operating within normal parameters.</p>
	</div>`

	// Get style for this alert type
	style := a.handler.GetAlertStyle(alerts.AlertTypeNormal)

	// Generate HTML
	message := alerts.CreateAlertHTML(
		alerts.AlertTypeNormal,
		style,
		"MEMORY STATUS NORMALIZED",
		statusChanged,
		tableContent,
		serverInfo,
		additionalContent,
	)

	// Send notification
	a.handler.SendNotifications("Memory Status Normalized", message, "info")
	a.monitor.UpdateLastAlertTime()
}

// Helper method to create memory-specific table content
func (a *AlertHandler) createMemoryTableContent(info *MemoryInfo) string {
	// Get style for alert
	style := a.handler.GetAlertStyle(alerts.AlertTypeWarning)

	// Create status line
	statusLine := alerts.CreateStatusLine(
		style.StatusColorClass,
		style.StatusText,
	)

	// Convert memory values to GB for readability
	usedMemoryGB := float64(info.UsedMemory) / 1024 / 1024 / 1024
	totalMemoryGB := float64(info.TotalMemory) / 1024 / 1024 / 1024
	freeMemoryGB := float64(info.FreeMemory) / 1024 / 1024 / 1024

	// Create table rows
	tableRows := []alerts.TableRow{
		{Label: "Usage Percentage", Value: fmt.Sprintf("%.2f%%", info.UsedMemoryPercentage)},
		{Label: "Used Memory", Value: fmt.Sprintf("%.2f GB (%.2f%%)", usedMemoryGB, info.UsedMemoryPercentage)},
		{Label: "Total Memory", Value: fmt.Sprintf("%.2f GB", totalMemoryGB)},
		{Label: "Free Memory", Value: fmt.Sprintf("%.2f GB (%.2f%%)", freeMemoryGB, info.FreeMemoryPercentage)},
	}

	// Add swap information if available
	if info.SwapTotal > 0 {
		swapUsedGB := float64(info.SwapUsed) / 1024 / 1024 / 1024
		swapTotalGB := float64(info.SwapTotal) / 1024 / 1024 / 1024
		swapPercentage := 0.0
		if info.SwapTotal > 0 {
			swapPercentage = float64(info.SwapUsed) * 100.0 / float64(info.SwapTotal)
		}

		tableRows = append(tableRows, alerts.TableRow{
			Label: "Swap Usage",
			Value: fmt.Sprintf("%.2f GB / %.2f GB (%.2f%%)", swapUsedGB, swapTotalGB, swapPercentage),
		})
	}

	// Create the table HTML
	tableHTML := alerts.CreateTable(tableRows)

	// Return the complete content
	return statusLine + tableHTML
}

// performRecoveryActions takes steps to reduce memory usage
func (a *AlertHandler) performRecoveryActions(info *MemoryInfo) {
	logger.Info("Performing memory recovery actions due to critical memory usage")
	logger.Info("This is a critical situation that requires immediate attention. The system is attempting automatic recovery.")

	// Get config with proper type assertion
	configInterface := a.monitor.GetConfig()
	cfg, ok := configInterface.(*config.Config)
	if !ok {
		logger.Error("Failed to convert config to *config.Config in performRecoveryActions")
		return
	}

	// Restart MariaDB if configured and running
	if cfg.Monitoring.MariaDB.Enabled &&
		cfg.Monitoring.MariaDB.RestartOnThreshold.Enabled {

		serviceName := cfg.Monitoring.MariaDB.ServiceName

		// Check if service is running first
		isRunning, _ := mariadb.CheckServiceStatus(serviceName, nil)
		if isRunning {
			logger.Info("Attempting to restart MariaDB service to free memory",
				logger.String("service", serviceName),
				logger.Float64("memory_usage", info.UsedMemoryPercentage))

			err := mariadb.RestartMariaDBService(serviceName)
			if err != nil {
				logger.Error("Failed to restart MariaDB service",
					logger.String("error", err.Error()))
			} else {
				logger.Info("Successfully restarted MariaDB service")
			}
		} else {
			logger.Warn("MariaDB service is not running, no restart performed",
				logger.String("service", serviceName))
		}

		// Log memory-intensive processes for additional context
		logger.Info("Consider checking for memory-intensive processes if issues persist")
	}
}
