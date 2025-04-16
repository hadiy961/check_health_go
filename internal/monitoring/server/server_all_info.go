package server

import (
	"CheckHealthDO/internal/monitoring/server/cpu"
	"CheckHealthDO/internal/monitoring/server/disk"
	"CheckHealthDO/internal/monitoring/server/memory"
	"CheckHealthDO/internal/monitoring/server/sysinfo"
	"CheckHealthDO/internal/pkg/config"
	"time"
)

// ServerMetrics represents all metrics from the server
type ServerMetrics struct {
	Timestamp  time.Time           `json:"timestamp"`
	CPU        *cpu.CPUInfo        `json:"cpu,omitempty"`
	Memory     *memory.MemoryInfo  `json:"memory,omitempty"`
	Disk       *DiskMetrics        `json:"disk,omitempty"`
	SystemInfo *sysinfo.SystemInfo `json:"system_info,omitempty"`
}

// DiskMetrics contains all disk-related metrics
type DiskMetrics struct {
	Partitions   []disk.StorageInfo `json:"partitions,omitempty"`
	TotalStorage *disk.TotalStorage `json:"total_storage,omitempty"`
}

// GetAllServerMetrics collects all server metrics
func GetServerAllInfo(cfg *config.Config) (*ServerMetrics, error) {
	metrics := &ServerMetrics{
		Timestamp: time.Now(),
	}

	// Get CPU info
	cpuInfo, err := cpu.GetCPUInfo(cfg.Monitoring.CPU.WarningThreshold, cfg.Monitoring.CPU.CriticalThreshold)
	if err == nil && cpuInfo != nil {
		metrics.CPU = cpuInfo
	}

	// Get Memory info
	memInfo, err := memory.GetMemoryInfo(cfg.Monitoring.Memory.WarningThreshold, cfg.Monitoring.Memory.CriticalThreshold)
	if err == nil && memInfo != nil {
		metrics.Memory = memInfo
	}

	// Get Disk info including monitored paths
	diskPartitions, totalStorage, err := disk.GetStorageInfo()
	if err == nil {
		metrics.Disk = &DiskMetrics{
			Partitions:   diskPartitions,
			TotalStorage: totalStorage,
		}
	}

	// Get System info
	sysInfo, err := sysinfo.GetSystemInfo()
	if err == nil && sysInfo != nil {
		metrics.SystemInfo = sysInfo
	}


	return metrics, nil
}
