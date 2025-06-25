package models

import (
	"os"
	"strconv"
	"strings"
)

type PortMapping struct {
	Name          string `json:"name"`     // "game", "rcon", "query", etc.
	Protocol      string `json:"protocol"` // "tcp" or "udp"
	ContainerPort int    `json:"container_port"`
	HostPort      int    `json:"host_port"` // 0 = auto-assign
}

// PortAllocator manages port assignments for gameservers
type PortAllocator struct {
	portRangeStart int
	portRangeEnd   int
	allowedPorts   []int
}

func NewPortAllocator() *PortAllocator {
	// Default port range (Kubernetes style)
	portRangeStart := 30000
	portRangeEnd := 32767
	var allowedPorts []int

	// Read from environment variables
	if startEnv := os.Getenv("PORT_RANGE_START"); startEnv != "" {
		if start, err := strconv.Atoi(startEnv); err == nil && start >= 1024 && start <= 65535 {
			portRangeStart = start
		}
	}

	if endEnv := os.Getenv("PORT_RANGE_END"); endEnv != "" {
		if end, err := strconv.Atoi(endEnv); err == nil && end >= 1024 && end <= 65535 && end > portRangeStart {
			portRangeEnd = end
		}
	}

	// Parse allowed ports (e.g., "25565,27015,7777")
	if allowedEnv := os.Getenv("ALLOWED_PORTS"); allowedEnv != "" {
		portStrs := strings.Split(allowedEnv, ",")
		for _, portStr := range portStrs {
			portStr = strings.TrimSpace(portStr)
			if portStr == "" {
				continue
			}
			if port, err := strconv.Atoi(portStr); err == nil && port >= 1 && port <= 65535 {
				allowedPorts = append(allowedPorts, port)
			}
		}
	}

	// Default allowed ports for common games
	if len(allowedPorts) == 0 {
		allowedPorts = []int{25565, 27015, 7777, 8211, 2456}
	}

	return &PortAllocator{
		portRangeStart: portRangeStart,
		portRangeEnd:   portRangeEnd,
		allowedPorts:   allowedPorts,
	}
}

// isPortAllowed checks if a port is allowed for allocation
func (pa *PortAllocator) isPortAllowed(port int) bool {
	// Check if port is in allowed ports list
	for _, allowedPort := range pa.allowedPorts {
		if port == allowedPort {
			return true
		}
	}

	// Check if port is in range
	return port >= pa.portRangeStart && port <= pa.portRangeEnd
}

// AllocatePortsForServer assigns available ports to all zero-valued port mappings
// Port mappings with the same name will get the same host port (for TCP+UDP on same port)
func (pa *PortAllocator) AllocatePortsForServer(server *Gameserver, usedPorts map[int]bool) error {
	// Group port mappings by name to assign same port to same-named mappings
	portGroups := make(map[string]int) // name -> assigned port

	for i := range server.PortMappings {
		if server.PortMappings[i].HostPort == 0 {
			portName := server.PortMappings[i].Name

			// Check if we already assigned a port for this name
			if assignedPort, exists := portGroups[portName]; exists {
				server.PortMappings[i].HostPort = assignedPort
			} else {
				// Try to assign the container port first (game-specific default)
				containerPort := server.PortMappings[i].ContainerPort
				if containerPort > 0 && !usedPorts[containerPort] && pa.isPortAllowed(containerPort) {
					server.PortMappings[i].HostPort = containerPort
					portGroups[portName] = containerPort
					usedPorts[containerPort] = true
				} else {
					// Find a new available port for this name
					port, err := pa.findAvailablePort(usedPorts)
					if err != nil {
						return err
					}
					server.PortMappings[i].HostPort = port
					portGroups[portName] = port
					usedPorts[port] = true
				}
			}
		}
	}
	return nil
}

func (pa *PortAllocator) findAvailablePort(usedPorts map[int]bool) (int, error) {
	// Check allowed ports first
	for _, port := range pa.allowedPorts {
		if !usedPorts[port] {
			return port, nil
		}
	}
	
	// Then check the port range
	for port := pa.portRangeStart; port <= pa.portRangeEnd; port++ {
		if !usedPorts[port] {
			return port, nil
		}
	}
	
	return 0, &DatabaseError{
		Op:  "allocate_port",
		Msg: "no available ports in configured range or allowed ports",
		Err: nil,
	}
}
