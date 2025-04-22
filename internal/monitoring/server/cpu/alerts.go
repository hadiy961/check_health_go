package cpu

import (
	"CheckHealthDO/internal/alerts"
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"fmt"
	"time"
)

// AlertHandler handles CPU alerts
type AlertHandler struct {
	monitor               *Monitor
	handler               *alerts.Handler
	lastWarningAlertTime  time.Time
	lastCriticalAlertTime time.Time
	lastNormalAlertTime   time.Time
}

// NewAlertHandler creates a new alert handler
func NewAlertHandler(monitor *Monitor) *AlertHandler {
	return &AlertHandler{
		monitor: monitor,
		handler: alerts.NewHandler(monitor, nil), // Use default styles
	}
}

// HandleWarningAlert handles warning level CPU alerts
func (a *AlertHandler) HandleWarningAlert(info *CPUInfo, statusChanged bool) {
	var counter *int = &a.handler.SuppressedWarningCount

	// Get config with proper type assertion to determine cooldown
	configInterface := a.monitor.GetConfig()
	cfg, ok := configInterface.(*config.Config)

	// Default cooldown of 5 minutes if can't get config
	cooldownPeriod := 300
	if ok && cfg.Notifications.Throttling.Enabled {
		cooldownPeriod = cfg.Notifications.Throttling.CooldownPeriod
	}

	// Log the attempted alert regardless of whether it's throttled
	if statusChanged {
		logger.Info("CPU entered warning state",
			logger.Float64("usage_percent", info.Usage),
			logger.String("status", info.CPUStatus),
			logger.String("timestamp", time.Now().Format(time.RFC3339)),
			logger.Bool("notification_will_be_sent", time.Since(a.lastWarningAlertTime) >= time.Duration(cooldownPeriod)*time.Second))
	}

	// Apply throttling even for status changes
	if !a.lastWarningAlertTime.IsZero() {
		sinceLastWarning := time.Since(a.lastWarningAlertTime)
		if sinceLastWarning < time.Duration(cooldownPeriod)*time.Second {
			logger.Debug("Suppressing CPU warning notification due to cooldown",
				logger.Int("seconds_since_last", int(sinceLastWarning.Seconds())),
				logger.Int("cooldown_period", cooldownPeriod))
			*counter++
			return
		}
	}

	// Get server information using the common utility function
	serverInfo := alerts.GetServerInfoForAlert()

	// Create table content for CPU info
	tableContent := a.createCPUTableContent(info)

	// Additional content for warning
	additionalContent := `<p><b>Recommendation:</b> Please monitor the system closely if this condition persists.</p>`

	// Get style for this alert type
	style := a.handler.GetAlertStyle(alerts.AlertTypeWarning)

	// Generate HTML
	message := alerts.CreateAlertHTML(
		alerts.AlertTypeWarning,
		style,
		"CPU WARNING ALERT",
		statusChanged,
		tableContent,
		serverInfo,
		additionalContent,
	)

	// Send notification
	a.handler.SendNotifications("CPU Warning", message, "warning")
	a.monitor.UpdateLastAlertTime()
	a.lastWarningAlertTime = time.Now()
}

// HandleCriticalAlert handles critical level CPU alerts
func (a *AlertHandler) HandleCriticalAlert(info *CPUInfo, statusChanged bool) {
	var counter *int = &a.handler.SuppressedCriticalCount

	// Get config with proper type assertion to determine cooldown
	configInterface := a.monitor.GetConfig()
	cfg, ok := configInterface.(*config.Config)

	// Default cooldown of 5 minutes if can't get config
	cooldownPeriod := 300
	if ok && cfg.Notifications.Throttling.Enabled {
		cooldownPeriod = cfg.Notifications.Throttling.CooldownPeriod
	}

	// Log the attempted alert regardless of whether it's throttled
	if statusChanged {
		logger.Info("CPU entered critical state",
			logger.Float64("usage_percent", info.Usage),
			logger.String("status", info.CPUStatus),
			logger.String("timestamp", time.Now().Format(time.RFC3339)),
			logger.Bool("notification_will_be_sent", time.Since(a.lastCriticalAlertTime) >= time.Duration(cooldownPeriod)*time.Second))
	}

	// Apply throttling even for status changes
	if !a.lastCriticalAlertTime.IsZero() {
		sinceLastCritical := time.Since(a.lastCriticalAlertTime)
		if sinceLastCritical < time.Duration(cooldownPeriod)*time.Second {
			logger.Debug("Suppressing CPU critical notification due to cooldown",
				logger.Int("seconds_since_last", int(sinceLastCritical.Seconds())),
				logger.Int("cooldown_period", cooldownPeriod))
			*counter++
			return
		}
	}

	// Get server information using the common utility function
	serverInfo := alerts.GetServerInfoForAlert()

	// Create table content for CPU info
	tableContent := a.createCPUTableContent(info)

	// Additional content for critical
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
		"CRITICAL CPU ALERT",
		statusChanged,
		tableContent,
		serverInfo,
		additionalContent,
	)

	// Send notification
	a.handler.SendNotifications("CRITICAL CPU Alert", message, "critical")
	a.monitor.UpdateLastAlertTime()
	a.lastCriticalAlertTime = time.Now()
}

