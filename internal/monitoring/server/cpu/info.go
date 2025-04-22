package cpu

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	gopsutilCPU "github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"
)

// GetCPUInfo retrieves the current CPU information
func GetCPUInfo(warningThreshold, criticalThreshold float64) (*CPUInfo, error) {
	// Get CPU stats
	cpuStats, err := gopsutilCPU.Info()
	if err != nil {
		return nil, err
	}

	// Validate if CPU data exists
	if len(cpuStats) == 0 {
		return nil, fmt.Errorf("no CPU information available")
	}

	// Start concurrent operations
	// Get virtualization info in parallel
	virtChan := make(chan struct {
		system string
		role   string
		err    error
	})
	go func() {
		system, role, err := host.Virtualization()
		virtChan <- struct {
			system string
			role   string
			err    error
		}{system, role, err}
	}()

	// Get both total and per-core CPU usage with a single wait period
	// This reduces the total wait time from 2 seconds to 1 second
	timeoutDuration := 500 * time.Millisecond // Reduced timeout for faster results

	// Get total CPU usage percentage
	totalUsageChan := make(chan []float64)
	totalUsageErrChan := make(chan error)
	go func() {
		usage, err := gopsutilCPU.Percent(timeoutDuration, false)
		totalUsageChan <- usage
		totalUsageErrChan <- err
	}()

	// Get per-core CPU usage percentage
	perCoreUsageChan := make(chan []float64)
	perCoreUsageErrChan := make(chan error)
	go func() {
		usage, err := gopsutilCPU.Percent(timeoutDuration, true)
		perCoreUsageChan <- usage
		perCoreUsageErrChan <- err
	}()

	// Get CPU times breakdown
	cpuTimesChan := make(chan []gopsutilCPU.TimesStat)
	cpuTimesErrChan := make(chan error)
	go func() {
		times, err := gopsutilCPU.Times(false)
		cpuTimesChan <- times
		cpuTimesErrChan <- err
	}()

	// Collect results from concurrent operations
	// Get virtualization results
	virtResult := <-virtChan
	if virtResult.err != nil {
		return nil, virtResult.err
	}
	virtualizationSystem, virtualizationRole := virtResult.system, virtResult.role
	isVirtual := virtualizationRole == "guest" // If role is "guest", it's a VM

	// Get CPU usage results
	totalUsage := <-totalUsageChan
	if err := <-totalUsageErrChan; err != nil {
		return nil, err
	}

	perCoreUsage := <-perCoreUsageChan
	if err := <-perCoreUsageErrChan; err != nil {
		return nil, err
	}

	// Get CPU times results
	cpuTimes := <-cpuTimesChan
	if err := <-cpuTimesErrChan; err != nil {
		return nil, err
	}

	// Determine CPU status based on threshold
	status := "normal"
	if totalUsage[0] >= criticalThreshold {
		status = "critical"
	} else if totalUsage[0] >= warningThreshold {
		status = "warning"
	}

	// Convert CPU times to a map - ensure these are properly normalized
	cpuTimeMap := make(map[string]float64)
	if len(cpuTimes) > 0 {
		// Store raw time values, we'll calculate percentages when displaying
		cpuTimeMap["user"] = cpuTimes[0].User
		cpuTimeMap["system"] = cpuTimes[0].System
		cpuTimeMap["idle"] = cpuTimes[0].Idle
		cpuTimeMap["nice"] = cpuTimes[0].Nice
		cpuTimeMap["iowait"] = cpuTimes[0].Iowait
		cpuTimeMap["irq"] = cpuTimes[0].Irq
		cpuTimeMap["softirq"] = cpuTimes[0].Softirq
		cpuTimeMap["steal"] = cpuTimes[0].Steal
	}

	// Count unique physical CPU packages
	physicalIDs := make(map[string]bool)
	for _, cpu := range cpuStats {
		physicalIDs[cpu.PhysicalID] = true
	}
	processorCount := len(physicalIDs)

	// Get frequency information
	minFreq := 0.0
	maxFreq := 0.0
	// This is a simplified approach - actual implementation might need to read from /sys/devices
	// on Linux or use other platform-specific methods
	if cpuStats[0].Mhz > 0 {
		// If we can't get min/max, at least set max to current
		maxFreq = cpuStats[0].Mhz / 1000.0
	}

	// Use the first CPU for model, core, cache, and flags info
	return &CPUInfo{
		ModelName:  cpuStats[0].ModelName,
		Cores:      int(cpuStats[0].Cores),
		Threads:    len(cpuStats),              // Number of threads/logical processors
		ClockSpeed: cpuStats[0].Mhz / 1000.0,   // Convert MHz to GHz
		Usage:      totalUsage[0],              // Total CPU usage percentage
		CoreUsage:  perCoreUsage,               // Per-core CPU usage percentage
		CacheSize:  int(cpuStats[0].CacheSize), // CPU cache size in KB
		IsVirtual:  isVirtual,                  // Whether running on a VM
		Hypervisor: virtualizationSystem,       // Hypervisor name if running on a VM
		CPUStatus:  status,                     // CPU status based on thresholds

		// New CPU information fields
		VendorID: cpuStats[0].VendorID,      // CPU vendor (Intel, AMD, etc)
		Family:   cpuStats[0].Family,        // CPU family information
		Stepping: int(cpuStats[0].Stepping), // CPU stepping/revision

		// Additional CPU information
		PhysicalID:     cpuStats[0].PhysicalID,
		Microcode:      cpuStats[0].Microcode,
		Architecture:   runtime.GOARCH,
		MinFrequency:   minFreq,
		MaxFrequency:   maxFreq,
		CPUTimes:       cpuTimeMap,
		Temperature:    0.0, // Would need additional platform-specific code to get temperature
		ProcessorCount: processorCount,
	}, nil
}

