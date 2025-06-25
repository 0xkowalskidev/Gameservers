package models

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// SystemInfo holds system resource information
type SystemInfo struct {
	TotalMemoryMB int
}

// GetSystemInfo retrieves system resource information
func GetSystemInfo() (*SystemInfo, error) {
	memInfo, err := getMemoryInfo()
	if err != nil {
		return nil, err
	}

	return &SystemInfo{
		TotalMemoryMB: memInfo,
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