package memory

import (
	"CheckHealthDO/internal/pkg/logger"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/shirou/gopsutil/mem"
)

// StatusLogger logs memory status changes to both the application log and a dedicated status log file
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
			statusLogFile: filepath.Join(logDir, "memory_status_changes.log"),
		}
	})
	return statusLogger
}

// Enhance the LogStatusChange method to provide more informative logging
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

	// Create a more detailed log message
	message := fmt.Sprintf("[%s] Memory status changed: %s -> %s (Usage: %.2f%%, Trend: %s)",
		timestamp, previous, current, usage, trend)

	// Log to application log with additional context
	logger.Info("Memory status transition",
		logger.String("previous", previous),
		logger.String("current", current),
		logger.Float64("usage_percent", usage),
		logger.String("timestamp", timestamp),
		logger.String("trend", trend),
		logger.String("available_memory", FormatBytes(GetAvailableMemory())))

	// Log to dedicated file
	s.mutex.Lock()
	defer s.mutex.Unlock()

	f, err := os.OpenFile(s.statusLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Error("Failed to open memory status log file",
			logger.String("error", err.Error()))
		return
	}
	defer f.Close()

	if _, err := f.WriteString(message + "\n"); err != nil {
		logger.Error("Failed to write to memory status log file",
			logger.String("error", err.Error()))
	}
}

// Add a helper function to get available memory
func GetAvailableMemory() uint64 {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return 0
	}
	return vm.Available
}
