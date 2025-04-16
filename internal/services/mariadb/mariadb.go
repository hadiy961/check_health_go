package mariadb

import (
	"fmt"

	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
)

// GetMariaDBInfo returns comprehensive information about the MariaDB service
func GetMariaDBInfo(cfg *config.Config) (*MariaDBInfo, error) {
	serviceName := cfg.Monitoring.MariaDB.ServiceName

	// Check if service is running with the improved check
	isRunning, err := CheckServiceStatus(serviceName, cfg)
	if err != nil {
		logger.Error("Failed to check MariaDB status",
			logger.String("service", serviceName),
			logger.String("error", err.Error()))
		return nil, fmt.Errorf("failed to check MariaDB status: %w", err)
	}

	info := &MariaDBInfo{
		Status: "stopped",
	}

	if isRunning {
		info.Status = "running"

		// Get database config from application config
		dbConfig := GetDBConfigFromConfig(cfg)

		// Get uptime
		uptime, err := GetUptime(dbConfig)
		if err != nil {
			logger.Error("Failed to get MariaDB uptime",
				logger.String("error", err.Error()))
			return nil, fmt.Errorf("failed to get MariaDB uptime: %w", err)
		}
		info.UptimeSeconds = uptime
		info.Uptime = FormatUptime(uptime) // Using FormatUptime from info.go

		// Get version
		version, err := GetVersion(dbConfig)
		if err != nil {
			logger.Error("Failed to get MariaDB version",
				logger.String("error", err.Error()))
			return nil, fmt.Errorf("failed to get MariaDB version: %w", err)
		}
		info.Version = version

		// Get active connections
		connections, err := GetActiveConnections(dbConfig)
		if err != nil {
			logger.Error("Failed to get MariaDB connections",
				logger.String("error", err.Error()))
			return nil, fmt.Errorf("failed to get MariaDB connections: %w", err)
		}
		info.ConnectionsActive = connections

		// Get memory usage
		memUsed, memPercent, err := GetMariaDBMemoryUsage()
		if err != nil {
			logger.Warn("Failed to get MariaDB memory usage",
				logger.String("error", err.Error()))
			// Don't return an error, just continue without memory info
		} else {
			info.MemoryUsed = memUsed
			info.MemoryUsedHuman = FormatBytes(memUsed)
			info.MemoryUsedPercent = memPercent
		}
	}

	return info, nil
}
