package cpu

import (
	"time"
)

// CPUInfo represents CPU statistics
type CPUInfo struct {
	ModelName  string    `json:"model_name"`  // Nama model CPU
	Cores      int       `json:"cores"`       // Jumlah core fisik
	Threads    int       `json:"threads"`     // Jumlah thread (logical processors)
	ClockSpeed float64   `json:"clock_speed"` // Kecepatan clock dalam GHz
	Usage      float64   `json:"usage"`       // Persentase penggunaan CPU (total)
	CoreUsage  []float64 `json:"core_usage"`  // Persentase penggunaan per core
	CacheSize  int       `json:"cache_size"`  // Ukuran cache CPU dalam KB
	IsVirtual  bool      `json:"is_virtual"`  // Apakah berjalan di atas VM
	Hypervisor string    `json:"hypervisor"`  // Nama hypervisor jika berjalan di atas VM
	CPUStatus  string    `json:"cpu_status"`  // Status CPU: normal, warning, critical

	// New fields
	VendorID string   `json:"vendor_id"` // CPU vendor (Intel, AMD, etc)
	Family   string   `json:"family"`    // CPU family
	Stepping int      `json:"stepping"`  // CPU stepping/revision
	Flags    []string `json:"flags"`     // CPU feature flags

	// Additional CPU information fields
	PhysicalID     string             `json:"physical_id"`     // Physical ID for multi-socket systems
	Microcode      string             `json:"microcode"`       // CPU microcode version
	Architecture   string             `json:"architecture"`    // CPU architecture (x86_64, ARM, etc.)
	MinFrequency   float64            `json:"min_frequency"`   // Minimum frequency in GHz
	MaxFrequency   float64            `json:"max_frequency"`   // Maximum frequency in GHz
	CPUTimes       map[string]float64 `json:"cpu_times"`       // CPU time breakdown (user, system, idle)
	Temperature    float64            `json:"temperature"`     // CPU temperature if available
	ProcessorCount int                `json:"processor_count"` // Number of physical processor packages
}

// CPUMetricsMsg is the message structure for WebSocket updates
type CPUMetricsMsg struct {
	Timestamp      time.Time `json:"timestamp"`
	CPU            CPUInfo   `json:"cpu"`
	LastUpdateTime string    `json:"last_update_time"`
}
