package mariadb

import (
	"database/sql"
	"fmt"
	"time"

	// Import MySQL driver
	_ "github.com/go-sql-driver/mysql"
)

// GetUptime returns the uptime of MariaDB in seconds
func GetUptime(dbConfig *DBConfig) (int64, error) {
	// Connect to MariaDB
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		dbConfig.Username, dbConfig.Password, dbConfig.Host, dbConfig.Port, dbConfig.Database)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to MariaDB: %w", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		return 0, fmt.Errorf("failed to ping MariaDB: %w", err)
	}

	// Query for uptime
	var uptime int64
	err = db.QueryRow("SELECT VARIABLE_VALUE FROM information_schema.GLOBAL_STATUS WHERE VARIABLE_NAME = 'Uptime'").Scan(&uptime)
	if err != nil {
		return 0, fmt.Errorf("failed to query MariaDB uptime: %w", err)
	}

	return uptime, nil
}

// GetVersion returns the version of MariaDB
func GetVersion(dbConfig *DBConfig) (string, error) {
	// Connect to MariaDB
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		dbConfig.Username, dbConfig.Password, dbConfig.Host, dbConfig.Port, dbConfig.Database)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return "", fmt.Errorf("failed to connect to MariaDB: %w", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		return "", fmt.Errorf("failed to ping MariaDB: %w", err)
	}

	// Query for version
	var version string
	err = db.QueryRow("SELECT VERSION()").Scan(&version)
	if err != nil {
		return "", fmt.Errorf("failed to query MariaDB version: %w", err)
	}

	return version, nil
}

// GetActiveConnections returns the number of active connections
func GetActiveConnections(dbConfig *DBConfig) (int, error) {
	// Connect to MariaDB
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		dbConfig.Username, dbConfig.Password, dbConfig.Host, dbConfig.Port, dbConfig.Database)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to MariaDB: %w", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		return 0, fmt.Errorf("failed to ping MariaDB: %w", err)
	}

	// Query for active connections
	var connections int
	err = db.QueryRow("SELECT COUNT(*) FROM information_schema.PROCESSLIST").Scan(&connections)
	if err != nil {
		return 0, fmt.Errorf("failed to query MariaDB connections: %w", err)
	}

	return connections, nil
}

// FormatUptime converts seconds to a human-readable string
func FormatUptime(seconds int64) string {
	duration := time.Duration(seconds) * time.Second
	days := int(duration.Hours() / 24)
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60

	return fmt.Sprintf("%d days, %d hours, %d minutes", days, hours, minutes)
}
