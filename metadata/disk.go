package metadata

import (
	"fmt"

	"github.com/shirou/gopsutil/v4/disk"
)

func GetTotalDiskGB() (float64, error) {
	// 3. Storage (Total of all physical partitions)
	partitions, err := disk.Partitions(false)
	if err != nil {
		return 0, fmt.Errorf("failed to get disk partitions: %v", err)
	}

	var totalDiskSpace uint64
	processedDevices := make(map[string]bool)

	for _, p := range partitions {
		// Avoid duplicate counting (e.g., multiple mounts of same device)
		if _, ok := processedDevices[p.Device]; ok {
			continue
		}

		// Get usage stats for the mount point
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue // Skip if permission denied or virtual fs
		}

		totalDiskSpace += usage.Total
		processedDevices[p.Device] = true
	}
	return float64(totalDiskSpace) / 1024 / 1024 / 1024, nil
}
