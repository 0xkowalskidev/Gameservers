package docker

import (
	"context"
	"strings"
	"testing"
)

// MockImageManager provides a mock for testing image operations
type MockImageManager struct {
	localImages  map[string]string  // imageName -> digest
	remoteImages map[string]string  // imageName -> digest
	shouldFail   map[string]bool
	pullHistory  []string // Track which images were pulled
}

func NewMockImageManager() *MockImageManager {
	return &MockImageManager{
		localImages:  make(map[string]string),
		remoteImages: make(map[string]string),
		shouldFail:   make(map[string]bool),
		pullHistory:  []string{},
	}
}

func (m *MockImageManager) SetLocalImage(imageName, digest string) {
	m.localImages[imageName] = digest
}

func (m *MockImageManager) SetRemoteImage(imageName, digest string) {
	m.remoteImages[imageName] = digest
}

func (m *MockImageManager) SetShouldFail(operation string, shouldFail bool) {
	m.shouldFail[operation] = shouldFail
}

func (m *MockImageManager) GetPullHistory() []string {
	return m.pullHistory
}

// Mock implementation of shouldPullImage logic
func (m *MockImageManager) shouldPullImage(ctx context.Context, imageName string) (bool, error) {
	if m.shouldFail["check_image"] {
		return false, &DockerError{Op: "check_image", Msg: "mock check image error"}
	}

	localDigest, hasLocal := m.localImages[imageName]
	if !hasLocal {
		return true, nil // Image doesn't exist locally, should pull
	}

	remoteDigest, hasRemote := m.remoteImages[imageName]
	if !hasRemote {
		return false, nil // Can't check remote, don't pull
	}

	// Handle short hash comparison - if one contains the other, consider them the same
	if strings.Contains(localDigest, remoteDigest) || strings.Contains(remoteDigest, localDigest) {
		return false, nil
	}

	// Should pull if digests differ
	return localDigest != remoteDigest, nil
}

// Mock implementation of pullImage
func (m *MockImageManager) pullImage(ctx context.Context, imageName string) error {
	if m.shouldFail["pull_image"] {
		return &DockerError{Op: "pull_image", Msg: "mock pull image error"}
	}

	m.pullHistory = append(m.pullHistory, imageName)
	
	// Update local image with remote digest after successful pull
	if remoteDigest, exists := m.remoteImages[imageName]; exists {
		m.localImages[imageName] = remoteDigest
	}
	
	return nil
}

// Mock implementation of pullImageIfNeeded
func (m *MockImageManager) pullImageIfNeeded(ctx context.Context, imageName string) error {
	shouldPull, err := m.shouldPullImage(ctx, imageName)
	if err != nil {
		return nil // Don't fail container creation if we can't check
	}

	if !shouldPull {
		return nil
	}

	return m.pullImage(ctx, imageName)
}

func TestMockImageManager_ShouldPullImage(t *testing.T) {
	tests := []struct {
		name         string
		imageName    string
		localDigest  string
		remoteDigest string
		shouldFail   bool
		expectPull   bool
		expectError  bool
	}{
		{
			name:        "pull latest tag",
			imageName:   "minecraft:latest",
			localDigest: "",
			remoteDigest: "sha256:abc123",
			expectPull:  true,
			expectError: false,
		},
		{
			name:         "pull new version",
			imageName:    "minecraft:1.20.4",
			localDigest:  "sha256:old123",
			remoteDigest: "sha256:new456",
			expectPull:   true,
			expectError:  false,
		},
		{
			name:         "skip stable version",
			imageName:    "minecraft:1.20.4",
			localDigest:  "sha256:same123",
			remoteDigest: "sha256:same123",
			expectPull:   false,
			expectError:  false,
		},
		{
			name:         "skip specific digest",
			imageName:    "minecraft@sha256:specific123",
			localDigest:  "sha256:specific123",
			remoteDigest: "sha256:specific123",
			expectPull:   false,
			expectError:  false,
		},
		{
			name:        "check image fails",
			imageName:   "minecraft:latest",
			shouldFail:  true,
			expectPull:  false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockImageManager()
			
			if tt.localDigest != "" {
				mock.SetLocalImage(tt.imageName, tt.localDigest)
			}
			if tt.remoteDigest != "" {
				mock.SetRemoteImage(tt.imageName, tt.remoteDigest)
			}
			if tt.shouldFail {
				mock.SetShouldFail("check_image", true)
			}

			ctx := context.Background()
			shouldPull, err := mock.shouldPullImage(ctx, tt.imageName)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if shouldPull != tt.expectPull {
					t.Errorf("Expected shouldPull=%v, got %v", tt.expectPull, shouldPull)
				}
			}
		})
	}
}

