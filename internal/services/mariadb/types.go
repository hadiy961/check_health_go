package mariadb

// MariaDBInfo holds information about the MariaDB service
type MariaDBInfo struct {
	Status            string  `json:"status"`              // Running or stopped
	Uptime            string  `json:"uptime"`              // Human-readable uptime
	UptimeSeconds     int64   `json:"uptime_seconds"`      // Uptime in seconds
	Version           string  `json:"version"`             // MariaDB version
	ConnectionsActive int     `json:"connections_active"`  // Current active connections
	MemoryUsed        uint64  `json:"memory_used"`         // Memory used by MariaDB in bytes
	MemoryUsedHuman   string  `json:"memory_used_human"`   // Human-readable memory usage
	MemoryUsedPercent float64 `json:"memory_used_percent"` // Percentage of system memory used
}

// DBConfig holds connection details for MariaDB
type DBConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	Database string
}
