package mariadb

import (
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"CheckHealthDO/internal/services/mariadb"
	"CheckHealthDO/internal/websocket"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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

	logger.Info("API-initiated MariaDB action flagged",
		logger.String("action", actionType),
		logger.Any("initiated_at", m.apiActionTime))
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

	// FIRST: Check for memory auto-recovery using the log messages and journal
	// This check has the highest priority
	
	// Check our internal log file first
	logDir := "logs"
	logFile := filepath.Join(logDir, "mariadb_restarts.log")
	
	// First check if the log file exists
	if _, err := os.Stat(logFile); err == nil {
		// Check for recent entries (within last 5 minutes)
		recentLogCmd := exec.Command("bash", "-c", 
			fmt.Sprintf("grep -a 'Memory Critical Auto-Recovery' %s | tail -n 10", logFile))
		logOutput, err := recentLogCmd.CombinedOutput()
		
		if err == nil && len(logOutput) > 0 {
			logEntries := strings.Split(string(logOutput), "\n")
			for _, entry := range logEntries {
				// Skip empty entries
				if entry == "" {
					continue
				}
				
				// Try to parse the timestamp from the log entry
				timestampPattern := regexp.MustCompile(`\[(.*?)\]`)
				matches := timestampPattern.FindStringSubmatch(entry)
				
				if len(matches) > 1 {
					timestampStr := matches[1]
					timestamp, parseErr := time.Parse(time.RFC3339, timestampStr)
					
					// If we found a recent entry (within last 5 minutes)
					if parseErr == nil && time.Since(timestamp) < 5*time.Minute {
						logger.Info("Found evidence of recent auto-recovery in logs",
							logger.String("log_entry", entry))
						return "Memory Critical Auto-Recovery", fmt.Sprintf("MariaDB was automatically restarted due to critical memory conditions: %s", entry)
					}
				}
			}
		}
	}
	
	// Check system journal for our specific auto-recovery message
	journalCmd := exec.Command("bash", "-c", 
		"journalctl --since='5 minutes ago' | grep -i 'CHECKHEALTHDO_MEMORY_AUTO_RECOVERY' | tail -n 5")
	journalOutput, _ := journalCmd.CombinedOutput()
	
	if len(journalOutput) > 0 {
		logger.Info("Found evidence of memory-triggered restart in system logs",
			logger.String("output", string(journalOutput)))
		return "Memory Critical Auto-Recovery", fmt.Sprintf("MariaDB was automatically restarted due to critical memory conditions (from journal): %s", string(journalOutput))
	}
	
	// SECOND: Check for any restart/memory related entries in recent system logs
	memoryRestartCmd := exec.Command("bash", "-c", 
		fmt.Sprintf("journalctl -u %s --since='5 minutes ago' | grep -i 'restart.*memory\\|memory.*restart\\|critical memory' | tail -n 5", 
			serviceName))
	memoryOutput, _ := memoryRestartCmd.CombinedOutput()
	
	if len(memoryOutput) > 0 {
		logger.Info("Found evidence of memory-related restart in MariaDB logs",
			logger.String("output", string(memoryOutput)))
		return "Memory Critical Auto-Recovery", fmt.Sprintf("MariaDB was automatically restarted due to critical memory conditions: %s", string(memoryOutput))
	}
	
	// THIRD: Check for OOM kills in kernel logs
	oomCmd := exec.Command("bash", "-c", "journalctl -k -b | grep -i 'killed process' | grep -i 'mysqld\\|mariadb' | tail -n 5")
	oomOutput, err := oomCmd.CombinedOutput()
	if err == nil && len(oomOutput) > 0 {
		logger.Info("Found OOM kill evidence in logs",
			logger.String("service", serviceName),
			logger.String("logs", string(oomOutput)))
		return "Out of Memory Kill", string(oomOutput)
	}

	// FOURTH: Check for manual systemctl stop
	manualStopCmd := exec.Command("bash", "-c",
		fmt.Sprintf("journalctl -u %s -n 50 --no-pager | grep -i 'systemctl stop\\|Stopped.*mariadb\\|Stopping.*mariadb' | tail -n 5",
			serviceName))
	manualOutput, _ := manualStopCmd.CombinedOutput()

	if len(manualOutput) > 0 {
		outputStr := string(manualOutput)

		// Don't identify as manual stop if it appears to be a memory-related restart
		if strings.Contains(outputStr, "memory") || strings.Contains(outputStr, "auto-recovery") {
			return "Memory Critical Auto-Recovery", fmt.Sprintf("MariaDB was automatically restarted due to critical memory conditions (detected in stop logs): %s", outputStr)
		}

		// Look for systemctl stop command
		if strings.Contains(outputStr, "systemctl stop") || strings.Contains(outputStr, "systemd[1]: Stopped") {
			// Try to extract who stopped it
			userPattern := regexp.MustCompile(`by\s+(\w+)`)
			matches := userPattern.FindStringSubmatch(outputStr)

			if len(matches) > 1 {
				// Found specific user who ran the command
				user := matches[1]

				// Check if this is a system user that might be involved in automated tasks
				if user == "root" || user == "system" || user == "systemd" {
					// Double-check recent activity for memory-related actions
					recentCmd := exec.Command("bash", "-c",
						fmt.Sprintf("journalctl -n 100 --since='5 minutes ago' | grep -i 'memory\\|restart\\|critical'"))
					recentOutput, _ := recentCmd.CombinedOutput()

					if len(recentOutput) > 0 && (strings.Contains(string(recentOutput), "memory") ||
						strings.Contains(string(recentOutput), "critical") ||
						strings.Contains(string(recentOutput), "auto-recovery")) {
						return "Memory Critical Auto-Recovery", fmt.Sprintf("MariaDB was automatically restarted due to critical memory conditions (system-initiated): %s", string(recentOutput))
					}
				}

				// If we get here, it's likely a genuine manual stop
				return "Manual Systemctl Stop", fmt.Sprintf("MariaDB was manually stopped by user '%s' via systemctl command: %s", user, outputStr)
			}

			// We can see it was systemctl but not who ran it
			// Do one more check for memory-related restart
			recentCmd := exec.Command("bash", "-c",
				fmt.Sprintf("journalctl -n 100 --since='5 minutes ago' | grep -i 'memory\\|restart\\|critical'"))
			recentOutput, _ := recentCmd.CombinedOutput()

			if len(recentOutput) > 0 && (strings.Contains(string(recentOutput), "memory") ||
				strings.Contains(string(recentOutput), "critical") ||
				strings.Contains(string(recentOutput), "auto-recovery")) {
				return "Memory Critical Auto-Recovery", fmt.Sprintf("MariaDB was automatically restarted due to critical memory conditions (system-initiated): %s", string(recentOutput))
			}

			return "Manual Systemctl Stop", fmt.Sprintf("MariaDB was manually stopped via systemctl command: %s", outputStr)
		}
	}

	// FIFTH: Check for explicit manual stop via service status
	statusCmd := exec.Command("bash", "-c", fmt.Sprintf("systemctl status %s | grep -i 'inactive\\|failed\\|stopped'", serviceName))
	statusOutput, err := statusCmd.CombinedOutput()
	if err == nil && len(statusOutput) > 0 {
		outputStr := string(statusOutput)

		// Don't identify as manual stop if it appears to be a memory-related restart
		if strings.Contains(outputStr, "memory") || strings.Contains(outputStr, "auto-recovery") {
			return "Memory Critical Auto-Recovery", fmt.Sprintf("MariaDB was automatically restarted due to critical memory conditions (detected in status): %s", outputStr)
		}

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

	// SIXTH: Check systemd journal logs for service failures
	journalCmd = exec.Command("bash", "-c", fmt.Sprintf("journalctl -u %s --no-pager -n 50 | grep -i 'fail\\|error\\|terminate\\|abort\\|denied\\|shutdown'", serviceName))
	journalOutput, err = journalCmd.CombinedOutput()
	if err == nil && len(journalOutput) > 0 {
		// Look for specific patterns in the output
		outputStr := string(journalOutput)

		// Check for normal shutdown
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

	// Check MySQL error logs as a last resort
	logCmd := exec.Command("bash", "-c", "tail -n 50 /var/log/mysql/error.log 2>/dev/null || tail -n 50 /var/log/mysqld.log 2>/dev/null")
	logOutput, err := logCmd.CombinedOutput()
	if err == nil && len(logOutput) > 0 {
		return "Database Error", string(logOutput)
	}

	// If we couldn't determine a specific reason
	return "Unknown Failure", "Could not determine the specific reason for service failure"
}

// getStartReason attempts to determine why MariaDB service started
func (m *Monitor) getStartReason() (string, string) {
	serviceName := m.config.Monitoring.MariaDB.ServiceName

	// First check for manual service start via systemctl
	syslogCmd := exec.Command("bash", "-c",
		fmt.Sprintf("journalctl -u %s -n 100 --no-pager | grep -i 'Starting\\|Started.*mariadb\\|systemctl start' | tail -n 10",
			serviceName))
	syslogOutput, _ := syslogCmd.CombinedOutput()

	if len(syslogOutput) > 0 {
		syslogStr := string(syslogOutput)

		// Check for manual systemctl start command
		if strings.Contains(syslogStr, "systemctl start") || strings.Contains(syslogStr, "systemd[1]: Started") {
			// Try to extract the user who ran the command
			userPattern := regexp.MustCompile(`by\s+(\w+)`)
			matches := userPattern.FindStringSubmatch(syslogStr)

			if len(matches) > 1 {
				// Found specific user who ran the command
				user := matches[1]
				return fmt.Sprintf("MariaDB manually started by user '%s' via systemctl", user), syslogStr
			}

			// We can see it was systemctl but not who ran it
			return "MariaDB manually started via systemctl command", syslogStr
		}
	}

	// Check if it's a system boot
	bootCmd := exec.Command("bash", "-c",
		"journalctl -b -n 100 | grep -i 'system startup\\|boot\\|reboot'")
	bootOutput, bootErr := bootCmd.CombinedOutput()

	if bootErr == nil && len(bootOutput) > 0 &&
		time.Since(m.status.Timestamp) < 10*time.Minute {
		return "System startup detected - MariaDB service started during boot process", string(bootOutput)
	}

	// Check for memory marker file indicating recovery restart
	logFile := filepath.Join("logs", "mariadb_restarts.log")
	if _, err := os.Stat(logFile); err == nil {
		// Check for recent entries (within last 5 minutes)
		recentLogCmd := exec.Command("bash", "-c", 
			fmt.Sprintf("grep -a 'Memory Critical Auto-Recovery' %s | tail -n 10", logFile))
		logOutput, err := recentLogCmd.CombinedOutput()
		
		if err == nil && len(logOutput) > 0 {
			return "Service restarted after memory-related shutdown", string(logOutput)
		}
	}

	// Check system journal for our specific auto-recovery message
	journalCmd := exec.Command("bash", "-c", 
		"journalctl --since='5 minutes ago' | grep -i 'CHECKHEALTHDO_MEMORY_AUTO_RECOVERY_COMPLETED' | tail -n 5")
	journalOutput, _ := journalCmd.CombinedOutput()
	
	if len(journalOutput) > 0 {
		return "Service restarted after memory-related shutdown", string(journalOutput)
	}

	// Check for MySQL startup messages in logs
	logCmd := exec.Command("bash", "-c",
		"tail -n 100 /var/log/mysql/error.log 2>/dev/null || tail -n 100 /var/log/mysqld.log 2>/dev/null")
	logOutput, _ := logCmd.CombinedOutput()

	if len(logOutput) > 0 {
		logStr := string(logOutput)
		if strings.Contains(logStr, "starting") || strings.Contains(logStr, "started") {
			return "MariaDB service started (found startup messages in logs)", logStr
		}
	}

	// Check process start time
	processCmd := exec.Command("bash", "-c", "ps -o lstart= -C mysqld")
	processOutput, _ := processCmd.CombinedOutput()

	if len(processOutput) > 0 {
		startTimeStr := strings.TrimSpace(string(processOutput))
		return fmt.Sprintf("MariaDB service started at %s", startTimeStr),
			"Process start time detected from system"
	}

	// Default message with more context
	uptime, _ := getSystemUptime()
	loadAvg, _ := getSystemLoadAverage()

	details := fmt.Sprintf("System uptime: %s, Load average: %.2f, %.2f, %.2f",
		uptime, loadAvg[0], loadAvg[1], loadAvg[2])

	return "MariaDB service started (unable to determine specific trigger)", details
}

// Helper function to get system uptime
func getSystemUptime() (string, error) {
	cmd := exec.Command("bash", "-c", "uptime -p")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// Helper function to get system load average
func getSystemLoadAverage() ([]float64, error) {
	cmd := exec.Command("bash", "-c", "cat /proc/loadavg")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return []float64{0, 0, 0}, err
	}

	parts := strings.Fields(string(output))
	if len(parts) < 3 {
		return []float64{0, 0, 0}, fmt.Errorf("invalid load average format")
	}

	load1, _ := strconv.ParseFloat(parts[0], 64)
	load5, _ := strconv.ParseFloat(parts[1], 64)
	load15, _ := strconv.ParseFloat(parts[2], 64)

	return []float64{load1, load5, load15}, nil
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

				if strings.Contains(stopReason, "Manual Systemctl Stop") {
					reason = errorDetails // Use the full details as reason
				} else {
					reason = fmt.Sprintf("MariaDB service unexpectedly stopped - Reason: %s", stopReason)
				}

				// Log detailed error information
				logger.Error("MariaDB service stopped",
					logger.String("reason", stopReason),
					logger.String("details", errorDetails))
			} else if previousStatus == "stopped" && m.status.Status == "running" {
				// Get more detailed information about the service start
				startReason, startDetails := m.getStartReason()

				// Use the start reason directly, it's already formatted well
				reason = startReason

				// Log the detailed start information
				logger.Info("MariaDB service started",
					logger.String("reason", startReason),
					logger.String("details", startDetails))
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
