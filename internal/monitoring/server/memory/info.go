package memory

import (
	"github.com/shirou/gopsutil/mem"
)

// GetMemoryInfo retrieves the current memory information
func GetMemoryInfo(warningThreshold, criticalThreshold float64) (*MemoryInfo, error) {
	// Ambil statistik memory menggunakan gopsutil
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	// Hitung persentase memory yang digunakan dan tersedia
	usedPercentage := vmStat.UsedPercent
	freePercentage := 100 - usedPercentage

	// Tentukan status memory berdasarkan threshold
	status := "normal"
	if usedPercentage >= criticalThreshold {
		status = "critical"
	} else if usedPercentage >= warningThreshold {
		status = "warning"
	}

	memInfo := &MemoryInfo{
		TotalMemory:          vmStat.Total,
		UsedMemory:           vmStat.Used,
		FreeMemory:           vmStat.Free,
		AvailableMemory:      vmStat.Available,
		UsedMemoryPercentage: usedPercentage,
		FreeMemoryPercentage: freePercentage,
		MemoryStatus:         status,

		// Add additional memory metrics
		CachedMemory:   vmStat.Cached,
		BufferMemory:   vmStat.Buffers,
		ActiveMemory:   vmStat.Active,
		InactiveMemory: vmStat.Inactive,
		SharedMemory:   vmStat.Shared,
	}

	// Try to get swap information
	swapStat, err := mem.SwapMemory()
	if err == nil && swapStat != nil {
		memInfo.SwapTotal = swapStat.Total
		memInfo.SwapUsed = swapStat.Used
		memInfo.SwapFree = swapStat.Free

		// Calculate swap usage percentage
		if swapStat.Total > 0 {
			memInfo.SwapUsedPercentage = float64(swapStat.Used) / float64(swapStat.Total) * 100
		}
	}

	return memInfo, nil
}
