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
// Ports are allocated sequentially from minPort (49152) upward
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

			// Find next available port sequentially
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

// ValidateManualPorts validates user-specified port mappings
// Returns error if ports are invalid (out of range 1-65535, duplicates, or inconsistent same-named ports)
func ValidateManualPorts(mappings []PortMapping) error {
	usedPorts := make(map[int]bool)
	portGroups := make(map[string]int) // name -> assigned port (for TCP+UDP consistency)

	for _, pm := range mappings {
		// Check port is in valid range (1-65535 for manual)
		if pm.HostPort < 1 || pm.HostPort > 65535 {
			return &OperationError{
				Op:  "validate_port",
				Msg: fmt.Sprintf("port %d is out of valid range (1-65535)", pm.HostPort),
			}
		}

		// Check same-named ports have same host port (e.g., TCP+UDP on same port)
		if existingPort, exists := portGroups[pm.Name]; exists {
			if existingPort != pm.HostPort {
				return &OperationError{
					Op:  "validate_port",
					Msg: fmt.Sprintf("port mappings with name '%s' must use the same host port (got %d and %d)", pm.Name, existingPort, pm.HostPort),
				}
			}
		} else {
			// First time seeing this name, check for duplicates with different names
			if usedPorts[pm.HostPort] {
				return &OperationError{
					Op:  "validate_port",
					Msg: fmt.Sprintf("duplicate host port %d", pm.HostPort),
				}
			}
			portGroups[pm.Name] = pm.HostPort
			usedPorts[pm.HostPort] = true
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