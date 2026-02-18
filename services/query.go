package services

import (
	"context"
	"fmt"
	"time"

	"github.com/0xkowalskidev/gameserverquery/protocol"
	"github.com/0xkowalskidev/gameserverquery/query"
	"github.com/rs/zerolog/log"

	"0xkowalskidev/gameservers/models"
)

// QueryService handles game server queries
type QueryService struct {
	// We can add caching here if needed
}

// NewQueryService creates a new query service
func NewQueryService() *QueryService {
	return &QueryService{}
}

// QueryGameserver queries a gameserver for its current status
func (qs *QueryService) QueryGameserver(gameserver *models.Gameserver, game *models.Game) (*protocol.ServerInfo, error) {
	// Only query running servers
	if gameserver.Status != models.StatusRunning {
		return &protocol.ServerInfo{
			Online: false,
		}, nil
	}

	return qs.doQuery(gameserver, game)
}

// IsServerReady checks if a gameserver is responding to queries (used during startup)
func (qs *QueryService) IsServerReady(gameserver *models.Gameserver, game *models.Game) bool {
	result, err := qs.doQuery(gameserver, game)
	if err != nil {
		return false
	}
	return result.Online
}

// doQuery performs the actual query regardless of server status
func (qs *QueryService) doQuery(gameserver *models.Gameserver, game *models.Game) (*protocol.ServerInfo, error) {
	// Get the query port (preferred) or game port
	var queryPort *models.PortMapping

	// First look for a "query" port mapping
	for i := range gameserver.PortMappings {
		if gameserver.PortMappings[i].Name == "query" {
			queryPort = &gameserver.PortMappings[i]
			break
		}
	}

	// Fall back to game port if no query port found
	if queryPort == nil {
		queryPort = gameserver.GetGamePort()
	}

	if queryPort == nil || queryPort.HostPort == 0 {
		log.Warn().Str("gameserver_id", gameserver.ID).Msg("No query or game port found for gameserver")
		return &protocol.ServerInfo{
			Online: false,
		}, nil
	}

	// Use 127.0.0.1 since we're querying from the same host
	address := fmt.Sprintf("127.0.0.1:%d", queryPort.HostPort)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Query the server using the game slug
	result, err := query.Query(ctx, game.Slug, address, query.WithPlayers())
	if err != nil {
		log.Debug().Err(err).Str("gameserver_id", gameserver.ID).Str("address", address).Msg("Failed to query gameserver")
		return &protocol.ServerInfo{
			Online: false,
		}, nil
	}

	return result, nil
}

