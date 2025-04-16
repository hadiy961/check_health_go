package alerts

import "time"

// AlertType defines the type of alert (warning, critical, normal)
type AlertType string

// Alert type constants
const (
	AlertTypeWarning  AlertType = "warning"
	AlertTypeCritical AlertType = "critical"
	AlertTypeNormal   AlertType = "normal"
)

// AlertStyle holds styling information for different alert types
type AlertStyle struct {
	BorderColor      string
	HeaderColor      string
	StatusColorClass string
	StatusText       string
}

// DefaultStyles returns a map of default styles for each alert type
func DefaultStyles() map[AlertType]AlertStyle {
	return map[AlertType]AlertStyle{
		AlertTypeWarning: {
			BorderColor:      "#f0ad4e",
			HeaderColor:      "#f0ad4e",
			StatusColorClass: "warning-text",
			StatusText:       "WARNING",
		},
		AlertTypeCritical: {
			BorderColor:      "#d9534f",
			HeaderColor:      "#d9534f",
			StatusColorClass: "critical-text",
			StatusText:       "CRITICAL",
		},
		AlertTypeNormal: {
			BorderColor:      "#5cb85c",
			HeaderColor:      "#5cb85c",
			StatusColorClass: "normal-text",
			StatusText:       "NORMAL",
		},
	}
}

// Common HTML styles for all alert types
const CommonStyles = `
body { font-family: Arial, sans-serif; }
.container { padding: 20px; border-radius: 5px; }
.header { color: white; padding: 10px; text-align: center; border-radius: 5px; }
.content { margin: 20px 0; }
table { width: 100%%; border-collapse: collapse; }
table, th, td { border: 1px solid #ddd; }
th, td { padding: 8px; text-align: left; }
th { background-color: #f5f5f5; }
.note { background-color: #f5f5f5; padding: 10px; margin-top: 20px; border-left: 5px solid; }
.section { margin-top: 20px; }
.section-title { color: #333; border-bottom: 1px solid #ddd; padding-bottom: 5px; }
`

// AlertData is an interface for data that can be included in an alert
type AlertData interface {
	// Any type that can be used for alert data should implement AlertData
}

// NotificationManager is an interface for sending notifications
type NotificationManager interface {
	SendEmail(subject, body string) error
}

// ConfigProvider is an interface for accessing configuration
type ConfigProvider interface {
	GetNotificationManagers() NotificationManager
	GetConfig() interface{}
	GetLastAlertTime() time.Time
	UpdateLastAlertTime()
}
