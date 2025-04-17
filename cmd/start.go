package cmd

import (
	"CheckHealthDO/internal/startup"
	"CheckHealthDO/internal/utils/daemon"
	"CheckHealthDO/internal/utils/signal"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	foreground bool
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the CheckHealth service",
	Long:  `Start the CheckHealth monitoring service in foreground or as a daemon.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Check if the service is already running
		if daemon.IsRunning(pidFile) {
			fmt.Printf("CheckHealth service is already running (PID file exists at %s)\n", pidFile)
			os.Exit(1)
		}

		// Check if running as child process (environment variable will be set)
		isChild := os.Getenv("CHECK_HEALTH_GO_DAEMON") == "1"

		// If not in foreground mode and not already a child process, daemonize
		if !foreground && !isChild {
			daemon.Daemonize(configPath, pidFile)
			return
		}

		// Initialize application with configuration
		application := startup.InitializeApplication(configPath)

		// Start HTTP server and get the builder
		builder := startup.StartServer(application)

		// Write PID to file - always done in the child process
		if !foreground || isChild {
			daemon.WritePIDFile(pidFile)

			// Register cleanup function to remove PID file on exit
			signal.RegisterCleanupFunc(func() {
				daemon.RemovePIDFile(pidFile)
			})
		}

		// Handle system signals for graceful shutdown
		signal.HandleSignals(application, builder)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().BoolVarP(&foreground, "foreground", "f", false, "Run in foreground (not as daemon)")
}
