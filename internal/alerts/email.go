package alerts

import (
	"CheckHealthDO/internal/notifications"
	"CheckHealthDO/internal/pkg/config"
)

// NewEmailNotifier creates a new email notifier that implements NotificationManager
func NewEmailNotifier(cfg *config.Config) NotificationManager {
	return notifications.NewEmailManager(cfg)
}
