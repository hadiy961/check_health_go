package alerts

import (
	"CheckHealthDO/internal/monitoring/server/sysinfo"
	"CheckHealthDO/internal/pkg/logger"
)

// GetServerInfoForAlert retrieves system information for inclusion in alerts
func GetServerInfoForAlert() *ServerInfo {
	sysinfo, err := sysinfo.GetSystemInfo()
	if err != nil {
		logger.Error("Failed to get server information for alert",
			logger.String("error", err.Error()))
		return nil
	}

	if sysinfo == nil {
		return nil
	}

	// Convert system info to ServerInfo structure
	return &ServerInfo{
		Hostname:        sysinfo.Hostname,
		IPAddress:       sysinfo.IPAddresses[0], // Assuming first IP is the main one
		Platform:        sysinfo.Platform,
		PlatformVersion: sysinfo.PlatformVersion,
		KernelVersion:   sysinfo.KernelVersion,
		Uptime:          sysinfo.Uptime,
	}
}
