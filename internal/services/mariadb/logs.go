package mariadb

import (
	"CheckHealthDO/internal/pkg/logger"
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GetLatestMariaDBLogs retrieves the most recent log entries from the MariaDB error log
func GetLatestMariaDBLogs(logPath string, maxEntries int) ([]string, error) {
	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		logger.Warn("MariaDB log file not found, checking alternative locations",
			logger.String("path", logPath))

		// Try common alternative log locations
		alternativePaths := []string{
			"/var/log/mysql/error.log",
			"/var/log/mariadb/mariadb.log",
			"/var/log/mariadb/error.log",
			"/var/log/mysql/mariadb.log",
			"/var/log/mysql.log",
			"/var/lib/mysql/mysql.log",
		}

		for _, altPath := range alternativePaths {
			if altPath != logPath {
				if _, err := os.Stat(altPath); err == nil {
					logger.Info("Using alternative MariaDB log path",
						logger.String("original_path", logPath),
						logger.String("alternative_path", altPath))
					logPath = altPath
					break
				}
			}
		}

		// If no log file is found, return a fallback message instead of an error
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			return []string{"Log file not found. This is not critical for basic monitoring."}, nil
		}
	}

	// Open the log file
	file, err := os.Open(logPath)
	if err != nil {
		logger.Error("Failed to open MariaDB log file",
			logger.String("path", logPath),
			logger.String("error", err.Error()))
		return nil, err
	}
	defer file.Close()

	// Use a circular buffer to keep the last maxEntries lines
	logs := make([]string, 0, maxEntries)
	scanner := bufio.NewScanner(file)

	// For each line in the file
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines or those that don't contain useful information
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "--") {
			continue
		}

		// Add the line to our logs
		logs = append(logs, line)

		// If we've exceeded maxEntries, remove the oldest
		if len(logs) > maxEntries {
			logs = logs[1:]
		}
	}

	if err := scanner.Err(); err != nil {
		logger.Error("Error reading MariaDB log file",
			logger.String("path", logPath),
			logger.String("error", err.Error()))
		return logs, err // Still return what we've read
	}

	return logs, nil
}

// Add new function to get system logs if MariaDB logs aren't available
func GetSystemdServiceLogs(serviceName string, maxEntries int) ([]string, error) {
	// Try journalctl for systemd logs
	cmd := exec.Command("journalctl", "-u", serviceName, "--no-pager", "-n", fmt.Sprintf("%d", maxEntries))
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	logs := []string{}
	for _, line := range strings.Split(string(output), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			logs = append(logs, line)
		}
	}

	// If we got systemd logs, return them
	if len(logs) > 0 {
		return logs, nil
	}

	// Try traditional syslog if systemd logs aren't available
	cmd = exec.Command("grep", serviceName, "/var/log/syslog")
	output, err = cmd.Output()
	if err != nil && err.Error() != "exit status 1" { // grep returns 1 if no matches
		return nil, err
	}

	logs = []string{}
	lines := strings.Split(string(output), "\n")
	// Get last maxEntries lines (or all if fewer)
	startIndex := len(lines) - maxEntries
	if startIndex < 0 {
		startIndex = 0
	}

	for _, line := range lines[startIndex:] {
		if line = strings.TrimSpace(line); line != "" {
			logs = append(logs, line)
		}
	}

	return logs, nil
}

// AnalyzeMariaDBLogs examines log entries and returns possible diagnoses
func AnalyzeMariaDBLogs(logs []string) []string {
	diagnoses := make([]string, 0)

	// Check for common error patterns
	for _, log := range logs {
		log = strings.ToLower(log)

		// Memory issues
		if strings.Contains(log, "out of memory") || strings.Contains(log, "memory allocation") {
			diagnoses = append(diagnoses, "Memory allocation issues detected - consider increasing available memory")
		}

		// Disk space issues
		if strings.Contains(log, "disk full") || strings.Contains(log, "no space left") ||
			strings.Contains(log, "can't create/write to file") {
			diagnoses = append(diagnoses, "Disk space issues detected - free up disk space or check filesystem permissions")
		}

		// Permission problems
		if strings.Contains(log, "permission denied") || strings.Contains(log, "access denied") {
			diagnoses = append(diagnoses, "Permission problems detected - check file and directory permissions")
		}

		// Corruption issues
		if strings.Contains(log, "corrupt") || strings.Contains(log, "crashed") {
			diagnoses = append(diagnoses, "Database corruption may have occurred - consider running repair tools")
		}

		// Connection problems
		if strings.Contains(log, "connection refused") || strings.Contains(log, "could not connect") {
			diagnoses = append(diagnoses, "Connection issues detected - check network configuration")
		}

		// Too many connections
		if strings.Contains(log, "too many connections") {
			diagnoses = append(diagnoses, "Connection limit reached - consider increasing max_connections")
		}
	}

	// If no specific issues found
	if len(diagnoses) == 0 {
		diagnoses = append(diagnoses, "No specific issues identified in the logs")
	}

	return diagnoses
}
