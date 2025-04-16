package mariadb

import (
	"CheckHealthDO/internal/alerts"
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"fmt"
	"strings"
	"time"
)

// Notifier handles sending notifications for MariaDB status changes
type Notifier struct {
	config       *config.Config
	emailManager alerts.NotificationManager
	lastNotified map[string]time.Time // Track last notification time by type
}

// NewNotifier creates a new notifier
func NewNotifier(cfg *config.Config) *Notifier {
	return &Notifier{
		config:       cfg,
		emailManager: alerts.NewEmailNotifier(cfg),
		lastNotified: make(map[string]time.Time),
	}
}

// SendStatusChangeNotification sends notifications about MariaDB status changes
func (n *Notifier) SendStatusChangeNotification(status *Status, reason string) {
	// Check if throttling is enabled and if we should throttle this notification
	if n.config.Notifications.Throttling.Enabled {
		// Create notification key
		notifKey := fmt.Sprintf("mariadb-status-%s", status.Status)

		// Check if we've sent this type of notification recently
		if lastTime, exists := n.lastNotified[notifKey]; exists {
			cooldownPeriod := time.Duration(n.config.Notifications.Throttling.CooldownPeriod) * time.Second
			if time.Since(lastTime) < cooldownPeriod {
				logger.Info("Throttling MariaDB status notification",
					logger.String("status", status.Status),
					logger.String("reason", "cooldown period not elapsed"))
				return
			}
		}

		// Update last notification time
		n.lastNotified[notifKey] = time.Now()
	}

	// Get server information
	serverInfo := alerts.GetServerInfoForAlert()

	// Determine alert type and styling based on MariaDB status
	var alertType alerts.AlertType
	var subject string

	if status.Status == "stopped" {
		alertType = alerts.AlertTypeCritical
		subject = "CRITICAL: MariaDB Service Stopped"
	} else {
		alertType = alerts.AlertTypeNormal
		subject = "MariaDB Service Running"
	}

	// Create table content for MariaDB status info
	tableContent := n.createMariaDBStatusTable(status, reason)

	// Create additional content based on status
	var additionalContent string
	if status.Status == "stopped" && status.StopReason != "" {
		additionalContent = n.createErrorDetailsContent(status)
	} else if status.Status == "running" {
		additionalContent = `
		<div style="background-color: #dff0d8; color: #3c763d; padding: 10px; margin: 20px 0; text-align: center; border-radius: 5px;">
			<p>MariaDB service is running normally.</p>
		</div>`
	}

	// Get default styling
	styles := alerts.DefaultStyles()
	style := styles[alertType]

	// Generate HTML
	message := alerts.CreateAlertHTML(
		alertType,
		style,
		fmt.Sprintf("MariaDB Service Status: %s", strings.ToUpper(status.Status)),
		true, // Status changes are always important
		tableContent,
		serverInfo,
		additionalContent,
	)

	// Send email notification if enabled
	if n.config.Notifications.Email.Enabled {
		err := n.emailManager.SendEmail(subject, message)
		if err != nil {
			logger.Error("Failed to send email notification for MariaDB status change",
				logger.String("error", err.Error()))
		} else {
			logger.Info("Sent email notification for MariaDB status change",
				logger.String("status", status.Status))
		}
	}
}

// createMariaDBStatusTable creates a table with MariaDB status information
func (n *Notifier) createMariaDBStatusTable(status *Status, reason string) string {
	// Create status line
	statusClass := "normal-text"
	statusText := "RUNNING"
	if status.Status == "stopped" {
		statusClass = "critical-text"
		statusText = "STOPPED"
	}

	statusLine := alerts.CreateStatusLine(statusClass, statusText)

	// Create table rows
	tableRows := []alerts.TableRow{
		{Label: "Previous Status", Value: status.PreviousStatus},
		{Label: "Current Status", Value: status.Status},
		{Label: "Timestamp", Value: status.Timestamp.Format(time.RFC1123)},
		{Label: "Detected Reason", Value: reason},
	}

	// Create the table HTML
	tableHTML := alerts.CreateTable(tableRows)

	// Return the complete content
	return statusLine + tableHTML
}

// createErrorDetailsContent creates HTML content for error details
func (n *Notifier) createErrorDetailsContent(status *Status) string {
	errorDetails := formatErrorDetails(status.StopErrorDetails)

	return fmt.Sprintf(`
	<div style="background-color: #f5f5f5; padding: 10px; margin-top: 20px; border-left: 5px solid #d9534f;">
		<h3>Failure Details</h3>
		<p><strong>Failure Type:</strong> %s</p>
		<div style="background-color: #f8f8f8; padding: 10px; border-left: 4px solid #cc0000; 
			font-family: monospace; white-space: pre-wrap; margin: 10px 0;">%s</div>
	</div>`, status.StopReason, errorDetails)
}

// formatErrorDetails prepares error details for display in notifications
func formatErrorDetails(details string) string {
	if details == "" {
		return "No additional details available"
	}

	// Limit length and sanitize for HTML display
	if len(details) > 1000 {
		lines := strings.Split(details, "\n")
		truncated := []string{}
		totalLen := 0

		for _, line := range lines {
			if totalLen+len(line) > 1000 {
				break
			}
			truncated = append(truncated, line)
			totalLen += len(line) + 1 // +1 for newline
		}

		return strings.Join(truncated, "\n") + "\n... (truncated)"
	}

	return details
}
