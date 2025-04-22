package memory

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

// Monitor handles periodic memory monitoring
type Monitor struct {
	config          *config.Config
	ticker          *time.Ticker
	stopChan        chan struct{}
	isRunning       bool
	mutex           sync.Mutex
	lastInfo        *MemoryInfo
	lastAlertTime   time.Time
	emailManager    *notifications.EmailManager
	checkCount      int // Counter for reducing log frequency
	alertHandler    *AlertHandler
	summaryReporter *SummaryReporter // Add this field
	usageReadings   []float64        // Store recent memory readings
	maxReadings     int              // Maximum number of readings to store
	readingInterval time.Duration    // Time between readings
	lastReadingTime time.Time        // When the last reading was taken
}

// NewMonitor creates a new memory monitor instance
func NewMonitor(cfg *config.Config) *Monitor {
	m := &Monitor{
		config:          cfg,
		stopChan:        make(chan struct{}),
		emailManager:    notifications.NewEmailManager(cfg),
		usageReadings:   make([]float64, 0, 10), // Store last 10 readings
		maxReadings:     10,
		readingInterval: time.Minute, // Take readings every minute for trend analysis
		lastReadingTime: time.Time{},
	}
	m.alertHandler = NewAlertHandler(m)
	m.summaryReporter = NewSummaryReporter(m, cfg) // Initialize the summary reporter
	return m
}

// StartMonitoring begins the memory monitoring process
func (m *Monitor) StartMonitoring() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.isRunning {
		return fmt.Errorf("memory monitor is already running")
	}

	// Check if memory monitoring is enabled
	if !m.config.Monitoring.Memory.Enabled {
		return fmt.Errorf("memory monitoring is disabled in configuration")
	}

	interval := time.Duration(m.config.Monitoring.Memory.CheckInterval) * time.Second
	m.ticker = time.NewTicker(interval)
	m.isRunning = true

	logger.Info("Starting memory monitor",
		logger.Int("interval_seconds", m.config.Monitoring.Memory.CheckInterval),
		logger.Float64("warning_threshold", m.config.Monitoring.Memory.WarningThreshold),
		logger.Float64("critical_threshold", m.config.Monitoring.Memory.CriticalThreshold))

	// Run the first check immediately, then continue at intervals
	go func() {
		m.checkMemory()

		for {
			select {
			case <-m.ticker.C:
				m.checkMemory()
			case <-m.stopChan:
				m.ticker.Stop()
				return
			}
		}
	}()

	return nil
}

// StopMonitoring halts the memory monitoring process
func (m *Monitor) StopMonitoring() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isRunning {
		return
	}

	close(m.stopChan)
	m.isRunning = false
	logger.Info("Memory monitor stopped")
}

