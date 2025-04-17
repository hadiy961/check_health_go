package main

import (
	"CheckHealthDO/internal/pkg/logger"
	"CheckHealthDO/internal/startup"
	"CheckHealthDO/internal/utils/signal"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

func main() {
	// Initialize default logger for early startup
	startup.SetupDefaultLogger()
	defer logger.Sync()

	// Parse configuration path flag
	configPath, foreground := parseFlags()
	if configPath == "" {
		configPath = "conf/config.yaml" // Default configuration path
	}

	// Default file locations
	pidFile := "/var/run/check_health_go.pid"

	// Check if running as child process (environment variable will be set)
	isChild := os.Getenv("CHECK_HEALTH_GO_DAEMON") == "1"

	// If not in foreground mode and not already a child process, daemonize
	if !foreground && !isChild {
		daemonize(configPath, pidFile)
		return
	}

	// Initialize application with configuration
	application := startup.InitializeApplication(configPath)

	// Start HTTP server and get the builder
	builder := startup.StartServer(application)

	// Write PID to file - always done in the child process
	if !foreground || isChild {
		writePIDFile(pidFile)

		// Register cleanup function to remove PID file on exit
		signal.RegisterCleanupFunc(func() {
			// Remove PID file during graceful shutdown
			if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
				logger.Error("Failed to remove PID file during shutdown",
					logger.String("error", err.Error()),
					logger.String("file", pidFile))
			} else {
				logger.Info("Removed PID file during shutdown",
					logger.String("file", pidFile))
			}
		})
	}

	// Handle system signals for graceful shutdown
	signal.HandleSignals(application, builder)
}

func parseFlags() (string, bool) {
	var configPath string
	var foreground bool

	flag.StringVar(&configPath, "config", "", "Path to configuration file")
	flag.StringVar(&configPath, "c", "", "Path to configuration file (shorthand)")
	flag.BoolVar(&foreground, "foreground", false, "Run in foreground (not as daemon)")
	flag.BoolVar(&foreground, "f", false, "Run in foreground (shorthand)")
	flag.Parse()

	return configPath, foreground
}

// daemonize forks the process and exits the parent
func daemonize(configPath, pidFile string) {
	// Get the full path of the current executable
	executable, err := os.Executable()
	if err != nil {
		logger.Fatal("Failed to get executable path", logger.String("error", err.Error()))
	}

	// Prepare the command to run the child process
	args := make([]string, 0, len(os.Args))
	for _, arg := range os.Args[1:] {
		// Filter out any -f or --foreground flags
		if arg != "-f" && arg != "--foreground" {
			args = append(args, arg)
		}
	}

	// Pass configuration flag to child process if specified
	configFlagFound := false
	for _, arg := range args {
		if arg == "-c" || arg == "--config" || arg == "-config" {
			configFlagFound = true
			break
		}
	}

	if !configFlagFound && configPath != "" {
		args = append(args, "-config", configPath)
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

	// Write PID to file from parent process to ensure it's available immediately
	if err := os.MkdirAll(filepath.Dir(pidFile), 0755); err != nil {
		logger.Error("Failed to create directory for PID file",
			logger.String("error", err.Error()),
			logger.String("directory", filepath.Dir(pidFile)))
	}

	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644); err != nil {
		logger.Error("Failed to write PID file",
			logger.String("error", err.Error()),
			logger.String("file", pidFile))
	} else {
		logger.Info("Wrote PID to file",
			logger.Int("pid", pid),
			logger.String("file", pidFile))
	}

	// Exit the parent process
	os.Exit(0)
}

// writePIDFile writes the current process ID to the specified file
func writePIDFile(pidFile string) {
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
