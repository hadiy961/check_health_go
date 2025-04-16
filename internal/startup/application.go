package startup

import (
	"CheckHealthDO/internal/app"
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"CheckHealthDO/internal/utils/finder"
	"os"
)

// InitializeApplication initializes the application with the given config path
func InitializeApplication(configPath string) *app.Application {
	// Find configuration file
	foundConfigPath, err := finder.FindConfigFile(configPath, true)
	if err != nil {
		logger.Error("Failed to find configuration", logger.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("Using configuration file", logger.String("path", foundConfigPath))

	// Create and initialize application
	application := app.New(foundConfigPath)
	if err := application.Initialize(); err != nil {
		logger.Error("Failed to initialize application", logger.String("error", err.Error()))
		os.Exit(1)
	}

	return application
}

// SetupDefaultLogger initializes a default logger for early startup
func SetupDefaultLogger() {
	if err := logger.Init(config.GetDefaultConfig()); err != nil {
		// Can't use logger yet, so use fmt
		panic("Error initializing logger: " + err.Error())
	}
}
