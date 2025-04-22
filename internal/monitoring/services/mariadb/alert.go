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
}

// NewNotifier creates a new notifier
func NewNotifier(cfg *config.Config) *Notifier {
	return &Notifier{
		config:       cfg,
		emailManager: alerts.NewEmailNotifier(cfg),
	}
}

// SendStatusChangeNotification sends notifications about MariaDB status changes
func (n *Notifier) SendStatusChangeNotification(status *Status, reason string) {
	// Get server information
	serverInfo := alerts.GetServerInfoForAlert()

	// Determine alert type and styling based on MariaDB status
	var alertType alerts.AlertType
	var subject string

	if status.Status == "stopped" {
		// Customize based on stop reason for more specific alerts
		if strings.Contains(status.StopReason, "Manual Systemctl Stop") {
			alertType = alerts.AlertTypeWarning // It's not critical if manually stopped
			subject = "NOTICE: MariaDB Service Manually Stopped"
		} else if strings.Contains(status.StopReason, "Memory Critical Auto-Recovery") {
			alertType = alerts.AlertTypeWarning
			subject = "NOTICE: MariaDB Service Restarted Due to Memory Issues"
		} else if strings.Contains(status.StopReason, "Out of Memory") {
			alertType = alerts.AlertTypeCritical
			subject = "CRITICAL: MariaDB Service Killed by OOM"
		} else {
			alertType = alerts.AlertTypeCritical
			subject = "CRITICAL: MariaDB Service Unexpectedly Stopped"
		}
	} else { // running
		// Customize based on start reason
		if strings.Contains(reason, "manually started") {
			alertType = alerts.AlertTypeNormal
			subject = "INFO: MariaDB Service Manually Started"
		} else if strings.Contains(reason, "system startup") || strings.Contains(reason, "boot process") {
			alertType = alerts.AlertTypeNormal
			subject = "INFO: MariaDB Service Started During System Boot"
		} else if strings.Contains(reason, "memory-related shutdown") {
			alertType = alerts.AlertTypeWarning
			subject = "NOTICE: MariaDB Service Recovered After Memory Issues"
		} else {
			alertType = alerts.AlertTypeNormal
			subject = "INFO: MariaDB Service Running"
		}
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

	// Add common info first
	tableRows := []alerts.TableRow{
		{Label: "Status", Value: statusText},
		{Label: "Service Name", Value: status.ServiceName},
		{Label: "Timestamp", Value: status.Timestamp.Format(time.RFC3339)},
	}

	// Add reason for status change if available
	if reason != "" {
		// Format reason to highlight it better depending on the content
		reasonDisplay := reason

		// Add formatting based on reason content
		if strings.Contains(reason, "manually started") || strings.Contains(reason, "manually by user") {
			// Green for manual operations
			reasonDisplay = fmt.Sprintf("<span style='color: #3c763d; font-weight: bold;'>%s</span>", reason)
		} else if strings.Contains(reason, "system startup") || strings.Contains(reason, "boot") {
			// Blue for boot-related events
			reasonDisplay = fmt.Sprintf("<span style='color: #31708f; font-weight: bold;'>%s</span>", reason)
		} else if strings.Contains(reason, "auto-restart") || strings.Contains(reason, "recovery") {
			// Yellow for recovery events
			reasonDisplay = fmt.Sprintf("<span style='color: #8a6d3b; font-weight: bold;'>%s</span>", reason)
		} else if strings.Contains(reason, "manually stopped") {
			// Orange for manual stop events
			reasonDisplay = fmt.Sprintf("<span style='color: #f0ad4e; font-weight: bold;'>%s</span>", reason)
		} else if strings.Contains(reason, "memory") {
			// Red for memory-related events
			reasonDisplay = fmt.Sprintf("<span style='color: #d9534f; font-weight: bold;'>%s</span>", reason)
		}

		tableRows = append(tableRows, alerts.TableRow{
			Label: "Change Reason",
			Value: reasonDisplay,
		})
	}

	// Highlight stop reason if it's a memory-related stop
	if status.StopReason != "" {
		stopReasonDisplay := status.StopReason
		if strings.Contains(status.StopReason, "Memory Critical Auto-Recovery") {
			stopReasonDisplay = fmt.Sprintf("<span style='color: #d9534f; font-weight: bold;'>%s</span>", status.StopReason)
		} else if strings.Contains(status.StopReason, "Out of Memory") {
			stopReasonDisplay = fmt.Sprintf("<span style='color: #d9534f; font-weight: bold;'>%s</span>", status.StopReason)
		}

		tableRows = append(tableRows, alerts.TableRow{
			Label: "Stop Reason",
			Value: stopReasonDisplay,
		})
	}

	// Create the table HTML
	tableHTML := alerts.CreateTable(tableRows)

	// Return the complete content
	return statusLine + tableHTML
}

// createErrorDetailsContent creates HTML content for error details
func (n *Notifier) createErrorDetailsContent(status *Status) string {
	var content string

	// Special handling for manually stopped service
	if strings.Contains(status.StopReason, "Manual Systemctl Stop") {
		content = `
        <div style="background-color: #fcf8e3; border-left: 5px solid #f0ad4e; padding: 10px; margin: 10px 0;">
            <h3 style="color: #8a6d3b; margin-top: 0;">Manual Service Stop Detected</h3>
            <p>The MariaDB service was manually stopped using the systemctl command.</p>
            <p><strong>This is an expected event if you or another administrator initiated the stop.</strong></p>
        </div>
        `
	} else if strings.Contains(status.StopReason, "Memory Critical Auto-Recovery") {
		content = `
        <div style="background-color: #fcf8e3; border-left: 5px solid #f0ad4e; padding: 10px; margin: 10px 0;">
            <h3 style="color: #8a6d3b; margin-top: 0;">Automatic Memory Recovery Action</h3>
            <p>The CheckHealthDO system detected critical memory conditions and <strong>automatically restarted</strong> the MariaDB service to free up memory resources.</p>
            <p><strong>This is an AUTOMATIC action initiated by the memory monitor - not a manual service stop.</strong></p>
            <p>Recommendations:</p>
            <ul>
                <li>Check memory usage on the server with <code>free -m</code></li>
                <li>Review MariaDB configuration, especially memory-related settings like <code>innodb_buffer_pool_size</code></li>
                <li>Consider adding more RAM to the server if this happens frequently</li>
                <li>Optimize queries and database structure to reduce memory pressure</li>
            </ul>
        </div>
        `
	} else if strings.Contains(status.StopReason, "Out of Memory") {
		content = `
		<div style="background-color: #f2dede; border-left: 5px solid #d9534f; padding: 10px; margin: 10px 0;">
			<h3 style="color: #a94442; margin-top: 0;">MariaDB Was Killed By The Operating System</h3>
			<p>The system detected that MariaDB was terminated by the kernel's Out-of-Memory (OOM) killer due to severe memory pressure.</p>
			<p><strong>This is a critical issue that requires immediate attention!</strong></p>
			<p>Recommendations:</p>
			<ul>
				<li>Check memory usage on the server with <code>free -m</code></li>
				<li>Review MariaDB configuration, especially memory-related settings</li>
				<li>Consider adding more RAM to the server</li>
				<li>Check for other processes consuming excessive memory</li>
				<li>Review and optimize database queries</li>
			</ul>
		</div>
		 `
	} else if status.StopReason != "" {
		// Generic stop reason
		content = fmt.Sprintf(`
        <div style="background-color: #f2dede; border-left: 5px solid #d9534f; padding: 10px; margin: 10px 0;">
            <h3 style="color: #a94442; margin-top: 0;">MariaDB Service Stopped: %s</h3>
            <p>The MariaDB service has stopped unexpectedly. This may require investigation.</p>
            <p><strong>Recommended actions:</strong></p>
            <ul>
                <li>Check the MariaDB error logs for specific error messages</li>
                <li>Verify system resources (disk space, memory, etc.)</li>
                <li>Check for any recent system changes or updates</li>
            </ul>
        </div>
        `, status.StopReason)
	}

	// Add a section for service start information if the service just started
	if status.Status == "running" && status.PreviousStatus == "stopped" {
		content += `
        <div style="background-color: #dff0d8; border-left: 5px solid #3c763d; padding: 10px; margin: 10px 0;">
            <h3 style="color: #3c763d; margin-top: 0;">MariaDB Service Started</h3>
            <p>The MariaDB service has been successfully started.</p>
            <p><strong>Steps to ensure optimal performance:</strong></p>
            <ul>
                <li>Check available system memory with <code>free -m</code></li>
                <li>Monitor connections with <code>SHOW PROCESSLIST</code></li>
                <li>Verify logs for any warnings during startup</li>
            </ul>
        </div>`
	}

	// Add error details section if we have any
	if status.StopErrorDetails != "" {
		content += fmt.Sprintf(`
		<div style="background-color: #f5f5f5; border-left: 5px solid #777; padding: 10px; margin: 10px 0;">
			<h3 style="margin-top: 0;">Error Details</h3>
			<pre style="background-color: #eee; padding: 10px; border-radius: 4px; overflow-x: auto;">%s</pre>
		</div>
		`, formatErrorDetails(status.StopErrorDetails))
	}

	return content
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
