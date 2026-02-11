package models

import (
	"fmt"
)

type PortMapping struct {
	Name          string `json:"name"`     // "game", "rcon", "query", etc.
	Protocol      string `json:"protocol"` // "tcp" or "udp"
	ContainerPort int    `json:"container_port"`
	HostPort      int    `json:"host_port"` // 0 = auto-assign
}

// Port allocation range (IANA recommended for ephemeral ports)
const (
	minPort = 49152
	maxPort = 65535
)

// isPortAvailable checks if a port is within range and not already used
func isPortAvailable(port int, usedPorts map[int]bool) bool {
	return port >= minPort && port <= maxPort && !usedPorts[port]
}

// AllocatePortsForServer assigns available ports to all zero-valued port mappings
// Port mappings with the same name will get the same host port (for TCP+UDP on same port)
func AllocatePortsForServer(server *Gameserver, usedPorts map[int]bool) error {
	// Group port mappings by name to assign same port to same-named mappings
	portGroups := make(map[string]int) // name -> assigned port

	for i := range server.PortMappings {
		if server.PortMappings[i].HostPort == 0 {
			portName := server.PortMappings[i].Name

			// Check if we already assigned a port for this name
			if assignedPort, exists := portGroups[portName]; exists {
				server.PortMappings[i].HostPort = assignedPort
				continue
			}

			// Try container port first if it's in valid range
			containerPort := server.PortMappings[i].ContainerPort
			if containerPort > 0 && isPortAvailable(containerPort, usedPorts) {
				server.PortMappings[i].HostPort = containerPort
				portGroups[portName] = containerPort
				usedPorts[containerPort] = true
				continue
			}

			// Container port not available, find next available port
			port, err := findAvailablePort(usedPorts)
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

// findAvailablePort finds the next available port in the valid range
func findAvailablePort(usedPorts map[int]bool) (int, error) {
	for port := minPort; port <= maxPort; port++ {
		if !usedPorts[port] {
			return port, nil
		}
	}

	return 0, &DatabaseError{
		Op:  "allocate_port",
		Msg: fmt.Sprintf("no available ports in range %d-%d", minPort, maxPort),
		Err: nil,
	}
}