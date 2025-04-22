package cpu

import (
	"CheckHealthDO/internal/pkg/logger"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
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
	message := fmt.Sprintf("[%s] CPU status changed: %s -> %s (Usage: %.2f%%)",
		timestamp, previous, current, usage)

	// Log to application log
	logger.Info("CPU status transition",
		logger.String("previous", previous),
		logger.String("current", current),
		logger.Float64("usage_percent", usage),
		logger.String("timestamp", timestamp))

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
