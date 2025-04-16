package app

import (
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"fmt"
)

// Application represents the main application
type Application struct {
	configPath string
	config     *config.Config
	isRunning  bool
}

// New creates a new application instance
func New(configPath string) *Application {
	return &Application{
		configPath: configPath,
		isRunning:  false,
	}
}

// Initialize loads configuration and initializes components
func (a *Application) Initialize() error {
	// Load configuration
	cfg, err := config.LoadConfig(a.configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	a.config = cfg

	// Initialize logger with loaded configuration
	if err := logger.Init(cfg); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	logger.Info("Application initialized successfully")
	a.isRunning = true
	return nil
}

// GetConfig returns the application configuration
func (a *Application) GetConfig() *config.Config {
	return a.config
}

// GetConfigPath returns the path to the configuration file
func (a *Application) GetConfigPath() string {
	return a.configPath
}

// Shutdown performs cleanup and shutdown operations
func (a *Application) Shutdown() {
	logger.Info("Shutting down application...")
	// Perform cleanup operations here

	// Ensure logs are flushed
	if err := logger.Sync(); err != nil {
		fmt.Printf("Error flushing logs: %v\n", err)
	}

	a.isRunning = false
	logger.Info("Application shutdown complete")
}
