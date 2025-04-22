package cpu

import (
	"CheckHealthDO/internal/pkg/logger"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/process"
)

// StatusLogger logs CPU status changes to both the application log and a dedicated status log file
type StatusLogger struct {
	statusLogFile string
	mutex         sync.Mutex
}

var statusLogger *StatusLogger
var once sync.Once

// GetStatusLogger returns a singleton StatusLogger instance
func GetStatusLogger() *StatusLogger {
	once.Do(func() {
		// Use logs directory from the main application
		logDir := "logs"
		os.MkdirAll(logDir, 0755)
		statusLogger = &StatusLogger{
			statusLogFile: filepath.Join(logDir, "cpu_status_changes.log"),
		}
	})
	return statusLogger
}

// LogStatusChange logs a CPU status change to both the application log and a dedicated file
func (s *StatusLogger) LogStatusChange(previous, current string, usage float64) {
	timestamp := time.Now().Format(time.RFC3339)

	// Create a more descriptive message with trend information
	var trend string
	if previous == "normal" && (current == "warning" || current == "critical") {
		trend = "↑ INCREASING"
	} else if (previous == "warning" || previous == "critical") && current == "normal" {
		trend = "↓ DECREASING"
	} else if previous == "warning" && current == "critical" {
		trend = "↑ WORSENING"
	} else if previous == "critical" && current == "warning" {
		trend = "↓ IMPROVING"
	}

	// Get system load
	loadString := "Unknown"
	if loadAvg, err := load.Avg(); err == nil {
		loadString = fmt.Sprintf("%.2f, %.2f, %.2f", loadAvg.Load1, loadAvg.Load5, loadAvg.Load15)
	}

	// Get top 3 CPU-consuming processes for context
	topProcesses := getTopCPUProcesses(3)
	topProcessesStr := strings.Join(topProcesses, ", ")

	// Create a more detailed log message
	message := fmt.Sprintf("[%s] CPU status changed: %s -> %s (Usage: %.2f%%, Trend: %s, Load: %s, Top Processes: %s)",
		timestamp, previous, current, usage, trend, loadString, topProcessesStr)

	// Log to application log with additional context
	logger.Info("CPU status transition",
		logger.String("previous", previous),
		logger.String("current", current),
		logger.Float64("usage_percent", usage),
		logger.String("timestamp", timestamp),
		logger.String("trend", trend),
		logger.String("load_avg", loadString),
		logger.String("top_processes", topProcessesStr))

	// Log to dedicated file
	s.mutex.Lock()
	defer s.mutex.Unlock()

	f, err := os.OpenFile(s.statusLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Error("Failed to open CPU status log file",
			logger.String("error", err.Error()))
		return
	}
	defer f.Close()

	if _, err := f.WriteString(message + "\n"); err != nil {
		logger.Error("Failed to write to CPU status log file",
			logger.String("error", err.Error()))
	}
}

// getTopCPUProcesses returns the top N CPU-consuming processes
func getTopCPUProcesses(n int) []string {
	processes, err := process.Processes()
	if err != nil {
		return []string{"Error getting processes"}
	}

	type procInfo struct {
		pid  int32
		name string
		cpu  float64
	}

	var procInfos []procInfo

	for _, p := range processes {
		cpuPercent, err := p.CPUPercent()
		if err != nil {
			continue
		}

		name, err := p.Name()
		if err != nil {
			name = fmt.Sprintf("PID-%d", p.Pid)
		}

		procInfos = append(procInfos, procInfo{
			pid:  p.Pid,
			name: name,
			cpu:  cpuPercent,
		})
	}

	// Sort processes by CPU usage (descending)
	for i := 0; i < len(procInfos); i++ {
		for j := i + 1; j < len(procInfos); j++ {
			if procInfos[i].cpu < procInfos[j].cpu {
				procInfos[i], procInfos[j] = procInfos[j], procInfos[i]
			}
		}
	}

	// Get top N processes
	result := make([]string, 0, n)
	count := 0
	for _, p := range procInfos {
		if count >= n {
			break
		}
		result = append(result, fmt.Sprintf("%s(%.1f%%)", p.name, p.cpu))
		count++
	}

	if len(result) == 0 {
		return []string{"No processes found"}
	}

	return result
}
