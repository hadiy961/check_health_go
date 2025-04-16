package sysinfo

import (
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
	config    *config.Config
	ticker    *time.Ticker
	stopChan  chan struct{}
	isRunning bool
	mutex     sync.Mutex
	lastInfo  *SystemInfo
}

// NewMonitor creates a new memory monitor instance
func NewMonitor(cfg *config.Config) *Monitor {
	m := &Monitor{
		config:   cfg,
		stopChan: make(chan struct{}),
	}
	return m
}

// StartMonitoring begins the memory monitoring process
func (m *Monitor) StartMonitoring() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.isRunning {
		return fmt.Errorf("SysInfo monitor is already running")
	}

	interval := 1 * time.Second
	m.ticker = time.NewTicker(interval)
	m.isRunning = true

	// Run the first check immediately, then continue at intervals
	go func() {
		m.checkSysInfo()

		for {
			select {
			case <-m.ticker.C:
				m.checkSysInfo()
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
	logger.Info("SysInfo monitor stopped")
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

// GetConfig returns the monitor's configuration
func (m *Monitor) GetConfig() *config.Config {
	return m.config
}

// checkMemory performs a single memory check
func (m *Monitor) checkSysInfo() {
	info, err := GetSystemInfo()

	if err != nil {
		logger.Error("Failed to get system info",
			logger.String("error", err.Error()))
		return
	}

	// Lock before modifying shared data
	m.mutex.Lock()
	// Store the latest metrics
	m.lastInfo = info
	m.mutex.Unlock()

	// Format timestamp consistently for all messages
	timestamp := time.Now()
	formattedTime := timestamp.Format(time.RFC3339)

	// Create a completely separate metrics structure to ensure no overlap with CPU data
	// Using a deeply nested structure with explicit metric type identification to match CPU format
	combinedMsg := map[string]interface{}{
		"metric_type": "sysinfo", // Explicit identifier for the metric type
		"metrics_data": map[string]interface{}{
			"system_info": map[string]interface{}{
				"uptime":           info.Uptime,
				"current_time":     info.CurrentTime,
				"process_count":    info.ProcessCount,
				"hostname":         info.Hostname,
				"os":               info.OS,
				"platform":         info.Platform,
				"platform_version": info.PlatformVersion,
				"kernel_version":   info.KernelVersion,
				"ip_addresses":     info.IPAddresses,
			},
		},
		"meta": map[string]interface{}{
			"timestamp":        timestamp,
			"last_update_time": formattedTime,
			"source":           "sysinfo_monitor",
			"version":          "1.0",
		},
	}

	// Also broadcast to memory-specific WebSocket
	registry := websocket.GetRegistry()
	if handler := registry.GetSysHandler(); handler != nil {
		registry.BroadcastSysInfo(combinedMsg)
	}
}
