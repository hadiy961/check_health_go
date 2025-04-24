package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the main application configuration
type Config struct {
	AppName       string              `yaml:"app_name"`
	Server        ServerConfig        `yaml:"server"`
	Database      DatabaseConfig      `yaml:"database"` // Add database config
	Agent         AgentConfig         `yaml:"agent"`    // Add agent config
	Monitoring    MonitoringConfig    `yaml:"monitoring"`
	Notifications NotificationsConfig `yaml:"notifications"`
	Logs          LogsConfig          `yaml:"logs"`
	API           API                 `yaml:"api"` // Add API config
}

// ServerConfig holds server related configuration
type ServerConfig struct {
	Port           int    `yaml:"port"`
	Host           string `yaml:"host"`
	WebDir         string `yaml:"web_dir"`
	ReadTimeout    int    `yaml:"read_timeout"`
	WriteTimeout   int    `yaml:"write_timeout"`
	IdleTimeout    int    `yaml:"idle_timeout"`
	MaxHeaderBytes int    `yaml:"max_header_bytes"`
}

// AgentConfig holds the agent related configuration
type AgentConfig struct {
	Auth AuthConfig `yaml:"auth"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	User string `yaml:"user"`
	Pass string `yaml:"pass"`
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

// MariaDBMonitoringConfig contains configuration for MariaDB monitoring
type MariaDBMonitoringConfig struct {
	Enabled            bool   `yaml:"enabled"`
	ServiceName        string `yaml:"service_name"`
	CheckInterval      int    `yaml:"check_interval"`
	LogPath            string `yaml:"log_path"`
	AutoRestart        bool   `yaml:"auto_restart"`
	RestartOnThreshold struct {
		Enabled   bool   `yaml:"enabled"`
		Threshold string `yaml:"threshold"`
	} `yaml:"restart_on_threshold"`
}

// MemoryMonitoringConfig holds memory monitoring configuration
type MemoryMonitoringConfig struct {
	Enabled           bool    `yaml:"enabled"`
	WarningThreshold  float64 `yaml:"warning_threshold"`
	CriticalThreshold float64 `yaml:"critical_threshold"`
	CheckInterval     int     `yaml:"check_interval"`
}

// CPUMonitoringConfig holds CPU monitoring configuration
type CPUMonitoringConfig struct {
	Enabled           bool    `yaml:"enabled"`
	WarningThreshold  float64 `yaml:"warning_threshold"`
	CriticalThreshold float64 `yaml:"critical_threshold"`
	CheckInterval     int     `yaml:"check_interval"`
}

// DiskMonitoringConfig holds Disk monitoring configuration
type DiskMonitoringConfig struct {
	Enabled           bool     `yaml:"enabled"`
	WarningThreshold  float64  `yaml:"warning_threshold"`
	CriticalThreshold float64  `yaml:"critical_threshold"`
	CheckInterval     int      `yaml:"check_interval"`
	MonitoredPath     []string `yaml:"monitored_paths"`
}

// MonitoringConfig contains configuration for monitoring
type MonitoringConfig struct {
	Memory  MemoryMonitoringConfig  `yaml:"memory"`
	CPU     CPUMonitoringConfig     `yaml:"cpu"` // Add CPU monitoring config
	MariaDB MariaDBMonitoringConfig `yaml:"mariadb"`
	Disk    DiskMonitoringConfig    `yaml:"disk"`
}

// NotificationsConfig holds notification related configuration
type NotificationsConfig struct {
	Throttling ThrottlingConfig `yaml:"throttling"`
	Email      EmailConfig      `yaml:"email"`
}

// ThrottlingConfig holds throttling configuration for notifications
type ThrottlingConfig struct {
	Enabled           bool `yaml:"enabled"`
	CooldownPeriod    int  `yaml:"cooldown_period"`
	MaxWarningsPerDay int  `yaml:"max_warnings_per_day"`
	AggregationPeriod int  `yaml:"aggregation_period"`
	CriticalThreshold int  `yaml:"critical_threshold"`
}

// EmailConfig holds email notification configuration
type EmailConfig struct {
	Enabled         bool          `yaml:"enabled"`
	SMTPServer      string        `yaml:"smtp_server"`
	SMTPPort        int           `yaml:"smtp_port"`
	UseTLS          bool          `yaml:"use_tls"`
	UseSSL          bool          `yaml:"use_ssl"`
	Timeout         int           `yaml:"timeout"`
	SenderEmails    []SenderEmail `yaml:"sender_emails"`
	RecipientEmails []string      `yaml:"recipient_emails"`
	RetryCount      int           `yaml:"retry_count"`
	RetryInterval   int           `yaml:"retry_interval"`
	TemplateDir     string        `yaml:"template_dir"`
}

// SenderEmail represents an email sender with credentials
type SenderEmail struct {
	Email    string `yaml:"email"`
	Password string `yaml:"password"`
	RealName string `yaml:"real_name"` // Added real name field for display name in emails
}

// LogsConfig holds logging configuration
type LogsConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Level    string `yaml:"level"`
	FilePath string `yaml:"file_path"`
	Format   string `yaml:"format"`
	Stdout   bool   `yaml:"stdout"`
}

// LoadConfig loads the configuration from the specified file path
func LoadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// SaveConfig saves the configuration to the specified file path
func SaveConfig(cfg *Config, filePath string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetDefaultConfig returns the default configuration
func GetDefaultConfig() *Config {
	return &Config{
		AppName: "CheckHealthDO",
		Server: ServerConfig{
			Port:   8080,
			Host:   "0.0.0.0",
			WebDir: "./web", // Add default web directory path
		},
		Agent: AgentConfig{
			Auth: AuthConfig{
				User: "dbaDO",
				Pass: "dbaDO123!@#",
			},
		},
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     3306,
			Username: "root",
			Password: "",
			Database: "information_schema",
		},
		Monitoring: MonitoringConfig{
			Memory: MemoryMonitoringConfig{
				Enabled:           true,
				WarningThreshold:  80.0,
				CriticalThreshold: 90.0,
				CheckInterval:     1,
			},
			CPU: CPUMonitoringConfig{
				Enabled:           true,
				WarningThreshold:  70.0,
				CriticalThreshold: 90.0,
				CheckInterval:     1,
			},
			MariaDB: MariaDBMonitoringConfig{
				Enabled:     true,
				ServiceName: "mariadb",
				RestartOnThreshold: struct {
					Enabled   bool   `yaml:"enabled"`
					Threshold string `yaml:"threshold"`
				}{
					Enabled:   true,
					Threshold: "critical",
				},
				CheckInterval: 1,
				LogPath:       "/var/log/mysql/error.log",
				AutoRestart:   true,
			},
		},
		Notifications: NotificationsConfig{
			Throttling: ThrottlingConfig{
				Enabled:        true,
				CooldownPeriod: 300,
			},
			Email: EmailConfig{
				Enabled:    true,
				SMTPServer: "mail.dataon.com",
				SMTPPort:   587,
				UseTLS:     true,
				UseSSL:     false,
				Timeout:    10,
				SenderEmails: []SenderEmail{
					{
						Email:    "hadiyatna.muflihun@dataon.com",
						Password: "HadiyatnaMuflihun24!@#",
					},
				},
				RecipientEmails: []string{"hadiyatna.muflihun@dataon.com"},
				RetryCount:      3,
				RetryInterval:   5,
				TemplateDir:     "templates/email",
			},
		},
		Logs: LogsConfig{
			Enabled:  true,
			Level:    "debug",
			FilePath: "logs",
			Format:   "json",
			Stdout:   true,
		},
	}
}
