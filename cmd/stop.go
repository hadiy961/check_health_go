package cmd

import (
	"CheckHealthDO/internal/utils/daemon"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the CheckHealth service",
	Long:  `Stop the running CheckHealth monitoring service.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Try to stop the service
		pid, err := daemon.StopProcess(pidFile)
		if err != nil {
			fmt.Printf("Failed to stop CheckHealth service: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("CheckHealth service (PID: %d) has been stopped\n", pid)
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
