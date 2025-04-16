package sysinfo

import (
	"fmt"
	"net"
	"time"

	"github.com/shirou/gopsutil/host"
)

// GetSystemInfo retrieves general system information
func GetSystemInfo() (*SystemInfo, error) {
	// Get uptime
	uptimeSeconds, err := host.Uptime()
	if err != nil {
		return nil, fmt.Errorf("failed to get uptime: %w", err)
	}
	uptime := fmt.Sprintf("%d days, %d hours, %d minutes",
		uptimeSeconds/86400, (uptimeSeconds%86400)/3600, (uptimeSeconds%3600)/60)

	// Get current time
	currentTime := time.Now().Format(time.RFC3339)

	// Get host statistics
	hostStat, err := host.Info()
	if err != nil {
		return nil, fmt.Errorf("failed to get host info: %w", err)
	}

	// Get kernel version
	kernelVersion := hostStat.KernelVersion

	// Get IP addresses
	ipAddresses, err := getIPAddresses()
	if err != nil {
		return nil, fmt.Errorf("failed to get IP addresses: %w", err)
	}

	// Build system info
	sysInfo := &SystemInfo{
		Uptime:          uptime,
		CurrentTime:     currentTime,
		ProcessCount:    int(hostStat.Procs),
		Hostname:        hostStat.Hostname,
		OS:              hostStat.OS,
		Platform:        hostStat.Platform,
		PlatformVersion: hostStat.PlatformVersion,
		KernelVersion:   kernelVersion,
		IPAddresses:     ipAddresses,
	}

	return sysInfo, nil
}

// getIPAddresses retrieves all non-loopback IP addresses
func getIPAddresses() ([]string, error) {
	var ips []string
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip != nil && ip.IsGlobalUnicast() {
				ips = append(ips, ip.String())
			}
		}
	}

	return ips, nil
}
