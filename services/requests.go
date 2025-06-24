package services

// CreateGameserverRequest represents a request to create a new gameserver
type CreateGameserverRequest struct {
	Name     string
	GameID   string
	Port     int
	MemoryMB int
	CPUCores float64
}

// UpdateGameserverRequest represents a request to update a gameserver
type UpdateGameserverRequest struct {
	Name     string
	Port     int
	MemoryMB int
}