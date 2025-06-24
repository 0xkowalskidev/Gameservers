package docker

import (
	"testing"
)

func TestMockDockerManager_CreateVolume(t *testing.T) {
	tests := []struct {
		name        string
		volumeName  string
		shouldFail  bool
		expectError bool
	}{
		{
			name:        "successful volume creation",
			volumeName:  "test-volume",
			shouldFail:  false,
			expectError: false,
		},
		{
			name:        "volume creation failure",
			volumeName:  "fail-volume",
			shouldFail:  true,
			expectError: true,
		},
		{
			name:        "gameserver volume creation",
			volumeName:  "gameservers-minecraft-data",
			shouldFail:  false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockDockerManager()
			if tt.shouldFail {
				mock.SetShouldFail("create_volume", true)
			}

			err := mock.CreateVolume(tt.volumeName)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// Check that volume was created
				if !mock.HasVolume(tt.volumeName) {
					t.Error("Volume was not created in mock")
				}
			}
		})
	}
}

func TestMockDockerManager_RemoveVolume(t *testing.T) {
	mock := NewMockDockerManager()
	volumeName := "test-volume"

	// Create volume first
	err := mock.CreateVolume(volumeName)
	if err != nil {
		t.Fatalf("Failed to create volume: %v", err)
	}

	// Verify it exists
	if !mock.HasVolume(volumeName) {
		t.Fatal("Volume should exist before removal")
	}

	// Remove volume
	err = mock.RemoveVolume(volumeName)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify it's gone
	if mock.HasVolume(volumeName) {
		t.Error("Volume should have been removed")
	}
}

func TestMockDockerManager_RemoveVolume_Failure(t *testing.T) {
	mock := NewMockDockerManager()
	mock.SetShouldFail("remove_volume", true)

	err := mock.RemoveVolume("test-volume")
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestMockDockerManager_GetVolumeInfo(t *testing.T) {
	mock := NewMockDockerManager()
	volumeName := "test-volume"

	// Test non-existent volume
	_, err := mock.GetVolumeInfo("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent volume")
	}

	// Create volume
	err = mock.CreateVolume(volumeName)
	if err != nil {
		t.Fatalf("Failed to create volume: %v", err)
	}

	// Get volume info
	volumeInfo, err := mock.GetVolumeInfo(volumeName)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify volume info
	if volumeInfo.Name != volumeName {
		t.Errorf("Expected volume name %s, got %s", volumeName, volumeInfo.Name)
	}

	if volumeInfo.Driver != "local" {
		t.Errorf("Expected driver 'local', got %s", volumeInfo.Driver)
	}

	expectedMountPoint := "/var/lib/docker/volumes/" + volumeName + "/_data"
	if volumeInfo.MountPoint != expectedMountPoint {
		t.Errorf("Expected mount point %s, got %s", expectedMountPoint, volumeInfo.MountPoint)
	}

	// Check labels
	if volumeInfo.Labels["gameserver.managed"] != "true" {
		t.Error("Expected gameserver.managed label to be 'true'")
	}
}

func TestMockDockerManager_GetVolumeInfo_Failure(t *testing.T) {
	mock := NewMockDockerManager()
	mock.SetShouldFail("get_volume_info", true)

	_, err := mock.GetVolumeInfo("test-volume")
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestVolumeNameGeneration(t *testing.T) {
	mock := NewMockDockerManager()
	
	// Test the volume name generation function
	// Note: This would be the getVolumeNameForServer method in the real implementation
	expectedVolumeName := "gameservers-my-minecraft-server-data"
	
	err := mock.CreateVolume(expectedVolumeName)
	if err != nil {
		t.Fatalf("Failed to create volume: %v", err)
	}
	
	volumeInfo, err := mock.GetVolumeInfo(expectedVolumeName)
	if err != nil {
		t.Errorf("Failed to get volume info: %v", err)
	}
	
	if volumeInfo.Name != expectedVolumeName {
		t.Errorf("Volume name mismatch: expected %s, got %s", expectedVolumeName, volumeInfo.Name)
	}
}

func TestVolumeLifecycle(t *testing.T) {
	mock := NewMockDockerManager()
	volumeName := "lifecycle-test-volume"
	
	// 1. Volume should not exist initially
	if mock.HasVolume(volumeName) {
		t.Error("Volume should not exist initially")
	}
	
	// 2. Create volume
	err := mock.CreateVolume(volumeName)
	if err != nil {
		t.Fatalf("Failed to create volume: %v", err)
	}
	
	// 3. Volume should exist
	if !mock.HasVolume(volumeName) {
		t.Error("Volume should exist after creation")
	}
	
	// 4. Get volume info
	volumeInfo, err := mock.GetVolumeInfo(volumeName)
	if err != nil {
		t.Fatalf("Failed to get volume info: %v", err)
	}
	
	if volumeInfo == nil {
		t.Fatal("Volume info should not be nil")
	}
	
	// 5. Remove volume
	err = mock.RemoveVolume(volumeName)
	if err != nil {
		t.Fatalf("Failed to remove volume: %v", err)
	}
	
	// 6. Volume should not exist
	if mock.HasVolume(volumeName) {
		t.Error("Volume should not exist after removal")
	}
	
	// 7. Getting info should fail
	_, err = mock.GetVolumeInfo(volumeName)
	if err == nil {
		t.Error("Expected error when getting info for removed volume")
	}
}

func TestMultipleVolumes(t *testing.T) {
	mock := NewMockDockerManager()
	
	volumes := []string{
		"gameservers-server1-data",
		"gameservers-server2-data", 
		"gameservers-server3-data",
	}
	
	// Create all volumes
	for _, volumeName := range volumes {
		err := mock.CreateVolume(volumeName)
		if err != nil {
			t.Errorf("Failed to create volume %s: %v", volumeName, err)
		}
	}
	
	// Verify all exist
	for _, volumeName := range volumes {
		if !mock.HasVolume(volumeName) {
			t.Errorf("Volume %s should exist", volumeName)
		}
		
		volumeInfo, err := mock.GetVolumeInfo(volumeName)
		if err != nil {
			t.Errorf("Failed to get info for volume %s: %v", volumeName, err)
		}
		
		if volumeInfo.Name != volumeName {
			t.Errorf("Volume name mismatch for %s", volumeName)
		}
	}
	
	// Remove middle volume
	err := mock.RemoveVolume(volumes[1])
	if err != nil {
		t.Errorf("Failed to remove volume %s: %v", volumes[1], err)
	}
	
	// Check states
	if !mock.HasVolume(volumes[0]) {
		t.Error("First volume should still exist")
	}
	if mock.HasVolume(volumes[1]) {
		t.Error("Second volume should be removed")
	}
	if !mock.HasVolume(volumes[2]) {
		t.Error("Third volume should still exist")
	}
}