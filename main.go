package main

import (
	"CheckHealthDO/cmd"
	"CheckHealthDO/internal/pkg/logger"
)

func main() {
	// Execute the root command
	cmd.Execute()

	// Ensure logs are flushed on exit
	defer logger.Sync()
}
