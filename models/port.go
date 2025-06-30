package models

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

type PortMapping struct {
	Name          string `json:"name"`     // "game", "rcon", "query", etc.
	Protocol      string `json:"protocol"` // "tcp" or "udp"
	ContainerPort int    `json:"container_port"`
	HostPort      int    `json:"host_port"` // 0 = auto-assign
}

// PortAllocator manages port assignments for gameservers
type PortAllocator struct {
	minPort int
	maxPort int
}

func NewPortAllocator() *PortAllocator {
	// Default port range - allow most ports but stay away from system ports
	minPort := 1024
	maxPort := 65535

	// Read from environment variables if set
	if startEnv := os.Getenv("PORT_RANGE_START"); startEnv != "" {
		if start, err := strconv.Atoi(startEnv); err == nil && start >= 1024 && start <= 65535 {
			minPort = start
		}
	}

	if endEnv := os.Getenv("PORT_RANGE_END"); endEnv != "" {
		if end, err := strconv.Atoi(endEnv); err == nil && end >= 1024 && end <= 65535 && end > minPort {
			maxPort = end
		}
	}

	return &PortAllocator{
		minPort: minPort,
		maxPort: maxPort,
	}
}

// isPortAvailable checks if a port is available on the system
func (pa *PortAllocator) isPortAvailable(port int, protocol string) bool {
	if port < pa.minPort || port > pa.maxPort {
		return false
	}

	// Try to bind to the port to see if it's available
	address := fmt.Sprintf(":%d", port)
	
	switch protocol {
	case "tcp":
		listener, err := net.Listen("tcp", address)
		if err != nil {
			return false
		}
		listener.Close()
		return true
	case "udp":
		conn, err := net.ListenPacket("udp", address)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	default:
		return false
	}
}

// AllocatePortsForServer assigns available ports to all zero-valued port mappings
// Port mappings with the same name will get the same host port (for TCP+UDP on same port)
func (pa *PortAllocator) AllocatePortsForServer(server *Gameserver, usedPorts map[int]bool) error {
	// Group port mappings by name to assign same port to same-named mappings
	portGroups := make(map[string]int) // name -> assigned port

	for i := range server.PortMappings {
		if server.PortMappings[i].HostPort == 0 {
			portName := server.PortMappings[i].Name
			protocol := server.PortMappings[i].Protocol

			// Check if we already assigned a port for this name
			if assignedPort, exists := portGroups[portName]; exists {
				server.PortMappings[i].HostPort = assignedPort
				continue
			}

			// Try container port first (preferred approach)
			containerPort := server.PortMappings[i].ContainerPort
			if containerPort > 0 && !usedPorts[containerPort] && pa.isPortAvailable(containerPort, protocol) {
				server.PortMappings[i].HostPort = containerPort
				portGroups[portName] = containerPort
				usedPorts[containerPort] = true
				continue
			}

			// Container port not available, find next available port starting from container port + 1
			port, err := pa.findAvailablePort(containerPort+1, protocol, usedPorts)
			if err != nil {
				return err
			}
			
			server.PortMappings[i].HostPort = port
			portGroups[portName] = port
			usedPorts[port] = true
		}
	}
	return nil
}

// findAvailablePort finds the next available port starting from the given startPort
func (pa *PortAllocator) findAvailablePort(startPort int, protocol string, usedPorts map[int]bool) (int, error) {
	// Ensure we start within our valid range
	if startPort < pa.minPort {
		startPort = pa.minPort
	}

	// Search from startPort to maxPort
	for port := startPort; port <= pa.maxPort; port++ {
		if !usedPorts[port] && pa.isPortAvailable(port, protocol) {
			return port, nil
		}
	}

	// If we didn't find anything above startPort, search from minPort to startPort-1
	for port := pa.minPort; port < startPort; port++ {
		if !usedPorts[port] && pa.isPortAvailable(port, protocol) {
			return port, nil
		}
	}

	return 0, &DatabaseError{
		Op:  "allocate_port",
		Msg: fmt.Sprintf("no available %s ports in range %d-%d", protocol, pa.minPort, pa.maxPort),
		Err: nil,
	}
}