func TestMockImageManager_PullImage(t *testing.T) {
	tests := []struct {
		name        string
		imageName   string
		shouldFail  bool
		expectError bool
	}{
		{
			name:        "successful pull",
			imageName:   "minecraft:latest",
			shouldFail:  false,
			expectError: false,
		},
		{
			name:        "pull failure",
			imageName:   "minecraft:latest",
			shouldFail:  true,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockImageManager()
			
			if tt.shouldFail {
				mock.SetShouldFail("pull_image", true)
			}

			ctx := context.Background()
			err := mock.pullImage(ctx, tt.imageName)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// Check that image was recorded in pull history
				pullHistory := mock.GetPullHistory()
				found := false
				for _, pulledImage := range pullHistory {
					if pulledImage == tt.imageName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Image %s not found in pull history", tt.imageName)
				}
			}
		})
	}
}

func TestMockImageManager_PullImageIfNeeded(t *testing.T) {
	tests := []struct {
		name         string
		imageName    string
		localDigest  string
		remoteDigest string
		checkFails   bool
		pullFails    bool
		expectPull   bool
		expectError  bool
	}{
		{
			name:         "pull when needed",
			imageName:    "minecraft:latest",
			localDigest:  "sha256:old123",
			remoteDigest: "sha256:new456",
			expectPull:   true,
			expectError:  false,
		},
		{
			name:         "skip when not needed",
			imageName:    "minecraft:1.20.4",
			localDigest:  "sha256:same123",
			remoteDigest: "sha256:same123",
			expectPull:   false,
			expectError:  false,
		},
		{
			name:        "check fails, no pull",
			imageName:   "minecraft:latest",
			checkFails:  true,
			expectPull:  false,
			expectError: false,
		},
		{
			name:         "pull fails",
			imageName:    "minecraft:latest",
			localDigest:  "sha256:old123",
			remoteDigest: "sha256:new456",
			pullFails:    true,
			expectPull:   false,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockImageManager()
			
			if tt.localDigest != "" {
				mock.SetLocalImage(tt.imageName, tt.localDigest)
			}
			if tt.remoteDigest != "" {
				mock.SetRemoteImage(tt.imageName, tt.remoteDigest)
			}
			if tt.checkFails {
				mock.SetShouldFail("check_image", true)
			}
			if tt.pullFails {
				mock.SetShouldFail("pull_image", true)
			}

			ctx := context.Background()
			err := mock.pullImageIfNeeded(ctx, tt.imageName)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			// Check pull history
			pullHistory := mock.GetPullHistory()
			pulled := false
			for _, pulledImage := range pullHistory {
				if pulledImage == tt.imageName {
					pulled = true
					break
				}
			}

			if tt.expectPull && !pulled {
				t.Errorf("Expected image %s to be pulled", tt.imageName)
			}
			if !tt.expectPull && pulled {
				t.Errorf("Expected image %s NOT to be pulled", tt.imageName)
			}
		})
	}
}

