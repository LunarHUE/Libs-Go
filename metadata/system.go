package metadata

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v4/mem"
)

type SystemInfo struct {
	Hostname      string  `json:"hostname"`
	OS            string  `json:"os"`
	Arch          string  `json:"architecture"`
	CPUCores      int     `json:"cpu_cores"`
	TotalMemoryGB float64 `json:"total_memory_gb"`
	TotalDiskGB   float64 `json:"total_disk_gb"`
	MainIP        string  `json:"main_ip"`
	MainMAC       string  `json:"main_mac"`
}

func GetSystemInfo() (*SystemInfo, error) {
	info := &SystemInfo{
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUCores: runtime.NumCPU(),
	}

	// 1. Hostname
	name, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %v", err)
	}
	info.Hostname = name

	// 2. Memory (RAM)
	vMem, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory info: %v", err)
	}
	// Convert bytes to GB
	info.TotalMemoryGB = float64(vMem.Total) / 1024 / 1024 / 1024

	// 3. Storage (Total of all physical partitions)
	totalDisk, err := GetTotalDiskGB()
	if err != nil {
		return nil, fmt.Errorf("failed to get disk info: %v", err)
	}
	info.TotalDiskGB = totalDisk

	// 4. Network (IP and MAC)
	// We use a trick to determine the "outbound" IP by dialing a public DNS
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		// Fallback if no internet: just grab the first non-loopback
		info.MainIP = "Offline/Unknown"
		info.MainMAC = "Unknown"
	} else {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		info.MainIP = localAddr.IP.String()

		// Find the interface that matches this IP to get the MAC
		interfaces, _ := net.Interfaces()
		for _, iface := range interfaces {
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				// Check if the interface IP matches our outbound IP
				if strings.Contains(addr.String(), info.MainIP) {
					info.MainMAC = iface.HardwareAddr.String()
					break
				}
			}
		}
	}

	return info, nil
}
