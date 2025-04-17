package memory

import (
	"CheckHealthDO/internal/alerts"
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"CheckHealthDO/internal/services/mariadb"
	"fmt"
	"time"
)

// AlertHandler handles memory alerts
type AlertHandler struct {
	monitor              *Monitor
	handler              *alerts.Handler
	lastWarningAlertTime time.Time
	warningCount         int           // Track consecutive warnings
	warningEscalation    int           // Number of warnings before escalating
	pendingWarnings      []MemoryInfo  // For collecting multiple warnings
	aggregationInterval  time.Duration // How long to collect alerts before sending
	lastAggregationTime  time.Time     // When we last sent an aggregated alert
}

// NewAlertHandler creates a new alert handler
func NewAlertHandler(monitor *Monitor) *AlertHandler {
	return &AlertHandler{
		monitor:              monitor,
		handler:              alerts.NewHandler(monitor, nil), // Use default styles
		lastWarningAlertTime: time.Time{},                     // Initialize to zero time
		warningCount:         0,
		warningEscalation:    5, // Only notify after 5 consecutive warnings
		pendingWarnings:      make([]MemoryInfo, 0),
		aggregationInterval:  5 * time.Minute,
		lastAggregationTime:  time.Now(),
	}
}

// HandleWarningAlert handles warning level memory alerts
func (a *AlertHandler) HandleWarningAlert(info *MemoryInfo, statusChanged bool) {
	var counter *int = &a.handler.SuppressedWarningCount

	// Get config with proper type assertion to determine cooldown
	configInterface := a.monitor.GetConfig()
	cfg, ok := configInterface.(*config.Config)

	// Default cooldown of 5 minutes if can't get config
	cooldownPeriod := 300
	if ok && cfg.Notifications.Throttling.Enabled {
		cooldownPeriod = cfg.Notifications.Throttling.CooldownPeriod
	}

	// Apply additional time-based throttling to warnings specifically
	// This ensures we respect the cooldown even if the handler's throttling has issues
	if !statusChanged && !a.lastWarningAlertTime.IsZero() {
		sinceLastWarning := time.Since(a.lastWarningAlertTime)
		if sinceLastWarning < time.Duration(cooldownPeriod)*time.Second {
			logger.Debug("Suppressing memory warning due to local cooldown",
				logger.Int("seconds_since_last", int(sinceLastWarning.Seconds())),
				logger.Int("cooldown_period", cooldownPeriod))
			*counter++
			return
		}
	}

	// If status changed, send immediately
	if statusChanged {
		// Reset counter on status change
		a.warningCount = 0

		// Send normal notification for status changes
		a.sendWarningNotification(info, statusChanged, "")
		return
	}

	// For non-status change warnings, use escalation
	a.warningCount++

	// Collect for aggregation
	a.pendingWarnings = append(a.pendingWarnings, *info)

	// Only send aggregated alert if enough time has passed
	if time.Since(a.lastAggregationTime) >= a.aggregationInterval && len(a.pendingWarnings) > 0 {
		// Create an aggregated message
		a.sendAggregatedWarningAlert()
		return
	}

	// Only send notification if we hit escalation threshold
	if a.warningCount < a.warningEscalation {
		logger.Debug("Memory warning suppressed due to escalation policy",
			logger.Int("warning_count", a.warningCount),
			logger.Int("escalation_threshold", a.warningEscalation))
		return
	}

	// Add escalation information to the alert
	escalationNote := fmt.Sprintf(`
	<p><b>Note:</b> This alert was sent after %d consecutive warnings.</p>`,
		a.warningCount)

	// Send the alert with escalation information
	a.sendWarningNotification(info, statusChanged, escalationNote)
}

// sendWarningNotification sends a warning notification for memory issues
func (a *AlertHandler) sendWarningNotification(info *MemoryInfo, statusChanged bool, additionalNote string) {
	// For warning alert, check if we should throttle using the handler's method
	var counter *int = &a.handler.SuppressedWarningCount
	if a.handler.ShouldThrottleAlert(statusChanged, counter, alerts.AlertTypeWarning) {
		return
	}

	// Record the time we're sending this warning
	a.lastWarningAlertTime = time.Now()

	// Get server information using the common utility function
	serverInfo := alerts.GetServerInfoForAlert()

	// Create table content for memory info
	tableContent := a.createMemoryTableContent(info)

	// Base content for warning
	additionalContent := `<p><b>Recommendation:</b> Please monitor the system closely if this condition persists.</p>`

	// Add any additional note if provided
	if additionalNote != "" {
		additionalContent += additionalNote
	}

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
	// Reset escalation counter when returning to normal
	a.warningCount = 0

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

// sendAggregatedWarningAlert sends a single warning alert that summarizes multiple warnings
func (a *AlertHandler) sendAggregatedWarningAlert() {
	if len(a.pendingWarnings) == 0 {
		return
	}

	// Find highest memory usage from collected warnings
	highestUsage := float64(0)
	var worstMemoryInfo *MemoryInfo

	for i, info := range a.pendingWarnings {
		if info.UsedMemoryPercentage > highestUsage {
			highestUsage = info.UsedMemoryPercentage
			worstMemoryInfo = &a.pendingWarnings[i]
		}
	}

	// Create a summary message
	additionalContent := fmt.Sprintf(`
    <p><b>Aggregated Warning:</b> %d memory warnings detected in the last %d minutes.</p>
    <p>Highest memory usage was %.2f%%</p>
    <p><b>Recommendation:</b> Please monitor the system closely if this condition persists.</p>`,
		len(a.pendingWarnings), int(a.aggregationInterval.Minutes()), highestUsage)

	// Get server information using the common utility function
	serverInfo := alerts.GetServerInfoForAlert()

	// Create table content for memory info
	tableContent := a.createMemoryTableContent(worstMemoryInfo)

	// Get style for this alert type
	style := a.handler.GetAlertStyle(alerts.AlertTypeWarning)

	// Generate HTML
	message := alerts.CreateAlertHTML(
		alerts.AlertTypeWarning,
		style,
		"AGGREGATED MEMORY WARNING ALERT",
		false,
		tableContent,
		serverInfo,
		additionalContent,
	)

	// Send notification
	a.handler.SendNotifications("Memory Warning Summary", message, "warning")
	a.monitor.UpdateLastAlertTime()

	// Update tracking state
	a.lastAggregationTime = time.Now()
	a.pendingWarnings = make([]MemoryInfo, 0) // Clear pending warnings
}
