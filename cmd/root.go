package cmd

import (
	"fmt"
	"os"

	"CheckHealthDO/internal/startup"

	"github.com/spf13/cobra"
)

var (
	configPath string
	pidFile    = "/var/run/check_health_go.pid"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "check_health_go",
	Short: "A health monitoring service for DO",
	Long: `CheckHealthDO is a comprehensive health monitoring service that allows 
administrators to monitor and manage system resources.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Initialize default logger for early startup
	startup.SetupDefaultLogger()

	// Define flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "conf/config.yaml", "Path to configuration file")
}
