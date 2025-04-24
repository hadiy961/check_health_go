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
	// Try different patterns to find MariaDB process
	patterns := []string{"mariadb", "maria", "mysqld"}

	var pid int
	var err error

	for _, pattern := range patterns {
		pid, err = findProcessByPattern(pattern)
		if err == nil && pid > 0 {
			break
		}
	}

	if err != nil || pid == 0 {
		return 0, 0, fmt.Errorf("failed to find MariaDB process: no matching process found")
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

// findProcessByPattern attempts to find a process ID using the given pattern
func findProcessByPattern(pattern string) (int, error) {
	cmd := exec.Command("pgrep", "-f", pattern)
	output, err := cmd.Output()

	if err != nil {
		// Check if it's just that no processes were found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return 0, fmt.Errorf("no processes found matching pattern '%s'", pattern)
		}
		return 0, fmt.Errorf("error running pgrep: %w", err)
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		return 0, fmt.Errorf("no processes found matching pattern '%s'", pattern)
	}

	// Get the first process ID (there might be multiple matches)
	pidStr := strings.Split(outputStr, "\n")[0]
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse process ID: %w", err)
	}

	return pid, nil
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