// checkMemory performs a single memory check
func (m *Monitor) checkMemory() {
	info, err := GetMemoryInfo(
		m.config.Monitoring.Memory.WarningThreshold,
		m.config.Monitoring.Memory.CriticalThreshold,
	)

	if err != nil {
		logger.Error("Failed to get memory info",
			logger.String("error", err.Error()))
		return
	}

	// Check if status changed from the last check
	statusChanged := false
	m.mutex.Lock()
	if m.lastInfo != nil && m.lastInfo.MemoryStatus != info.MemoryStatus {
		statusChanged = true
		// Log the status change using the dedicated status logger
		GetStatusLogger().LogStatusChange(m.lastInfo.MemoryStatus, info.MemoryStatus, info.UsedMemoryPercentage)
	}

	// Store the latest metrics
	m.lastInfo = info
	m.mutex.Unlock()

	// Only log detailed information if not a status change but at a lower frequency
	m.checkCount++
	if !statusChanged && m.checkCount%60 == 0 { // Log once every ~60 checks
		logger.Info("Memory status",
			logger.Float64("usage_percent", info.UsedMemoryPercentage),
			logger.String("status", info.MemoryStatus),
			logger.String("total", FormatBytes(info.TotalMemory)),
			logger.String("used", FormatBytes(info.UsedMemory)),
			logger.String("free", FormatBytes(info.FreeMemory)))
	}

	// Format timestamp consistently for all messages
	timestamp := time.Now()
	formattedTime := timestamp.Format(time.RFC3339)

	// Create a completely separate metrics structure to ensure no overlap with CPU data
	// Using a deeply nested structure with explicit metric type identification to match CPU format
	combinedMsg := map[string]interface{}{
		"metric_type": "memory", // Explicit identifier for the metric type
		"metrics_data": map[string]interface{}{
			"memory_info": map[string]interface{}{
				"total_memory":           info.TotalMemory,
				"used_memory":            info.UsedMemory,
				"free_memory":            info.FreeMemory,
				"used_memory_percentage": info.UsedMemoryPercentage,
				"free_memory_percentage": info.FreeMemoryPercentage,
				"memory_status":          info.MemoryStatus,
				"swap_total":             info.SwapTotal,
				"swap_used":              info.SwapUsed,
				"swap_free":              info.SwapFree,
			},
		},
		"meta": map[string]interface{}{
			"timestamp":        timestamp,
			"last_update_time": formattedTime,
			"source":           "memory_monitor",
			"version":          "1.0",
		},
	}

	// Also broadcast to memory-specific WebSocket
	registry := websocket.GetRegistry()
	if handler := registry.GetMemoryHandler(); handler != nil {
		registry.BroadcastMemory(combinedMsg)
	}

	// Record the event for summary reporting
	m.summaryReporter.RecordEvent(info)

	// Record usage for trend analysis if enough time has passed
	if time.Since(m.lastReadingTime) >= m.readingInterval {
		m.mutex.Lock()
		m.usageReadings = append(m.usageReadings, info.UsedMemoryPercentage)
		if len(m.usageReadings) > m.maxReadings {
			m.usageReadings = m.usageReadings[1:] // Remove oldest reading
		}
		m.lastReadingTime = time.Now()
		m.mutex.Unlock()
	}

	// Process alerts based on status
	switch info.MemoryStatus {
	case "normal":
		m.alertHandler.HandleNormalAlert(info, statusChanged)
	case "warning":
		m.alertHandler.HandleWarningAlert(info, statusChanged)
	case "critical":
		m.alertHandler.HandleCriticalAlert(info, statusChanged)
	}
}

// GetLastMemoryInfo returns the most recently captured memory information
func (m *Monitor) GetLastMemoryInfo() *MemoryInfo {
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

// StartBackgroundMonitor creates and starts a memory monitor in a background goroutine
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

// Add this function to compute memory usage trend
func (m *Monitor) getMemoryTrend() (trend string, increasePct float64) {
	if len(m.usageReadings) < 2 {
		return "stable", 0.0
	}

	// Calculate the average of the first half vs the second half of readings
	midpoint := len(m.usageReadings) / 2
	var firstHalfSum, secondHalfSum float64

	for i := 0; i < midpoint; i++ {
		firstHalfSum += m.usageReadings[i]
	}

	for i := midpoint; i < len(m.usageReadings); i++ {
		secondHalfSum += m.usageReadings[i]
	}

	firstHalfAvg := firstHalfSum / float64(midpoint)
	secondHalfAvg := secondHalfSum / float64(len(m.usageReadings)-midpoint)

	percentChange := ((secondHalfAvg - firstHalfAvg) / firstHalfAvg) * 100.0

	switch {
	case percentChange > 5.0:
		return "rapidly increasing", percentChange
	case percentChange > 1.0:
		return "increasing", percentChange
	case percentChange < -5.0:
		return "rapidly decreasing", percentChange
	case percentChange < -1.0:
		return "decreasing", percentChange
	default:
		return "stable", percentChange
	}
}
