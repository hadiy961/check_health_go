package main

import (
	"CheckHealthDO/internal/pkg/logger"
	"CheckHealthDO/internal/startup"
	"CheckHealthDO/internal/utils/signal"
	"flag"
)

func main() {
	// Initialize default logger for early startup
	startup.SetupDefaultLogger()
	defer logger.Sync()

	// Parse configuration path flag
	configPath := parseFlags()
	if configPath == "" {
		configPath = "conf/config.yaml" // Default configuration path
	}

	// Initialize application with configuration
	application := startup.InitializeApplication(configPath)

	// Start HTTP server and get the builder
	builder := startup.StartServer(application)

	// Handle system signals for graceful shutdown
	signal.HandleSignals(application, builder)
}

func parseFlags() string {
	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to configuration file")
	flag.StringVar(&configPath, "c", "", "Path to configuration file (shorthand)")
	flag.Parse()
	return configPath
}
