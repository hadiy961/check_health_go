package cpu

import (
	"fmt"
)

// FormatClockSpeed formats CPU clock speed
func FormatClockSpeed(clockSpeed float64) string {
	return fmt.Sprintf("%.2f GHz", clockSpeed)
}
