package mariadb

import (
	"CheckHealthDO/internal/pkg/config"
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
	return &DBConfig{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		Username: cfg.Database.Username,
		Password: cfg.Database.Password,
		Database: cfg.Database.Database,
	}
}
