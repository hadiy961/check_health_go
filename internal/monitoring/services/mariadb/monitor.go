package mariadb

import (
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"CheckHealthDO/internal/services/mariadb"
	"CheckHealthDO/internal/websocket"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Status represents the current status of the MariaDB service
type Status struct {
	Status            string    `json:"status"`            // "running" or "stopped"
	ServiceName       string    `json:"service_name"`      // Service name (e.g., "mariadb")
	Timestamp         time.Time `json:"timestamp"`         // Time of status check
	Version           string    `json:"version,omitempty"` // MariaDB version (if running)
	UptimeSeconds     int64     `json:"uptime_seconds,omitempty"`
	MemoryUsed        int64     `json:"memory_used,omitempty"`         // Memory used by MariaDB in bytes
	MemoryUsedPercent float64   `json:"memory_used_percent,omitempty"` // Percentage of system memory used by MariaDB
	ConnectionsActive int       `json:"connections_active,omitempty"`  // Active connections count
	Message           string    `json:"message,omitempty"`             // Additional status message
	LastUpdateTime    time.Time `json:"last_update_time"`              // Last time the status was updated
	StatusChanged     bool      `json:"-"`                             // Indicates if the status has changed (not sent to clients)
	LastStatus        string    `json:"-"`                             // Last known status (not sent to clients)
	PreviousStatus    string    `json:"previous_status,omitempty"`     // Previous status for reference
	StopReason        string    `json:"stop_reason,omitempty"`         // Reason why MariaDB stopped
	StopErrorDetails  string    `json:"stop_error_details,omitempty"`  // Detailed error information
}

// Monitor handles MariaDB service monitoring
type Monitor struct {
	config             *config.Config
	status             *Status
	mu                 sync.RWMutex
	stopCh             chan struct{}
	statusChanged      bool
	notifier           *Notifier
	apiInitiatedChange bool         // Tracks if a change was initiated by the API
	apiActionTime      time.Time    // When the API action was initiated
	apiActionType      string       // Type of API action (start/stop/restart)
	apiActionMu        sync.RWMutex // Mutex for API action tracking
}

// NewMonitor creates a new MariaDB monitor
func NewMonitor(cfg *config.Config) (*Monitor, error) {
	// Validate config if needed
	if cfg == nil {
		return nil, fmt.Errorf("invalid configuration: nil config")
	}

	return &Monitor{
		config:   cfg,
		status:   &Status{LastStatus: "unknown"},
		stopCh:   make(chan struct{}),
		notifier: NewNotifier(cfg),
	}, nil
}

// StartBackgroundMonitor starts the monitoring process in the background
func StartBackgroundMonitor(ctx context.Context, cfg *config.Config) (func(), error) {
	if !cfg.Monitoring.MariaDB.Enabled {
		logger.Info("MariaDB monitoring is disabled in config, not starting monitor")
		return func() {}, nil
	}

	monitor, err := NewMonitor(cfg)
	if err != nil {
		return nil, err
	}

	// Register with the WebSocket registry handler using MariaDB-specific handler
	registry := websocket.GetRegistry()
	handler := registry.GetMariaDBHandler() // Use MariaDB handler instead of metrics handler
	if handler == nil {
		handler = websocket.NewHandler()
		registry.RegisterMariaDBHandler(handler)
	}

	go monitor.Start(ctx)

	logger.Info("Started MariaDB monitoring service")

	return func() {
		monitor.Stop()
		logger.Info("Stopped MariaDB monitoring service")
	}, nil
}

// Start begins the monitoring process
func (m *Monitor) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(m.config.Monitoring.MariaDB.CheckInterval) * time.Second)
	defer ticker.Stop()

	// Run immediately at start
	m.checkStatus()

	for {
		select {
		case <-ticker.C:
			m.checkStatus()
		case <-ctx.Done():
			m.Stop()
			return
		case <-m.stopCh:
			return
		}
	}
}

// Stop stops the monitoring process
func (m *Monitor) Stop() {
	close(m.stopCh)
	// No need to close WebSocket clients, as we're using the central registry
}

// GetStatus returns the current MariaDB status
func (m *Monitor) GetStatus() *Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// GetConfig returns the monitor's configuration
func (m *Monitor) GetConfig() *config.Config {
	return m.config
}

// MarkAPIAction sets a flag indicating that a change was initiated by the API
// Should be called by API handlers before starting/stopping the service
func (m *Monitor) MarkAPIAction(actionType string) {
	m.apiActionMu.Lock()
	defer m.apiActionMu.Unlock()

	m.apiInitiatedChange = true
	m.apiActionTime = time.Now()
	m.apiActionType = actionType

	logger.Info("API-initiated MariaDB service action marked",
		logger.String("action", actionType),
		logger.Any("time", m.apiActionTime))
}

