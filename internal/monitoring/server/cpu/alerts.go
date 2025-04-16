package cpu

import (
	"CheckHealthDO/internal/alerts"
	"fmt"
)

// AlertHandler handles CPU alerts
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

// HandleWarningAlert handles warning level CPU alerts
func (a *AlertHandler) HandleWarningAlert(info *CPUInfo, statusChanged bool) {
	var counter *int = &a.handler.SuppressedWarningCount

	// For warning alert, check if we should throttle
	if a.handler.ShouldThrottleAlert(statusChanged, counter, alerts.AlertTypeWarning) {
		return
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
}

// HandleCriticalAlert handles critical level CPU alerts
func (a *AlertHandler) HandleCriticalAlert(info *CPUInfo, statusChanged bool) {
	var counter *int = &a.handler.SuppressedCriticalCount

	// For critical alert, check if we should throttle
	if a.handler.ShouldThrottleAlert(statusChanged, counter, alerts.AlertTypeCritical) {
		return
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
}

// HandleNormalAlert handles notifications when CPU returns to normal state
func (a *AlertHandler) HandleNormalAlert(info *CPUInfo, statusChanged bool) {
	// Only send notification if the status has changed
	if !statusChanged {
		return
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