// GetCPUCoreInfo retrieves CPU core information without usage metrics
func GetCPUCoreInfo() (*CPUInfo, error) {
	// Get CPU stats
	cpuStats, err := gopsutilCPU.Info()
	if err != nil {
		return nil, err
	}

	// Validate if CPU data exists
	if len(cpuStats) == 0 {
		return nil, fmt.Errorf("no CPU information available")
	}

	// Check if system is running on a VM
	virtualizationSystem, virtualizationRole, err := host.Virtualization()
	if err != nil {
		return nil, err
	}

	isVirtual := virtualizationRole == "guest" // If role is "guest", it's a VM

	// Count unique physical CPU packages
	physicalIDs := make(map[string]bool)
	for _, cpu := range cpuStats {
		physicalIDs[cpu.PhysicalID] = true
	}
	processorCount := len(physicalIDs)

	// Use the first CPU for model, core info
	return &CPUInfo{
		ModelName:  cpuStats[0].ModelName,
		Cores:      int(cpuStats[0].Cores),
		Threads:    len(cpuStats),        // Number of threads/logical processors
		IsVirtual:  isVirtual,            // Whether running on a VM
		Hypervisor: virtualizationSystem, // Hypervisor name if running on a VM

		// New CPU information fields
		VendorID: cpuStats[0].VendorID,      // CPU vendor (Intel, AMD, etc)
		Family:   cpuStats[0].Family,        // CPU family information
		Stepping: int(cpuStats[0].Stepping), // CPU stepping/revision

		// Additional CPU information
		PhysicalID:     cpuStats[0].PhysicalID,
		Microcode:      cpuStats[0].Microcode,
		Architecture:   runtime.GOARCH,
		ProcessorCount: processorCount,
	}, nil
}

// Helper function to detect if running in a virtualized environment
func detectVirtualization() (bool, string) {
	info, err := host.Info()
	if err != nil {
		return false, ""
	}

	// Check if virtualization system is detected
	if info.VirtualizationSystem != "" && info.VirtualizationSystem != "none" {
		return true, info.VirtualizationSystem
	}

	return false, ""
}

// Helper function to count physical processors
func getProcessorCount(cpuInfo []gopsutilCPU.InfoStat) int {
	// Create a map to track unique physical IDs
	physicalIDs := make(map[string]bool)

	for _, cpu := range cpuInfo {
		physicalIDs[cpu.PhysicalID] = true
	}

	// If physical ID information is available, return the count
	if len(physicalIDs) > 0 {
		return len(physicalIDs)
	}

	// Default to returning the number of CPU entries
	return len(cpuInfo)
}

// Helper function to get CPU temperature from thermal sensors
func getCPUTemperature() float64 {
	// This is a simplified implementation - in real systems, you might
	// need to use platform-specific approaches or additional libraries
	// like lm-sensors to get accurate temperature data.

	// For now, just returning 0 as a placeholder
	return 0
}

// FormatBytes formats bytes into a human-readable string
func FormatBytes(bytes uint64) string {
	const (
		B  = 1
		KB = B * 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
		PB = TB * 1024
	)

	unit := ""
	value := float64(bytes)

	switch {
	case bytes >= PB:
		unit = "PB"
		value = value / PB
	case bytes >= TB:
		unit = "TB"
		value = value / TB
	case bytes >= GB:
		unit = "GB"
		value = value / GB
	case bytes >= MB:
		unit = "MB"
		value = value / MB
	case bytes >= KB:
		unit = "KB"
		value = value / KB
	case bytes >= B:
		unit = "B"
	case bytes == 0:
		return "0B"
	}

	result := strconv.FormatFloat(value, 'f', 2, 64)
	result = strings.TrimSuffix(result, ".00")
	return result + unit
}