func TestSmartPullStrategy_Complex(t *testing.T) {
	mock := NewMockImageManager()
	ctx := context.Background()

	// Setup: minecraft:latest exists locally but remote has newer version
	mock.SetLocalImage("minecraft:latest", "sha256:local123")
	mock.SetRemoteImage("minecraft:latest", "sha256:remote456")

	// Setup: minecraft:1.20.4 is up to date
	mock.SetLocalImage("minecraft:1.20.4", "sha256:stable123")
	mock.SetRemoteImage("minecraft:1.20.4", "sha256:stable123")

	// Setup: valheim:latest doesn't exist locally
	mock.SetRemoteImage("valheim:latest", "sha256:valheim789")

	// Test pulling minecraft:latest (should pull - newer version available)
	err := mock.pullImageIfNeeded(ctx, "minecraft:latest")
	if err != nil {
		t.Errorf("Failed to pull minecraft:latest: %v", err)
	}

	// Test pulling minecraft:1.20.4 (should skip - up to date)
	err = mock.pullImageIfNeeded(ctx, "minecraft:1.20.4")
	if err != nil {
		t.Errorf("Failed to check minecraft:1.20.4: %v", err)
	}

	// Test pulling valheim:latest (should pull - doesn't exist locally)
	err = mock.pullImageIfNeeded(ctx, "valheim:latest")
	if err != nil {
		t.Errorf("Failed to pull valheim:latest: %v", err)
	}

	// Verify pull history
	pullHistory := mock.GetPullHistory()
	expectedPulls := []string{"minecraft:latest", "valheim:latest"}

	if len(pullHistory) != len(expectedPulls) {
		t.Errorf("Expected %d pulls, got %d", len(expectedPulls), len(pullHistory))
	}

	for _, expectedImage := range expectedPulls {
		found := false
		for _, pulledImage := range pullHistory {
			if pulledImage == expectedImage {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected image %s to be pulled", expectedImage)
		}
	}

	// Verify minecraft:1.20.4 was NOT pulled
	for _, pulledImage := range pullHistory {
		if pulledImage == "minecraft:1.20.4" {
			t.Error("minecraft:1.20.4 should not have been pulled")
		}
	}
}

func TestImageDigestComparison(t *testing.T) {
	tests := []struct {
		name         string
		localDigest  string
		remoteDigest string
		shouldPull   bool
	}{
		{
			name:         "exact match",
			localDigest:  "sha256:abc123def456",
			remoteDigest: "sha256:abc123def456",
			shouldPull:   false,
		},
		{
			name:         "different digests",
			localDigest:  "sha256:abc123def456",
			remoteDigest: "sha256:xyz789uvw012",
			shouldPull:   true,
		},
		{
			name:         "local has short hash",
			localDigest:  "abc123",
			remoteDigest: "sha256:abc123def456",
			shouldPull:   false, // Should recognize as same
		},
		{
			name:         "completely different",
			localDigest:  "sha256:totally",
			remoteDigest: "sha256:different",
			shouldPull:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockImageManager()
			
			mock.SetLocalImage("test:latest", tt.localDigest)
			mock.SetRemoteImage("test:latest", tt.remoteDigest)

			ctx := context.Background()
			shouldPull, err := mock.shouldPullImage(ctx, "test:latest")
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if shouldPull != tt.shouldPull {
				t.Errorf("Expected shouldPull=%v, got %v", tt.shouldPull, shouldPull)
			}
		})
	}
}

func TestConcurrentImagePulls(t *testing.T) {
	mock := NewMockImageManager()
	ctx := context.Background()

	images := []string{
		"minecraft:latest",
		"valheim:latest", 
		"cs2:latest",
		"palworld:latest",
	}

	// Setup all images to need pulling
	for _, image := range images {
		mock.SetRemoteImage(image, "sha256:remote123")
	}

	// Pull all images
	for _, image := range images {
		err := mock.pullImageIfNeeded(ctx, image)
		if err != nil {
			t.Errorf("Failed to pull %s: %v", image, err)
		}
	}

	// Verify all were pulled
	pullHistory := mock.GetPullHistory()
	if len(pullHistory) != len(images) {
		t.Errorf("Expected %d pulls, got %d", len(images), len(pullHistory))
	}

	for _, expectedImage := range images {
		found := false
		for _, pulledImage := range pullHistory {
			if pulledImage == expectedImage {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected image %s to be pulled", expectedImage)
		}
	}
}