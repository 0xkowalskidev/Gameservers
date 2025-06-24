package models

type PortMapping struct {
	Name          string `json:"name"`     // "game", "rcon", "query", etc.
	Protocol      string `json:"protocol"` // "tcp" or "udp"
	ContainerPort int    `json:"container_port"`
	HostPort      int    `json:"host_port"` // 0 = auto-assign
}

// PortAllocator manages port assignments for gameservers
type PortAllocator struct {
	startPort int
	endPort   int
}

func NewPortAllocator() *PortAllocator {
	return &PortAllocator{
		startPort: 25565, // Start from Minecraft's default port
		endPort:   35565, // Allow up to 10,000 ports
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

			// Check if we already assigned a port for this name
			if assignedPort, exists := portGroups[portName]; exists {
				server.PortMappings[i].HostPort = assignedPort
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
	return nil
}

func (pa *PortAllocator) findAvailablePort(usedPorts map[int]bool) (int, error) {
	for port := pa.startPort; port <= pa.endPort; port++ {
		if !usedPorts[port] {
			return port, nil
		}
	}
	return 0, &DatabaseError{
		Op:  "allocate_port",
		Msg: "no available ports in range",
		Err: nil,
	}
}
