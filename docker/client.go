package docker

import (
	"time"

	"github.com/docker/docker/client"
	"github.com/rs/zerolog/log"

	"0xkowalskidev/gameservers/models"
)

// DockerError is an alias for models.OperationError
type DockerError = models.OperationError

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

// wrapErr creates a DockerError with the given operation, message, and wrapped error
func (d *DockerManager) wrapErr(op, msg string, err error) error {
	if err == nil {
		return nil
	}
	return &DockerError{Op: op, Msg: msg, Err: err}
}

// Ensure DockerManager implements the interface
var _ models.DockerManagerInterface = (*DockerManager)(nil)
