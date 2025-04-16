package alerts

import (
	"CheckHealthDO/internal/pkg/logger"
	"fmt"
	"time"
)

// Handler handles generic alerts with throttling and notification logic
type Handler struct {
	config                  ConfigProvider
	alertStyles             map[AlertType]AlertStyle
	SuppressedWarningCount  int // Exported field so it can be accessed from other packages
	SuppressedCriticalCount int // Exported field so it can be accessed from other packages
	suppressLogFrequency    int
}

// NewHandler creates a new alert handler
func NewHandler(config ConfigProvider, styles map[AlertType]AlertStyle) *Handler {
	if styles == nil {
		styles = DefaultStyles()
	}

	return &Handler{
		config:                  config,
		alertStyles:             styles,
		suppressLogFrequency:    60, // Only log suppression once per ~60 checks (~1 minute if check interval is 1 second)
		SuppressedWarningCount:  0,
		SuppressedCriticalCount: 0,
	}
}

// ShouldThrottleAlert determines if alert notifications should be throttled
func (h *Handler) ShouldThrottleAlert(statusChanged bool, counter *int, alertType AlertType) bool {
	// Default throttling values
	throttlingEnabled := false
	cooldownPeriod := 300 // Default 5 minutes

	// Try to extract throttling configuration using more specialized interface check
	// This approach avoids the type assertion error
	if cfgProvider, ok := h.config.(interface {
		IsThrottlingEnabled() bool
		GetThrottlingCooldownPeriod() int
	}); ok {
		throttlingEnabled = cfgProvider.IsThrottlingEnabled()
		cooldownPeriod = cfgProvider.GetThrottlingCooldownPeriod()
	}

	// Never throttle status change alerts
	if statusChanged || !throttlingEnabled {
		*counter = 0
		return false
	}

	cooldownDuration := time.Duration(cooldownPeriod) * time.Second
	if time.Since(h.config.GetLastAlertTime()) < cooldownDuration {
		// Increment the counter and only log periodically
		*counter++
		if *counter%h.suppressLogFrequency == 1 { // Log on 1, 61, 121, etc.
			logger.Debug(fmt.Sprintf("Suppressing %s notifications due to cooldown period", alertType),
				logger.Int("suppressed_count", *counter))
			return true
		}
		return true // Throttle but don't log
	}

	// Reset counter when we're out of cooldown
	*counter = 0
	return false
}

// GetAlertStyle returns the style for a specific alert type
func (h *Handler) GetAlertStyle(alertType AlertType) AlertStyle {
	if style, ok := h.alertStyles[alertType]; ok {
		return style
	}
	// Return warning style as default if the requested type doesn't exist
	return h.alertStyles[AlertTypeWarning]
}

// SendNotifications sends alerts through configured channels
func (h *Handler) SendNotifications(title, message, level string) {
	// This is a general implementation that should be adapted based on your NotificationManager
	emailManager := h.config.GetNotificationManagers()

	// Send Email notification
	if err := emailManager.SendEmail(title, message); err != nil {
		logger.Error("Failed to send Email notification",
			logger.String("error", err.Error()))
	}
}
