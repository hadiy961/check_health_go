package cpu

import (
	"CheckHealthDO/internal/alerts"
	"CheckHealthDO/internal/notifications"
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"CheckHealthDO/internal/websocket"
	"context"
	"fmt"
	"sync"
	"time"
)

// Monitor handles periodic CPU monitoring
type Monitor struct {
	config          *config.Config
	ticker          *time.Ticker
	stopChan        chan struct{}
	isRunning       bool
	mutex           sync.Mutex
	lastInfo        *CPUInfo
	lastAlertTime   time.Time
	emailManager    *notifications.EmailManager
	checkCount      int // Counter for reducing log frequency
	alertHandler    *AlertHandler
	summaryReporter *SummaryReporter
}

// NewMonitor creates a new CPU monitor instance
func NewMonitor(cfg *config.Config) *Monitor {
	m := &Monitor{
		config:       cfg,
		stopChan:     make(chan struct{}),
		emailManager: notifications.NewEmailManager(cfg),
	}
	m.alertHandler = NewAlertHandler(m)
	m.summaryReporter = NewSummaryReporter(m, cfg)
	return m
}

// StartMonitoring begins the CPU monitoring process
func (m *Monitor) StartMonitoring() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.isRunning {
		return fmt.Errorf("CPU monitor is already running")
	}

	// Check if CPU monitoring is enabled
	if !m.config.Monitoring.CPU.Enabled {
		return fmt.Errorf("CPU monitoring is disabled in configuration")
	}

	interval := time.Duration(m.config.Monitoring.CPU.CheckInterval) * time.Second
	m.ticker = time.NewTicker(interval)
	m.isRunning = true

	logger.Info("Starting CPU monitor",
		logger.Int("interval_seconds", m.config.Monitoring.CPU.CheckInterval),
		logger.Float64("warning_threshold", m.config.Monitoring.CPU.WarningThreshold),
		logger.Float64("critical_threshold", m.config.Monitoring.CPU.CriticalThreshold))

	// Run the first check immediately, then continue at intervals
	go func() {
		m.CheckCPU() // Update this call

		for {
			select {
			case <-m.ticker.C:
				m.CheckCPU() // Update this call
			case <-m.stopChan:
				m.ticker.Stop()
				return
			}
		}
	}()

	return nil
}

// StopMonitoring halts the CPU monitoring process
func (m *Monitor) StopMonitoring() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isRunning {
		return
	}

	close(m.stopChan)
	m.isRunning = false
	logger.Info("CPU monitor stopped")
}

// CheckCPU performs a single CPU check
func (m *Monitor) CheckCPU() {
	info, err := GetCPUInfo(
		m.config.Monitoring.CPU.WarningThreshold,
		m.config.Monitoring.CPU.CriticalThreshold,
	)

	if err != nil {
		logger.Error("Failed to get CPU info",
			logger.String("error", err.Error()))
		return
	}

	// Check if status changed from the last check
	statusChanged := false
	m.mutex.Lock()
	if m.lastInfo != nil && m.lastInfo.CPUStatus != info.CPUStatus {
		statusChanged = true
		// Log the status change using the dedicated status logger
		GetStatusLogger().LogStatusChange(m.lastInfo.CPUStatus, info.CPUStatus, info.Usage)
	}

	// Store the latest metrics
	m.lastInfo = info
	m.mutex.Unlock()

	// Only log detailed information if not a status change but at a lower frequency
	m.checkCount++
	if !statusChanged && m.checkCount%60 == 0 { // Log once every ~60 checks
		logger.Info("CPU status",
			logger.Float64("usage_percent", info.Usage),
			logger.String("status", info.CPUStatus),
			logger.Int("cores", info.Cores),
			logger.Int("threads", info.Threads),
			logger.Float64("clock_speed", info.ClockSpeed))
	}

	// Format timestamp consistently for all messages
	timestamp := time.Now()
	formattedTime := timestamp.Format(time.RFC3339)

	// Create a completely separate metrics structure to ensure no overlap with memory data
	// Using a deeply nested structure with explicit metric type identification
	combinedMsg := map[string]interface{}{
		"metric_type": "cpu", // Explicit identifier for the metric type
		"metrics_data": map[string]interface{}{
			"cpu_info": map[string]interface{}{
				"model_name":      info.ModelName,
				"cores":           info.Cores,
				"threads":         info.Threads,
				"clock_speed":     info.ClockSpeed,
				"usage":           info.Usage,
				"core_usage":      info.CoreUsage,
				"cache_size":      info.CacheSize,
				"cpu_status":      info.CPUStatus,
				"is_virtual":      info.IsVirtual,
				"hypervisor":      info.Hypervisor,
				"status":          info.CPUStatus,
				"vendor_id":       info.VendorID,
				"family":          info.Family,
				"stepping":        info.Stepping,
				"physical_id":     info.PhysicalID,
				"microcode":       info.Microcode,
				"architecture":    info.Architecture,
				"min_frequency":   info.MinFrequency,
				"max_frequency":   info.MaxFrequency,
				"processor_count": info.ProcessorCount,
				"cpu_times":       info.CPUTimes,
				"temperature":     info.Temperature,
			},
		},
		"meta": map[string]interface{}{
			"timestamp":        timestamp,
			"last_update_time": formattedTime,
			"source":           "cpu_monitor",
			"version":          "1.0",
		},
	}

	// Get the registry
	registry := websocket.GetRegistry()

	// Broadcast to CPU-specific WebSocket with the dedicated CPU structure
	if handler := registry.GetCPUHandler(); handler != nil {
		registry.BroadcastCPU(combinedMsg)
	}

	// Record the event for summary reporting
	m.summaryReporter.RecordEvent(info)

	// Process alerts based on status
	switch info.CPUStatus {
	case "normal":
		m.alertHandler.HandleNormalAlert(info, statusChanged)
	case "warning":
		m.alertHandler.HandleWarningAlert(info, statusChanged)
	case "critical":
		m.alertHandler.HandleCriticalAlert(info, statusChanged)
	}
}

// GetLastCPUInfo returns the most recently captured CPU information
func (m *Monitor) GetLastCPUInfo() *CPUInfo {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.lastInfo
}

// GetConfig returns the monitor's configuration
// Modified to return interface{} to match the alerts.ConfigProvider interface
func (m *Monitor) GetConfig() interface{} {
	return m.config
}

// GetConfigPtr returns the monitor's configuration as a concrete type pointer
// This provides typed access to the config when needed internally
func (m *Monitor) GetConfigPtr() *config.Config {
	return m.config
}

// UpdateLastAlertTime updates the last alert time
func (m *Monitor) UpdateLastAlertTime() {
	m.lastAlertTime = time.Now()
}

// GetLastAlertTime returns the last alert time
func (m *Monitor) GetLastAlertTime() time.Time {
	return m.lastAlertTime
}

// GetNotificationManagers returns the notification managers
func (m *Monitor) GetNotificationManagers() alerts.NotificationManager {
	return m.emailManager
}

// StartBackgroundMonitor creates and starts a CPU monitor in a background goroutine
// Returns a function to stop monitoring
func StartBackgroundMonitor(ctx context.Context, cfg *config.Config) (func(), error) {
	monitor := NewMonitor(cfg)

	if err := monitor.StartMonitoring(); err != nil {
		return nil, err
	}

	// Return a function that can be called to stop monitoring
	return func() {
		monitor.StopMonitoring()
	}, nil
}
