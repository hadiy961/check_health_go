package finder

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindConfigFile looks for a configuration file in the given path and returns the absolute path
func FindConfigFile(configPath string, mustExist bool) (string, error) {
	// Check if file exists
	if _, err := os.Stat(configPath); err == nil {
		absPath, err := filepath.Abs(configPath)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path: %w", err)
		}
		return absPath, nil
	} else if mustExist {
		return "", fmt.Errorf("configuration file not found: %s", configPath)
	}

	return configPath, nil
}