// ClearAPIAction clears the API action flag after a certain time period
// This is automatically called during status checks
func (m *Monitor) ClearAPIAction() {
	m.apiActionMu.Lock()
	defer m.apiActionMu.Unlock()

	// If it's been more than 30 seconds since the API action, clear the flag
	if m.apiInitiatedChange && time.Since(m.apiActionTime) > 30*time.Second {
		logger.Debug("Clearing API-initiated action flag",
			logger.String("action", m.apiActionType),
			logger.Any("initiated_at", m.apiActionTime))

		m.apiInitiatedChange = false
		m.apiActionType = ""
	}
}

// wasChangeInitiatedByAPI attempts to determine if the status change was from API
func (m *Monitor) wasChangeInitiatedByAPI() bool {
	m.apiActionMu.RLock()
	defer m.apiActionMu.RUnlock()

	// If the flag is set and the API action was recent (within 30 seconds)
	if m.apiInitiatedChange && time.Since(m.apiActionTime) <= 30*time.Second {
		// For restart, we expect a stop followed by a start, so keep the flag
		if m.apiActionType == "restart" {
			logger.Info("Status change was initiated by API restart operation")
			return true
		}

		// For start/stop operations, check if the current status matches the expected outcome
		currentStatus := m.status.Status
		if (m.apiActionType == "start" && currentStatus == "running") ||
			(m.apiActionType == "stop" && currentStatus == "stopped") {
			logger.Info("Status change was initiated by API operation",
				logger.String("action", m.apiActionType),
				logger.String("resulting_status", currentStatus))
			return true
		}
	}

	return false
}

// getDatabaseStopReason attempts to determine why MariaDB service stopped
func (m *Monitor) getDatabaseStopReason() (string, string) {
	serviceName := m.config.Monitoring.MariaDB.ServiceName

	// Step 1: Check for OOM kills in kernel logs
	oomCmd := exec.Command("bash", "-c", "journalctl -k -b | grep -i 'killed process' | grep -i 'mysqld\\|mariadb' | tail -n 5")
	oomOutput, err := oomCmd.CombinedOutput()
	if err == nil && len(oomOutput) > 0 {
		logger.Info("Found OOM kill evidence in logs",
			logger.String("service", serviceName),
			logger.String("logs", string(oomOutput)))
		return "OOM Kill", string(oomOutput)
	}

	// Step 2: Check systemd journal logs for service failures
	journalCmd := exec.Command("bash", "-c", fmt.Sprintf("journalctl -u %s --no-pager -n 50 | grep -i 'fail\\|error\\|terminate\\|abort\\|denied\\|shutdown'", serviceName))
	journalOutput, err := journalCmd.CombinedOutput()
	if err == nil && len(journalOutput) > 0 {
		// Look for specific patterns in the output
		outputStr := string(journalOutput)

		// Check for disk space issues
		if strings.Contains(outputStr, "shutdown") || strings.Contains(outputStr, "Shutdown") {
			return "Shutdown Normal", outputStr
		}

		// Check for permission/access denied issues
		if strings.Contains(outputStr, "denied") || strings.Contains(outputStr, "permission") {
			return "Permission Error", outputStr
		}

		// Check for configuration errors
		if strings.Contains(outputStr, "configuration") || strings.Contains(outputStr, "config") {
			return "Configuration Error", outputStr
		}

		// Check for disk space issues
		if strings.Contains(outputStr, "disk space") || strings.Contains(outputStr, "no space") {
			return "Disk Space Error", outputStr
		}

		// If we found errors but couldn't categorize them, return generic error
		return "Service Error", outputStr
	}

	// Step 3: Check if service was manually stopped
	statusCmd := exec.Command("bash", "-c", fmt.Sprintf("systemctl status %s | grep -i 'inactive\\|failed\\|stopped'", serviceName))
	statusOutput, err := statusCmd.CombinedOutput()
	if err == nil && len(statusOutput) > 0 {
		outputStr := string(statusOutput)

		// Check for manual stop
		if strings.Contains(outputStr, "deactivated") || strings.Contains(outputStr, "stop") {
			// Try to extract who stopped it
			userPattern := regexp.MustCompile(`by\s+(\w+)`)
			matches := userPattern.FindStringSubmatch(outputStr)
			if len(matches) > 1 {
				return "Manual Stop", fmt.Sprintf("Service was manually stopped by user %s: %s", matches[1], outputStr)
			}
			return "Manual Stop", fmt.Sprintf("Service was manually stopped: %s", outputStr)
		}
	}

	// Check MySQL error logs as a last resort
	logCmd := exec.Command("bash", "-c", "tail -n 50 /var/log/mysql/error.log 2>/dev/null || tail -n 50 /var/log/mysqld.log 2>/dev/null")
	logOutput, err := logCmd.CombinedOutput()
	if err == nil && len(logOutput) > 0 {
		return "Database Error", string(logOutput)
	}

	// If we couldn't determine a specific reason
	return "Unknown Failure", "Could not determine the specific reason for service failure"
}

