package docker

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types/image"
	"github.com/rs/zerolog/log"
)

// pullImageIfNeeded implements a smart pull strategy that only pulls if there's a newer image
func (d *DockerManager) pullImageIfNeeded(ctx context.Context, imageName string) error {
	log.Debug().Str("image", imageName).Msg("Checking if image pull is needed")
	
	// Check if we should pull the image
	shouldPull, err := d.shouldPullImage(ctx, imageName)
	if err != nil {
		log.Warn().Err(err).Str("image", imageName).Msg("Failed to check if image should be pulled, skipping pull")
		return nil // Don't fail container creation if we can't check
	}
	
	if !shouldPull {
		log.Debug().Str("image", imageName).Msg("Local image is up to date, skipping pull")
		return nil
	}
	
	log.Info().Str("image", imageName).Msg("Pulling newer image version")
	return d.pullImage(ctx, imageName)
}

// shouldPullImage determines if we should pull the image based on comparing local and remote digests
func (d *DockerManager) shouldPullImage(ctx context.Context, imageName string) (bool, error) {
	// First, check if the image exists locally
	localImage, err := d.client.ImageInspect(ctx, imageName)
	if err != nil {
		// Image doesn't exist locally, should pull
		log.Debug().Str("image", imageName).Msg("Image not found locally, should pull")
		return true, nil
	}
	
	// Get the local image digest
	var localDigest string
	if len(localImage.RepoDigests) > 0 {
		localDigest = localImage.RepoDigests[0]
		log.Debug().Str("image", imageName).Str("local_digest", localDigest).Msg("Found local image digest")
	} else {
		// No repo digest available (probably built locally), check by ID
		log.Debug().Str("image", imageName).Str("local_id", localImage.ID).Msg("No repo digest found, using image ID")
		localDigest = localImage.ID
	}
	
	// Get remote image digest
	remoteDigest, err := d.getRemoteImageDigest(ctx, imageName)
	if err != nil {
		log.Warn().Err(err).Str("image", imageName).Msg("Failed to get remote image digest, skipping pull")
		return false, nil // Don't pull if we can't check remote
	}
	
	// Compare digests
	needsPull := !strings.Contains(localDigest, remoteDigest) && localDigest != remoteDigest
	log.Debug().
		Str("image", imageName).
		Str("local_digest", localDigest).
		Str("remote_digest", remoteDigest).
		Bool("needs_pull", needsPull).
		Msg("Compared image digests")
		
	return needsPull, nil
}

// getRemoteImageDigest gets the digest of the remote image without pulling it
func (d *DockerManager) getRemoteImageDigest(ctx context.Context, imageName string) (string, error) {
	// Use Docker's built-in registry client to get image info
	// This approach uses the Docker daemon's registry authentication
	encodedAuth := base64.URLEncoding.EncodeToString([]byte("{}"))
	
	// Get image distribution inspect (this gets manifest info without pulling)
	resp, err := d.client.DistributionInspect(ctx, imageName, encodedAuth)
	if err != nil {
		return "", fmt.Errorf("failed to inspect remote image: %w", err)
	}
	
	digest := resp.Descriptor.Digest.String()
	log.Debug().Str("image", imageName).Str("remote_digest", digest).Msg("Retrieved remote image digest")
	
	return digest, nil
}

// pullImage pulls the specified image
func (d *DockerManager) pullImage(ctx context.Context, imageName string) error {
	log.Info().Str("image", imageName).Msg("Pulling Docker image")
	
	// Use default authentication (will use Docker daemon's auth)
	encodedAuth := base64.URLEncoding.EncodeToString([]byte("{}"))
	
	reader, err := d.client.ImagePull(ctx, imageName, image.PullOptions{
		RegistryAuth: encodedAuth,
	})
	if err != nil {
		return &DockerError{
			Op:  "pull",
			Msg: fmt.Sprintf("failed to pull image %s", imageName),
			Err: err,
		}
	}
	defer reader.Close()
	
	// Read the pull output for logging
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, reader)
	if err != nil {
		log.Warn().Err(err).Str("image", imageName).Msg("Failed to read pull output")
	} else {
		// Log pull progress (Docker returns JSON-lines format)
		output := buf.String()
		if strings.TrimSpace(output) != "" {
			log.Debug().Str("image", imageName).Str("pull_output", output).Msg("Image pull completed")
		}
	}
	
	log.Info().Str("image", imageName).Msg("Successfully pulled Docker image")
	return nil
}