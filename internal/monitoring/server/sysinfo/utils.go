package sysinfo

import "fmt"

// FormatUptime formats uptime in seconds to a human-readable string
func FormatUptime(seconds uint64) string {
	return fmt.Sprintf("%d days, %d hours, %d minutes",
		seconds/86400, (seconds%86400)/3600, (seconds%3600)/60)
}
