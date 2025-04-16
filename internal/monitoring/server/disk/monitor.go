package disk

import (
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"CheckHealthDO/internal/websocket"
	"context"
	"fmt"
	"sync"
	"time"
)

// Monitor handles periodic storage monitoring
type Monitor struct {
	config    *config.Config
	ticker    *time.Ticker
	stopChan  chan struct{}
	isRunning bool
	mutex     sync.Mutex
	lastInfo  []StorageInfo // Changed from *StorageInfo to []StorageInfo
}

// NewMonitor creates a new storage monitor instance
func NewMonitor(cfg *config.Config) *Monitor {
	m := &Monitor{
		config:   cfg,
		stopChan: make(chan struct{}),
	}
	return m
}

// StartMonitoring begins the storage monitoring process
func (m *Monitor) StartMonitoring() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.isRunning {
		return fmt.Errorf("disk monitor is already running")
	}

	// Check if disk monitoring is enabled
	if !m.config.Monitoring.Disk.Enabled {
		return fmt.Errorf("disk monitoring is disabled in configuration")
	}

	interval := time.Duration(m.config.Monitoring.Disk.CheckInterval) * time.Second
	m.ticker = time.NewTicker(interval)
	m.isRunning = true

	logger.Info("Starting disk monitor",
		logger.Int("interval_seconds", m.config.Monitoring.Disk.CheckInterval),
		logger.Float64("warning_threshold", m.config.Monitoring.Disk.WarningThreshold),
		logger.Float64("critical_threshold", m.config.Monitoring.Disk.CriticalThreshold))

	// Run the first check immediately, then continue at intervals
	go func() {
		m.checkStorageInfo()

		for {
			select {
			case <-m.ticker.C:
				m.checkStorageInfo()
			case <-m.stopChan:
				m.ticker.Stop()
				return
			}
		}
	}()

	return nil
}

// StopMonitoring halts the storage monitoring process
func (m *Monitor) StopMonitoring() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isRunning {
		return
	}

	close(m.stopChan)
	m.isRunning = false
	logger.Info("Disk monitor stopped")
}

// StartBackgroundMonitor creates and starts a Disk monitor in a background goroutine
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

// formatBytes converts bytes to a human-readable string
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// checkStorageInfo performs a single storage check
func (m *Monitor) checkStorageInfo() {
	// Get storage information with monitored paths analysis
	infoSlice, totalStorage, err := GetStorageInfo()
	if err != nil {
		logger.Error("Failed to get storage info",
			logger.String("error", err.Error()))
		return
	}

	// Lock before modifying shared data
	m.mutex.Lock()
	// Store the latest metrics
	m.lastInfo = infoSlice
	m.mutex.Unlock()

	// Format timestamp consistently for all messages
	timestamp := time.Now()
	formattedTime := timestamp.Format(time.RFC3339)

	// Create a slice to hold disk information
	disksInfo := make([]map[string]interface{}, len(infoSlice))

	// Process each disk
	for i, diskInfo := range infoSlice {
		// Determine status for this specific disk
		diskStatus := determineDiskStatus(diskInfo.Usage, m.config)

		disksInfo[i] = map[string]interface{}{
			"device":             diskInfo.Device,
			"mountpoint":         diskInfo.MountPoint,
			"fstype":             diskInfo.FileSystem,
			"total_space":        diskInfo.Total,
			"free_space":         diskInfo.Free,
			"used_space":         diskInfo.Used,
			"used_space_percent": diskInfo.Usage,
			"is_external":        diskInfo.IsExternal,
			"status":             diskStatus,
		}
	}

	// Create a metrics structure with storage capacity information
	combinedMsg := map[string]interface{}{
		"metric_type": "storage", // Explicit identifier for the metric type
		"metrics_data": map[string]interface{}{
			"disks": disksInfo,
			"total_storage": map[string]interface{}{
				"capacity":           totalStorage.TotalCapacity,
				"formatted_capacity": formatBytes(totalStorage.TotalCapacity),
				"used":               totalStorage.TotalUsed,
				"formatted_used":     formatBytes(totalStorage.TotalUsed),
				"free":               totalStorage.TotalFree,
				"formatted_free":     formatBytes(totalStorage.TotalFree),
				"used_percent":       totalStorage.UsagePercent,
				"status":             determineStatus(totalStorage.UsagePercent, m.config),
				"threshold": map[string]interface{}{
					"warning":  m.config.Monitoring.Disk.WarningThreshold,
					"critical": m.config.Monitoring.Disk.CriticalThreshold,
				},
				"internal_storage": map[string]interface{}{
					"capacity":           totalStorage.TotalCapacityInternal,
					"formatted_capacity": formatBytes(totalStorage.TotalCapacityInternal),
					"used":               totalStorage.TotalUsedInternal,
					"formatted_used":     formatBytes(totalStorage.TotalUsedInternal),
					"free":               totalStorage.TotalFreeInternal,
					"formatted_free":     formatBytes(totalStorage.TotalFreeInternal),
					"used_percent":       totalStorage.TotalUsagePercentInternal,
					"device_count":       totalStorage.TotalDeviceInternal,
					"status":             determineStatus(totalStorage.TotalUsagePercentInternal, m.config),
				},
				"external_storage": map[string]interface{}{
					"capacity":           totalStorage.TotalCapacityExternal,
					"formatted_capacity": formatBytes(totalStorage.TotalCapacityExternal),
					"used":               totalStorage.TotalUsedExternal,
					"formatted_used":     formatBytes(totalStorage.TotalUsedExternal),
					"free":               totalStorage.TotalFreeExternal,
					"formatted_free":     formatBytes(totalStorage.TotalFreeExternal),
					"used_percent":       totalStorage.TotalUsagePercentExternal,
					"device_count":       totalStorage.TotalDeviceExternal,
					"status":             determineStatus(totalStorage.TotalUsagePercentExternal, m.config),
				},
			},
		},
		"meta": map[string]interface{}{
			"timestamp":        timestamp,
			"last_update_time": formattedTime,
			"source":           "storage_monitor",
			"version":          "1.0",
		},
	}

	// Broadcast to disk-specific WebSocket
	registry := websocket.GetRegistry()
	if handler := registry.GetDiskHandler(); handler != nil {
		registry.BroadcastDisk(combinedMsg)
	}
}

// determineDiskStatus determines the status of a disk based on usage percentage
func determineDiskStatus(usagePercent float64, cfg *config.Config) string {
	if usagePercent >= cfg.Monitoring.Disk.CriticalThreshold {
		return "critical"
	} else if usagePercent >= cfg.Monitoring.Disk.WarningThreshold {
		return "warning"
	}
	return "normal"
}

// determineStatus determines storage status based on usage percentage
func determineStatus(usagePercent float64, cfg *config.Config) string {
	if usagePercent >= cfg.Monitoring.Disk.CriticalThreshold {
		return "critical"
	} else if usagePercent >= cfg.Monitoring.Disk.WarningThreshold {
		return "warning"
	}
	return "normal"
}
