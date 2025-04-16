package disk

import (
	"fmt"
	"strings"

	"github.com/shirou/gopsutil/disk"
)

// Make GetStorageInfo a variable so it can be mocked in tests
var getStorageInfoFunc = getStorageInfo

// GetStorageInfo is a wrapper around getStorageInfo for easy mocking in tests
func GetStorageInfo() ([]StorageInfo, *TotalStorage, error) {
	return getStorageInfoFunc()
}

// Function to get disk I/O information
func getDiskIO(deviceName string) (*DiskIOInfo, error) {
	ioCounters, err := disk.IOCounters()
	if err != nil {
		return nil, err
	}

	// Extract actual device name from full path
	// e.g., /dev/sda1 -> sda1 or sda
	deviceShortName := deviceName
	if strings.HasPrefix(deviceName, "/dev/") {
		deviceShortName = strings.TrimPrefix(deviceName, "/dev/")
		// Some systems report IO stats at the disk level, not partition level
		// Try both the full partition name and the disk name
		parts := strings.Split(deviceShortName, "")
		if len(parts) > 0 && len(parts[0]) > 0 {
			// Remove the partition number to get the disk name
			for i, c := range parts[0] {
				if c >= '0' && c <= '9' {
					deviceShortName = parts[0][:i]
					break
				}
			}
		}
	}

	// Check if we have I/O stats for this device
	for name, counters := range ioCounters {
		if name == deviceShortName || strings.HasPrefix(name, deviceShortName) {
			// Calculate rates (this would be more accurate with previous measurements)
			// In a real implementation, you might want to store previous values and calculate actual rates
			readBytesPS := float64(counters.ReadBytes) / 1024   // Simplified rate calculation
			writeBytesPS := float64(counters.WriteBytes) / 1024 // Simplified rate calculation

			return &DiskIOInfo{
				ReadCount:    counters.ReadCount,
				WriteCount:   counters.WriteCount,
				ReadBytes:    counters.ReadBytes,
				WriteBytes:   counters.WriteBytes,
				ReadTime:     counters.ReadTime,
				WriteTime:    counters.WriteTime,
				IoTime:       counters.IoTime,
				WeightedIO:   counters.WeightedIO,
				ReadBytesPS:  readBytesPS,
				WriteBytesPS: writeBytesPS,
			}, nil
		}
	}

	// Return empty IO stats if we couldn't find matching device
	return nil, nil
}

// getStorageInfo is the actual implementation of storage info retrieval
func getStorageInfo() ([]StorageInfo, *TotalStorage, error) {
	// Ambil daftar partisi
	partitions, err := disk.Partitions(true)
	if err != nil {
		return nil, nil, err
	}

	var storageInfos []StorageInfo
	var totalCapacity, totalUsed, totalFree uint64
	var totalCapacityInternal, totalUsedInternal, totalFreeInternal uint64
	var totalCapacityExternal, totalUsedExternal, totalFreeExternal uint64
	var totalDeviceInternal, totalDeviceExternal int
	var hasInternalStorage bool = false

	// Iterasi setiap partisi untuk mendapatkan informasi detail
	for _, partition := range partitions {
		usageStat, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			// Jika ada error pada partisi tertentu, lanjutkan ke partisi berikutnya
			fmt.Printf("Error getting usage for partition %s: %v\n", partition.Mountpoint, err)
			continue
		}

		// Get I/O information for this device
		ioInfo, err := getDiskIO(partition.Device)
		if err != nil {
			fmt.Printf("Error getting I/O stats for device %s: %v\n", partition.Device, err)
			// Continue with nil I/O info
		}

		// Check if this is an external mount
		isExternal := isExternalMount(partition.Mountpoint, partition.Device)

		// Tambahkan informasi partisi ke dalam hasil - only if total > 0
		if usageStat.Total > 0 {
			storageInfos = append(storageInfos, StorageInfo{
				Device:     partition.Device,
				MountPoint: partition.Mountpoint,
				FileSystem: partition.Fstype,
				Total:      usageStat.Total,
				Used:       usageStat.Used,
				Free:       usageStat.Free,
				Usage:      usageStat.UsedPercent,
				IsReadOnly: partition.Opts == "ro", // Periksa apakah partisi hanya-baca
				IsExternal: isExternal,             // Set whether this is an external storage
				IO:         ioInfo,                 // Add I/O information
			})

			// Count and track storage by type (internal vs external)
			if isExternal {
				totalCapacityExternal += usageStat.Total
				totalUsedExternal += usageStat.Used
				totalFreeExternal += usageStat.Free
				totalDeviceExternal++
			} else {
				totalCapacityInternal += usageStat.Total
				totalUsedInternal += usageStat.Used
				totalFreeInternal += usageStat.Free
				totalDeviceInternal++
				hasInternalStorage = true
			}
		}

		// Existing code for tracking total (internal only) is still needed for backward compatibility
		if !isExternal && usageStat.Total > 0 {
			totalCapacity += usageStat.Total
			totalUsed += usageStat.Used
			totalFree += usageStat.Free
		}
	}

	// Calculate combined totals for all storage (internal + external)
	combinedTotalCapacity := totalCapacityInternal + totalCapacityExternal
	combinedTotalUsed := totalUsedInternal + totalUsedExternal
	combinedTotalFree := totalFreeInternal + totalFreeExternal

	// Calculate combined usage percentage
	var combinedUsagePercent float64
	if combinedTotalCapacity > 0 {
		combinedUsagePercent = (float64(combinedTotalUsed) / float64(combinedTotalCapacity)) * 100
	}

	// Jika tidak ada storage internal, return nil untuk TotalStorage
	if !hasInternalStorage || totalCapacity == 0 {
		return storageInfos, nil, nil
	}

	// Calculate usage percentages for internal and external storage
	var totalUsagePercentInternal, totalUsagePercentExternal float64
	if totalCapacityInternal > 0 {
		totalUsagePercentInternal = (float64(totalUsedInternal) / float64(totalCapacityInternal)) * 100
	}
	if totalCapacityExternal > 0 {
		totalUsagePercentExternal = (float64(totalUsedExternal) / float64(totalCapacityExternal)) * 100
	}

	// Kembalikan informasi storage dan total storage (removed monitoredPathInfos)
	return storageInfos, &TotalStorage{
		TotalCapacity: combinedTotalCapacity,
		TotalUsed:     combinedTotalUsed,
		TotalFree:     combinedTotalFree,
		UsagePercent:  combinedUsagePercent,

		// Add internal storage metrics
		TotalCapacityInternal:     totalCapacityInternal,
		TotalUsedInternal:         totalUsedInternal,
		TotalFreeInternal:         totalFreeInternal,
		TotalDeviceInternal:       totalDeviceInternal,
		TotalUsagePercentInternal: totalUsagePercentInternal,

		// Add external storage metrics
		TotalCapacityExternal:     totalCapacityExternal,
		TotalUsedExternal:         totalUsedExternal,
		TotalFreeExternal:         totalFreeExternal,
		TotalDeviceExternal:       totalDeviceExternal,
		TotalUsagePercentExternal: totalUsagePercentExternal,
	}, nil
}
