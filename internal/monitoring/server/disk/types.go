package disk

// DiskIOInfo represents disk I/O statistics
type DiskIOInfo struct {
	ReadCount    uint64  `json:"read_count"`     // Number of reads
	WriteCount   uint64  `json:"write_count"`    // Number of writes
	ReadBytes    uint64  `json:"read_bytes"`     // Bytes read
	WriteBytes   uint64  `json:"write_bytes"`    // Bytes written
	ReadTime     uint64  `json:"read_time"`      // Time spent reading (ms)
	WriteTime    uint64  `json:"write_time"`     // Time spent writing (ms)
	IoTime       uint64  `json:"io_time"`        // Time spent doing I/Os (ms)
	WeightedIO   uint64  `json:"weighted_io"`    // Weighted time spent doing I/Os (ms)
	ReadBytesPS  float64 `json:"read_bytes_ps"`  // Read bytes per second
	WriteBytesPS float64 `json:"write_bytes_ps"` // Write bytes per second
}

// StorageInfo contains detailed information about a storage device
type StorageInfo struct {
	Device     string      `json:"device"`
	MountPoint string      `json:"mount_point"`
	FileSystem string      `json:"file_system"`
	Total      uint64      `json:"total"`
	Used       uint64      `json:"used"`
	Free       uint64      `json:"free"`
	Usage      float64     `json:"usage"`
	IsReadOnly bool        `json:"is_readonly"`
	IsExternal bool        `json:"is_external"`
	IO         *DiskIOInfo `json:"io,omitempty"` // I/O information
}

// TotalStorage contains aggregated storage information
type TotalStorage struct {
	TotalCapacity uint64  `json:"total_capacity"`
	TotalUsed     uint64  `json:"total_used"`
	TotalFree     uint64  `json:"total_free"`
	UsagePercent  float64 `json:"usage_percent"`

	// Internal storage metrics
	TotalCapacityInternal     uint64  `json:"total_capacity_internal"`
	TotalUsedInternal         uint64  `json:"total_used_internal"`
	TotalFreeInternal         uint64  `json:"total_free_internal"`
	TotalDeviceInternal       int     `json:"total_device_internal"`
	TotalUsagePercentInternal float64 `json:"total_usage_percent_internal"`

	// External storage metrics
	TotalCapacityExternal     uint64  `json:"total_capacity_external"`
	TotalUsedExternal         uint64  `json:"total_used_external"`
	TotalFreeExternal         uint64  `json:"total_free_external"`
	TotalDeviceExternal       int     `json:"total_device_external"`
	TotalUsagePercentExternal float64 `json:"total_usage_percent_external"`
}
