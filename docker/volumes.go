package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/volume"
	"github.com/rs/zerolog/log"

	"0xkowalskidev/gameservers/models"
)

// CreateVolume creates a Docker volume
func (d *DockerManager) CreateVolume(volumeName string) error {
	ctx := context.Background()

	// Check if volume already exists
	_, err := d.client.VolumeInspect(ctx, volumeName)
	if err == nil {
		// Volume already exists, no need to create
		log.Debug().Str("volume", volumeName).Msg("Volume already exists")
		return nil
	}

	log.Info().Str("volume", volumeName).Msg("Creating Docker volume")

	_, err = d.client.VolumeCreate(ctx, volume.CreateOptions{
		Name: volumeName,
		Labels: map[string]string{
			"gameserver.managed": "true",
		},
	})
	if err != nil {
		return &DockerError{
			Op:  "create_volume",
			Msg: fmt.Sprintf("failed to create volume %s", volumeName),
			Err: err,
		}
	}

	log.Info().Str("volume", volumeName).Msg("Successfully created Docker volume")
	return nil
}

// RemoveVolume removes a Docker volume
func (d *DockerManager) RemoveVolume(volumeName string) error {
	ctx := context.Background()

	log.Info().Str("volume", volumeName).Msg("Removing Docker volume")

	err := d.client.VolumeRemove(ctx, volumeName, true) // force=true
	if err != nil {
		return &DockerError{
			Op:  "remove_volume",
			Msg: fmt.Sprintf("failed to remove volume %s", volumeName),
			Err: err,
		}
	}

	log.Info().Str("volume", volumeName).Msg("Successfully removed Docker volume")
	return nil
}

// GetVolumeNameForServer generates a volume name for a gameserver
func (d *DockerManager) GetVolumeNameForServer(server *models.Gameserver) string {
	return fmt.Sprintf("%s-%s-data", d.namespace, server.Name)
}

// GetVolumeInfo returns information about a Docker volume
func (d *DockerManager) GetVolumeInfo(volumeName string) (*models.VolumeInfo, error) {
	ctx := context.Background()

	vol, err := d.client.VolumeInspect(ctx, volumeName)
	if err != nil {
		return nil, &DockerError{
			Op:  "inspect_volume",
			Msg: fmt.Sprintf("failed to inspect volume %s", volumeName),
			Err: err,
		}
	}

	return &models.VolumeInfo{
		Name:       vol.Name,
		MountPoint: vol.Mountpoint,
		Driver:     vol.Driver,
		CreatedAt:  vol.CreatedAt,
		Labels:     vol.Labels,
	}, nil
}
