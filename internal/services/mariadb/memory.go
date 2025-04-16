package mariadb

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/process"
)

// GetMariaDBMemoryUsage retrieves memory usage for the MariaDB process
func GetMariaDBMemoryUsage() (uint64, float64, error) {
	// First, get the MariaDB process ID using pgrep
	cmd := exec.Command("pgrep", "-f", "mysqld")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to find MariaDB process: %w", err)
	}

	// Get the first process ID (there might be multiple matches)
	pidStr := strings.TrimSpace(strings.Split(string(output), "\n")[0])
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse MariaDB process ID: %w", err)
	}

	// Use gopsutil to get memory info for this process
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get process info: %w", err)
	}

	memInfo, err := proc.MemoryInfo()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get memory info: %w", err)
	}

	// Get total system memory to calculate percentage
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return memInfo.RSS, 0, fmt.Errorf("failed to get system memory info: %w", err)
	}

	// Calculate the percentage of total memory used by MariaDB
	percentUsed := float64(memInfo.RSS) / float64(vmStat.Total) * 100.0

	return memInfo.RSS, percentUsed, nil
}

// FormatBytes converts bytes to a human-readable string
func FormatBytes(bytes uint64) string {
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
