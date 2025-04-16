package sysinfo

// SystemInfo represents general system information
type SystemInfo struct {
	Uptime          string   `json:"uptime"`           // System uptime
	CurrentTime     string   `json:"current_time"`     // Current system time
	ProcessCount    int      `json:"process_count"`    // Number of running processes
	Hostname        string   `json:"hostname"`         // Hostname of the system
	OS              string   `json:"os"`               // Operating system
	Platform        string   `json:"platform"`         // Platform name
	PlatformVersion string   `json:"platform_version"` // Platform version
	KernelVersion   string   `json:"kernel_version"`   // Kernel version
	IPAddresses     []string `json:"ip_addresses"`     // List of IP addresses
}
