package websocket

import (
	"CheckHealthDO/internal/pkg/logger"
	"encoding/json"
)

// BroadcastCPU sends CPU metrics to all connected clients
func (r *Registry) BroadcastCPU(metrics interface{}) {
	if handler := r.GetCPUHandler(); handler != nil {
		data, err := json.Marshal(map[string]interface{}{
			"cpu":       metrics,
			"timestamp": timeNow(),
		})
		if err != nil {
			logger.Error("Failed to marshal CPU metrics for WebSocket broadcast",
				logger.String("error", err.Error()))
			return
		}
		handler.Broadcast(data)
	}
}

// BroadcastSysInfo sends SysInfo metrics to all connected clients
func (r *Registry) BroadcastSysInfo(metrics interface{}) {
	if handler := r.GetSysHandler(); handler != nil {
		data, err := json.Marshal(map[string]interface{}{
			"sys_info":  metrics,
			"timestamp": timeNow(),
		})
		if err != nil {
			logger.Error("Failed to marshal system metrics for WebSocket broadcast",
				logger.String("error", err.Error()))
			return
		}
		handler.Broadcast(data)
	}
}

// BroadcastSysInfo sends SysInfo metrics to all connected clients
func (r *Registry) BroadcastDisk(metrics interface{}) {
	if handler := r.GetDiskHandler(); handler != nil {
		data, err := json.Marshal(map[string]interface{}{
			"disk":      metrics,
			"timestamp": timeNow(),
		})
		if err != nil {
			logger.Error("Failed to marshal system metrics for WebSocket broadcast",
				logger.String("error", err.Error()))
			return
		}
		handler.Broadcast(data)
	}
}

// BroadcastMemory sends memory metrics to all connected clients
func (r *Registry) BroadcastMemory(metrics interface{}) {
	if handler := r.GetMemoryHandler(); handler != nil {
		data, err := json.Marshal(map[string]interface{}{
			"memory":    metrics,
			"timestamp": timeNow(),
		})
		if err != nil {
			logger.Error("Failed to marshal memory metrics for WebSocket broadcast",
				logger.String("error", err.Error()))
			return
		}
		handler.Broadcast(data)
	}
}

// BroadcastMariaDB sends MariaDB metrics to all connected clients
func (r *Registry) BroadcastMariaDB(metrics interface{}) {
	if handler := r.GetMariaDBHandler(); handler != nil {
		data, err := json.Marshal(metrics)
		if err != nil {
			logger.Error("Failed to marshal MariaDB metrics for WebSocket broadcast",
				logger.String("error", err.Error()))
			return
		}
		handler.Broadcast(data)
	}
}

// BroadcastMetrics sends general metrics to all connected handlers
func (r *Registry) BroadcastMetrics(metrics interface{}) {
	// Format the metrics for broadcast
	data, err := json.Marshal(metrics)
	if err != nil {
		logger.Error("Failed to marshal metrics for WebSocket broadcast",
			logger.String("error", err.Error()))
		return
	}

	// Determine which handler to use based on metric_type in the data
	// This allows specific metric types to go to their specific handlers
	// instead of broadcasting to all handlers
	metricsMap, ok := metrics.(map[string]interface{})
	if ok {
		if metricType, exists := metricsMap["metric_type"]; exists {
			switch metricType {
			case "cpu":
				if h := r.GetCPUHandler(); h != nil {
					h.Broadcast(data)
				}
				return
			case "memory":
				if h := r.GetMemoryHandler(); h != nil {
					h.Broadcast(data)
				}
				return
			case "mariadb":
				if h := r.GetMariaDBHandler(); h != nil {
					h.Broadcast(data)
				}
				return
			}
		}
	}

	// If no specific type is found or unable to determine type,
	// log a warning (optional)
	logger.Debug("Broadcasting metrics without specific handler type")
}

// Helper function to get current time as string
func timeNow() string {
	return formatTime(getCurrentTime())
}

// These functions can be implemented based on your time formatting needs
func formatTime(t interface{}) string {
	return "" // Implement according to your format needs
}

func getCurrentTime() interface{} {
	return nil // Implement to return current time
}
