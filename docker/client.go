package docker

import (
	"fmt"
	"time"

	"github.com/docker/docker/client"
	"github.com/rs/zerolog/log"

	"0xkowalskidev/gameservers/models"
)

// DockerError represents an error that occurred during a Docker operation
type DockerError struct {
	Op  string
	Msg string
	Err error
}

func (e *DockerError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("docker %s: %s: %v", e.Op, e.Msg, e.Err)
	}
	return fmt.Sprintf("docker %s: %s", e.Op, e.Msg)
}

// DockerManager manages Docker operations for gameservers
type DockerManager struct {
	client           *client.Client
	namespace        string
	stopTimeout      time.Duration
}

// NewDockerManager creates a new Docker manager instance
func NewDockerManager(dockerSocket, namespace string, stopTimeout time.Duration) (*DockerManager, error) {
	log.Info().Msg("Connecting to Docker daemon")

	opts := []client.Opt{
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	}

	// Use custom docker socket if provided
	if dockerSocket != "" {
		opts = append(opts, client.WithHost(dockerSocket))
		log.Info().Str("socket", dockerSocket).Msg("Using custom Docker socket")
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Docker client")
		return nil, &DockerError{
			Op:  "connect",
			Msg: "failed to create Docker client",
			Err: err,
		}
	}

	log.Info().Str("namespace", namespace).Dur("stop_timeout", stopTimeout).Msg("Docker client connected successfully")
	return &DockerManager{
		client:      cli,
		namespace:   namespace,
		stopTimeout: stopTimeout,
	}, nil
}

// Ensure DockerManager implements the interface
var _ models.DockerManagerInterface = (*DockerManager)(nil)
