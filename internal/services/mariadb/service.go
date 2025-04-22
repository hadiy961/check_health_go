package mariadb

import (
	"database/sql"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
)

// CheckServiceStatus checks if MariaDB service is running
func CheckServiceStatus(serviceName string, cfg *config.Config) (bool, error) {
	// First check if we're on a systemd system
	_, err := exec.LookPath("systemctl")
	systemdAvailable := err == nil

	if systemdAvailable {
		// On systemd systems, trust systemctl status as the source of truth
		cmd := exec.Command("systemctl", "is-active", serviceName)
		output, _ := cmd.Output()

		// Check the actual output regardless of error (systemctl returns non-zero if service is not active)
		status := strings.TrimSpace(string(output))
		serviceRunning := (status == "active")

		if !serviceRunning {
			// logger.Debug("MariaDB service is not active according to systemctl",
			// 	logger.String("service", serviceName),
			// 	logger.String("status", status))
			return false, nil
		}
	} else {
		// Not a systemd system, fallback to other checks
		serviceRunning := false

		// Try the service command (for init.d systems)
		cmd := exec.Command("service", serviceName, "status")
		output, err := cmd.Output()
		if err == nil {
			serviceRunning = strings.Contains(string(output), "running")
		}

		if !serviceRunning {
			// Last resort, try checking if the process is running
			cmd = exec.Command("pgrep", "-f", "mysqld")
			if _, err := cmd.Output(); err == nil {
				serviceRunning = true
			}
		}

		// If service appears to be down, return immediately
		if !serviceRunning {
			return false, nil
		}
	}

	// At this point, the service appears to be running, let's verify connectivity
	// Skip connectivity check if no config is provided
	if cfg == nil {
		logger.Warn("No config provided for MariaDB connection check, relying only on service status")
		return true, nil
	}

	// This ensures the service is not just running but actually functional
	dbConfig := GetDBConfigFromConfig(cfg)

	// Set a short timeout for this check
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=5s",
		dbConfig.Username, dbConfig.Password, dbConfig.Host, dbConfig.Port, dbConfig.Database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		logger.Warn("MariaDB service appears to be running but connection failed",
			logger.String("error", err.Error()))
		return false, nil
	}
	defer db.Close()

	// Set a short context timeout for ping
	db.SetConnMaxLifetime(time.Second * 5)

	// Test connection with ping
	if err := db.Ping(); err != nil {
		logger.Warn("MariaDB service appears to be running but ping failed",
			logger.String("error", err.Error()))
		return false, nil
	}

	// Successfully connected and pinged MariaDB
	return true, nil
}

// ControlMariaDBService executes a control command on the MariaDB service
func ControlMariaDBService(serviceName, action string) error {
	logger.Info("Attempting to control MariaDB service",
		logger.String("service", serviceName),
		logger.String("action", action))

	// Try using systemctl first (systemd-based systems)
	cmd := exec.Command("systemctl", action, serviceName)
	if err := cmd.Run(); err == nil {
		logger.Info("Successfully controlled MariaDB service using systemctl",
			logger.String("service", serviceName),
			logger.String("action", action))
		return nil
	}

	// If systemctl fails, try the service command (for init.d systems)
	cmd = exec.Command("service", serviceName, action)
	if err := cmd.Run(); err == nil {
		logger.Info("Successfully controlled MariaDB service using service command",
			logger.String("service", serviceName),
			logger.String("action", action))
		return nil
	}

	// If both methods fail, return an error
	errMsg := fmt.Sprintf("failed to %s MariaDB service", action)
	logger.Error(errMsg,
		logger.String("service", serviceName))
	return fmt.Errorf(errMsg)
}

// StartMariaDBService starts the MariaDB service
func StartMariaDBService(serviceName string) error {
	return ControlMariaDBService(serviceName, "start")
}

// StopMariaDBService stops the MariaDB service
func StopMariaDBService(serviceName string) error {
	return ControlMariaDBService(serviceName, "stop")
}

// RestartMariaDBService restarts the MariaDB service
func RestartMariaDBService(serviceName string) error {
	// Log the restart attempt
	logger.Info("Attempting to restart MariaDB service",
		logger.String("service", serviceName))

	// Use systemctl to restart the service
	cmd := exec.Command("systemctl", "restart", serviceName)
	output, err := cmd.CombinedOutput()

	if err != nil {
		logger.Error("Failed to restart MariaDB service",
			logger.String("service", serviceName),
			logger.String("error", err.Error()),
			logger.String("output", string(output)))
		return fmt.Errorf("failed to restart MariaDB service: %w", err)
	}

	logger.Info("Successfully restarted MariaDB service",
		logger.String("service", serviceName))

	return nil
}
