package models

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// SystemInfo holds system resource information
type SystemInfo struct {
	TotalMemoryMB int
	UsedDiskMB    int
	TotalDiskMB   int
	MountPoint    string
}

// GetSystemInfo retrieves system resource information
func GetSystemInfo() (*SystemInfo, error) {
	memInfo, err := getMemoryInfo()
	if err != nil {
		return nil, err
	}

	diskInfo, err := getDiskInfo()
	if err != nil {
		return nil, err
	}

	return &SystemInfo{
		TotalMemoryMB: memInfo,
		UsedDiskMB:    diskInfo.UsedMB,
		TotalDiskMB:   diskInfo.TotalMB,
		MountPoint:    diskInfo.MountPoint,
	}, nil
}

// getMemoryInfo reads total memory from /proc/meminfo
func getMemoryInfo() (int, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				memKB, err := strconv.Atoi(fields[1])
				if err != nil {
					return 0, err
				}
				// Convert KB to MB
				return memKB / 1024, nil
			}
		}
	}

	return 0, scanner.Err()
}

// diskInfo holds disk usage information
type diskInfo struct {
	UsedMB     int
	TotalMB    int
	MountPoint string
}

// getDiskInfo gets disk usage information for the root filesystem
func getDiskInfo() (*diskInfo, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs("/", &stat)
	if err != nil {
		return nil, err
	}

	// Calculate disk usage in MB
	blockSize := uint64(stat.Bsize)
	totalBytes := stat.Blocks * blockSize
	freeBytes := stat.Bavail * blockSize
	usedBytes := totalBytes - freeBytes

	totalMB := int(totalBytes / (1024 * 1024))
	usedMB := int(usedBytes / (1024 * 1024))

	return &diskInfo{
		UsedMB:     usedMB,
		TotalMB:    totalMB,
		MountPoint: "/",
	}, nil
}