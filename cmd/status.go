package cmd

import (
	"CheckHealthDO/internal/utils/daemon"
	"fmt"

	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of the CheckHealth service",
	Long:  `Check if the CheckHealth monitoring service is currently running.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Check if the service is running
		running, pid := daemon.GetStatus(pidFile)
		if running {
			fmt.Printf("CheckHealth service is running (PID: %d)\n", pid)
		} else {
			fmt.Println("CheckHealth service is not running")
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