// checkStatus checks the MariaDB service status and updates internal state
func (m *Monitor) checkStatus() error {
	// Clear any expired API action flags
	m.ClearAPIAction()

	serviceName := m.config.Monitoring.MariaDB.ServiceName
	// Pass config to enable connection verification
	isRunning, err := mariadb.CheckServiceStatus(serviceName, m.config)

	if err != nil {
		logger.Error("Failed to check MariaDB service status",
			logger.String("error", err.Error()))
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Store previous status before updating
	previousStatus := m.status.Status

	// Create new status with current time
	now := time.Now()
	m.status.Timestamp = now
	m.status.LastUpdateTime = now
	m.status.ServiceName = serviceName

	if isRunning {
		m.status.Status = "running"
		m.populateAdditionalInfo()
		m.status.Message = "MariaDB service is running normally"
	} else {
		m.status.Status = "stopped"
		m.status.Message = "MariaDB service is currently stopped"
		// Clear metrics that are only relevant when running
		m.status.Version = ""
		m.status.UptimeSeconds = 0
		m.status.MemoryUsed = 0
		m.status.MemoryUsedPercent = 0
		m.status.ConnectionsActive = 0
	}

	// Check if status has changed
	if previousStatus != m.status.Status && previousStatus != "" {
		m.status.StatusChanged = true
		m.status.PreviousStatus = previousStatus
		m.statusChanged = true

		// Log the status change
		logger.Info("MariaDB service status changed",
			logger.String("previous", previousStatus),
			logger.String("current", m.status.Status))

		// Send notification if change was not initiated via API
		if !m.wasChangeInitiatedByAPI() {
			reason := "Unknown cause"
			m.status.StopReason = ""
			m.status.StopErrorDetails = ""

			if previousStatus == "running" && m.status.Status == "stopped" {
				// Get detailed information about why the service stopped
				stopReason, errorDetails := m.getDatabaseStopReason()
				m.status.StopReason = stopReason
				m.status.StopErrorDetails = errorDetails

				reason = fmt.Sprintf("MariaDB service unexpectedly stopped - Reason: %s", stopReason)

				// Log detailed error information
				logger.Error("MariaDB service unexpectedly stopped",
					logger.String("reason", stopReason),
					logger.String("details", errorDetails))
			} else if previousStatus == "stopped" && m.status.Status == "running" {
				reason = "MariaDB service unexpectedly started"
			}

			m.notifier.SendStatusChangeNotification(m.status, reason)
		}
	} else {
		m.status.StatusChanged = false
	}

	// Broadcast metrics via WebSocket after each status check
	m.broadcastMetrics()

	return nil
}

// broadcastMetrics sends the current status to all WebSocket clients using the registry
func (m *Monitor) broadcastMetrics() {
	wsMsg := MariaDBMetricsMsg{
		Timestamp:      time.Now(),
		Status:         m.status,
		LastUpdateTime: time.Now().Format(time.RFC3339),
	}

	// Use the MariaDB-specific broadcast method
	websocket.GetRegistry().BroadcastMariaDB(wsMsg)

}

// populateAdditionalInfo adds additional metrics when MariaDB is running
func (m *Monitor) populateAdditionalInfo() {
	dbConfig := mariadb.GetDBConfigFromConfig(m.config)

	// Get MariaDB version
	version, err := mariadb.GetVersion(dbConfig)
	if err != nil {
		logger.Warn("Failed to get MariaDB version",
			logger.String("error", err.Error()))
	} else {
		m.status.Version = version
	}

	// Get MariaDB uptime - refresh this every time to ensure it's real-time
	uptime, err := mariadb.GetUptime(dbConfig)
	if err != nil {
		logger.Warn("Failed to get MariaDB uptime",
			logger.String("error", err.Error()))
		// Don't reset uptime to 0 here, keep the last known value
	} else {
		// Always update the uptime with the latest value
		m.status.UptimeSeconds = uptime
	}

	// Get active connections
	connections, err := mariadb.GetActiveConnections(dbConfig)
	if err != nil {
		logger.Warn("Failed to get MariaDB connections",
			logger.String("error", err.Error()))
	} else {
		m.status.ConnectionsActive = connections
	}

	// Get memory usage
	memUsed, memUsedPercent, err := mariadb.GetMariaDBMemoryUsage()
	if err != nil {
		logger.Warn("Failed to get MariaDB memory usage",
			logger.String("error", err.Error()))
	} else {
		m.status.MemoryUsed = int64(memUsed)
		m.status.MemoryUsedPercent = memUsedPercent
	}
}
