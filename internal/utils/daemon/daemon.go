package daemon

import (
	"CheckHealthDO/internal/pkg/logger"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// IsRunning checks if the service is already running
func IsRunning(pidFile string) bool {
	// Check if PID file exists
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		return false
	}

	// Read PID from file
	data, err := ioutil.ReadFile(pidFile)
	if err != nil {
		logger.Error("Failed to read PID file",
			logger.String("error", err.Error()),
			logger.String("file", pidFile))
		return false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		logger.Error("Invalid PID in file",
			logger.String("error", err.Error()),
			logger.String("file", pidFile))
		return false
	}

	// Check if process with PID exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix systems, FindProcess always succeeds, so we need to send
	// a signal 0 to check if the process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// Daemonize forks the process and exits the parent
func Daemonize(configPath, pidFile string) {
	// Get the full path of the current executable
	executable, err := os.Executable()
	if err != nil {
		logger.Fatal("Failed to get executable path", logger.String("error", err.Error()))
	}

	// Prepare the command to run the child process
	args := []string{"start"}

	// Add config flag if needed
	if configPath != "" {
		args = append(args, "--config", configPath)
	}

	cmd := exec.Command(executable, args...)

	// Set environment variable to indicate this is a child process
	env := os.Environ()
	cmd.Env = append(env, "CHECK_HEALTH_GO_DAEMON=1")

	// Detach process from terminal
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	// Start the detached process
	if err := cmd.Start(); err != nil {
		logger.Fatal("Failed to start daemon process", logger.String("error", err.Error()))
	}

	// Log the PID of the daemon process
	pid := cmd.Process.Pid
	logger.Info("Started daemon process", logger.Int("pid", pid))

	// Exit the parent process
	os.Exit(0)
}

// WritePIDFile writes the current process ID to the specified file
func WritePIDFile(pidFile string) {
	pid := os.Getpid()

	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(pidFile), 0755); err != nil {
		logger.Error("Failed to create directory for PID file",
			logger.String("error", err.Error()),
			logger.String("directory", filepath.Dir(pidFile)))
		return
	}

	// Write PID to file
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		logger.Error("Failed to write PID file",
			logger.String("error", err.Error()),
			logger.String("file", pidFile))
		return
	}

	logger.Info("Wrote PID to file",
		logger.Int("pid", pid),
		logger.String("file", pidFile))
}

// RemovePIDFile removes the PID file during shutdown
func RemovePIDFile(pidFile string) {
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		logger.Error("Failed to remove PID file during shutdown",
			logger.String("error", err.Error()),
			logger.String("file", pidFile))
	} else {
		logger.Info("Removed PID file during shutdown",
			logger.String("file", pidFile))
	}
}

// StopProcess stops the running process
func StopProcess(pidFile string) (int, error) {
	// Check if PID file exists
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		return 0, fmt.Errorf("service is not running (PID file not found)")
	}

	// Read PID from file
	data, err := ioutil.ReadFile(pidFile)
	if err != nil {
		return 0, fmt.Errorf("failed to read PID file: %w", err)
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file: %w", err)
	}

	// Find the process
	process, err := os.FindProcess(pid)
	if err != nil {
		return 0, fmt.Errorf("failed to find process: %w", err)
	}

	// Send SIGTERM signal
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return 0, fmt.Errorf("failed to send terminate signal: %w", err)
	}

	// Remove PID file
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		logger.Warn("Failed to remove PID file after stopping process",
			logger.String("error", err.Error()),
			logger.String("file", pidFile))
	}

	return pid, nil
}

// GetStatus checks if the service is running and returns the PID
func GetStatus(pidFile string) (bool, int) {
	// Check if PID file exists
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		return false, 0
	}

	// Read PID from file
	data, err := ioutil.ReadFile(pidFile)
	if err != nil {
		logger.Error("Failed to read PID file",
			logger.String("error", err.Error()),
			logger.String("file", pidFile))
		return false, 0
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		logger.Error("Invalid PID in file",
			logger.String("error", err.Error()),
			logger.String("file", pidFile))
		return false, 0
	}

	// Check if process with PID exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, 0
	}

	// On Unix systems, FindProcess always succeeds, so we need to send
	// a signal 0 to check if the process exists
	err = process.Signal(syscall.Signal(0))
	if err == nil {
		return true, pid
	}

	// Process does not exist, try to clean up the stale PID file
	os.Remove(pidFile)
	return false, 0
}
