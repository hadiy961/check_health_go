package mariadb

import (
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
)

// NewDefaultDBConfig returns a default configuration for connecting to MariaDB
func NewDefaultDBConfig() *DBConfig {
	return &DBConfig{
		Host:     "localhost",
		Port:     3306,
		Username: "root",
		Password: "",
		Database: "information_schema",
	}
}

// GetDBConfigFromConfig creates a DBConfig from the application configuration
func GetDBConfigFromConfig(cfg *config.Config) *DBConfig {
	dbConfig := &DBConfig{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		Username: cfg.Database.Username,
		Password: cfg.Database.Password,
		Database: cfg.Database.Database,
	}

	// Ensure we have default values for critical connection parameters
	// to prevent empty username/host issues
	if dbConfig.Host == "" {
		dbConfig.Host = "localhost"
	}
	if dbConfig.Port == 0 {
		dbConfig.Port = 3306
	}
	if dbConfig.Username == "" {
		dbConfig.Username = "root"
	}
	if dbConfig.Database == "" {
		dbConfig.Database = "information_schema"
	}

	// Add a warning if the password is empty - this might cause connection issues
	if dbConfig.Password == "" {
		logger.Warn("MariaDB password is empty in configuration",
			logger.String("username", dbConfig.Username),
			logger.String("host", dbConfig.Host),
			logger.String("tip", "Check your config.yaml to ensure Database.Password is set properly"))
	}

	return dbConfig
}
