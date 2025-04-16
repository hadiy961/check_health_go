package memory

import (
	"time"
)

// MemoryInfo represents system memory information
type MemoryInfo struct {
	TotalMemory          uint64  `json:"total_memory"`
	UsedMemory           uint64  `json:"used_memory"`
	FreeMemory           uint64  `json:"free_memory"`
	AvailableMemory      uint64  `json:"available_memory"` // Memory that can be made available without swapping
	UsedMemoryPercentage float64 `json:"used_memory_percent"`
	FreeMemoryPercentage float64 `json:"free_memory_percent"`
	MemoryStatus         string  `json:"memory_status"`

	// Additional memory metrics
	CachedMemory   uint64 `json:"cached_memory"`   // Memory used for file caching
	BufferMemory   uint64 `json:"buffer_memory"`   // Memory used for kernel buffers
	ActiveMemory   uint64 `json:"active_memory"`   // Memory recently used
	InactiveMemory uint64 `json:"inactive_memory"` // Memory not recently used
	SharedMemory   uint64 `json:"shared_memory"`   // Memory shared between processes

	// Swap metrics
	SwapTotal          uint64  `json:"swap_total"`
	SwapUsed           uint64  `json:"swap_used"`
	SwapFree           uint64  `json:"swap_free"`
	SwapUsedPercentage float64 `json:"swap_used_percent"` // Percentage of swap space used
}

// MemoryMetricsMsg is the message structure for WebSocket updates
type MemoryMetricsMsg struct {
	Timestamp      time.Time  `json:"timestamp"`
	Memory         MemoryInfo `json:"memory"`
	LastUpdateTime string     `json:"last_update_time"`
}
