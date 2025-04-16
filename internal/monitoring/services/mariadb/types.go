package mariadb

import "time"

// MariaDBMetricsMsg is the message structure for WebSocket updates
type MariaDBMetricsMsg struct {
	Timestamp      time.Time `json:"timestamp"`
	Status         *Status   `json:"status"`
	LastUpdateTime string    `json:"last_update_time"`
}