// HandleNormalAlert handles notifications when CPU returns to normal state
func (a *AlertHandler) HandleNormalAlert(info *CPUInfo, statusChanged bool) {
	// Only send notification if the status has changed
	if !statusChanged {
		return
	}

	// Get config with proper type assertion to determine cooldown
	configInterface := a.monitor.GetConfig()
	cfg, ok := configInterface.(*config.Config)

	// Default cooldown of 5 minutes if can't get config
	cooldownPeriod := 300
	if ok && cfg.Notifications.Throttling.Enabled {
		cooldownPeriod = cfg.Notifications.Throttling.CooldownPeriod
	}

	// Log the return to normal regardless of whether notification is throttled
	logger.Info("CPU returned to normal state",
		logger.Float64("usage_percent", info.Usage),
		logger.String("status", info.CPUStatus),
		logger.String("timestamp", time.Now().Format(time.RFC3339)),
		logger.Bool("notification_will_be_sent", time.Since(a.lastNormalAlertTime) >= time.Duration(cooldownPeriod)*time.Second))

	// Apply throttling even for normal status
	if !a.lastNormalAlertTime.IsZero() {
		sinceLastNormal := time.Since(a.lastNormalAlertTime)
		if sinceLastNormal < time.Duration(cooldownPeriod)*time.Second {
			logger.Debug("Suppressing CPU normal notification due to cooldown",
				logger.Int("seconds_since_last", int(sinceLastNormal.Seconds())),
				logger.Int("cooldown_period", cooldownPeriod))
			return
		}
	}

	// Get server information using the common utility function
	serverInfo := alerts.GetServerInfoForAlert()

	// Create table content for CPU info
	tableContent := a.createCPUTableContent(info)

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
		"CPU STATUS NORMALIZED",
		statusChanged,
		tableContent,
		serverInfo,
		additionalContent,
	)

	// Send notification
	a.handler.SendNotifications("CPU Status Normalized", message, "info")
	a.monitor.UpdateLastAlertTime()
	a.lastNormalAlertTime = time.Now()
}

// Helper method to create CPU-specific table content
func (a *AlertHandler) createCPUTableContent(info *CPUInfo) string {
	// Get style for alert
	style := a.handler.GetAlertStyle(alerts.AlertTypeWarning)

	// Create status line
	statusLine := alerts.CreateStatusLine(
		style.StatusColorClass,
		style.StatusText,
	)

	// Create table rows
	tableRows := []alerts.TableRow{
		{Label: "Usage Percentage", Value: fmt.Sprintf("%.2f%%", info.Usage)},
		{Label: "Model", Value: info.ModelName},
		{Label: "Cores", Value: fmt.Sprintf("%d", info.Cores)},
		{Label: "Threads", Value: fmt.Sprintf("%d", info.Threads)},
	}

	// Add processor count if available
	if info.ProcessorCount > 0 {
		tableRows = append(tableRows, alerts.TableRow{
			Label: "Physical CPUs",
			Value: fmt.Sprintf("%d", info.ProcessorCount),
		})
	}

	// Add temperature if available
	if info.Temperature > 0 {
		tableRows = append(tableRows, alerts.TableRow{
			Label: "Temperature",
			Value: fmt.Sprintf("%.1f°C", info.Temperature),
		})
	}

	// Create the table HTML
	tableHTML := alerts.CreateTable(tableRows)

	// Return the complete content
	return statusLine + tableHTML
}
