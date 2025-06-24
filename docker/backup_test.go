package docker

import (
	"testing"
)

func TestMockDockerManager_CreateBackup(t *testing.T) {
	tests := []struct {
		name           string
		containerID    string
		gameserverName string
		shouldFail     bool
		expectError    bool
	}{
		{
			name:           "successful backup",
			containerID:    "container-123",
			gameserverName: "my-server",
			shouldFail:     false,
			expectError:    false,
		},
		{
			name:           "backup failure",
			containerID:    "container-456",
			gameserverName: "fail-server",
			shouldFail:     true,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockDockerManager()
			if tt.shouldFail {
				mock.SetShouldFail("create_backup", true)
			}

			err := mock.CreateBackup(tt.containerID, tt.gameserverName)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// Check that backup was created
				backups := mock.GetBackups(tt.containerID)
				if len(backups) != 1 {
					t.Errorf("Expected 1 backup, got %d", len(backups))
				}

				// Check backup filename format
				backup := backups[0]
				if backup != "backup-2024-01-01_12-00-00.tar.gz" {
					t.Errorf("Unexpected backup filename: %s", backup)
				}
			}
		})
	}
}

func TestMockDockerManager_RestoreBackup(t *testing.T) {
	mock := NewMockDockerManager()
	containerID := "container-123"
	backupFilename := "backup-2024-01-01_12-00-00.tar.gz"

	// Test restoring non-existent backup
	err := mock.RestoreBackup(containerID, "non-existent.tar.gz")
	if err == nil {
		t.Error("Expected error for non-existent backup")
	}

	// Create a backup first
	err = mock.CreateBackup(containerID, "test-server")
	if err != nil {
		t.Fatalf("Failed to create backup: %v", err)
	}

	// Test successful restore
	err = mock.RestoreBackup(containerID, backupFilename)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Test restore failure
	mock.SetShouldFail("restore_backup", true)
	err = mock.RestoreBackup(containerID, backupFilename)
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestMockDockerManager_CleanupOldBackups(t *testing.T) {
	tests := []struct {
		name        string
		maxBackups  int
		shouldFail  bool
		expectError bool
	}{
		{
			name:        "cleanup with limit",
			maxBackups:  2,
			shouldFail:  false,
			expectError: false,
		},
		{
			name:        "no cleanup needed",
			maxBackups:  10,
			shouldFail:  false,
			expectError: false,
		},
		{
			name:        "unlimited backups",
			maxBackups:  0,
			shouldFail:  false,
			expectError: false,
		},
		{
			name:        "cleanup failure",
			maxBackups:  1,
			shouldFail:  true,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockDockerManager()
			containerID := "container-123"

			// Create multiple backups
			for i := 0; i < 5; i++ {
				err := mock.CreateBackup(containerID, "test-server")
				if err != nil {
					t.Fatalf("Failed to create backup %d: %v", i, err)
				}
			}

			// Verify we have 5 backups
			backups := mock.GetBackups(containerID)
			if len(backups) != 5 {
				t.Fatalf("Expected 5 backups before cleanup, got %d", len(backups))
			}

			if tt.shouldFail {
				mock.SetShouldFail("cleanup_backups", true)
			}

			err := mock.CleanupOldBackups(containerID, tt.maxBackups)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// Check backup count after cleanup
				backups = mock.GetBackups(containerID)
				expectedCount := tt.maxBackups
				if tt.maxBackups == 0 || tt.maxBackups > 5 {
					expectedCount = 5 // No cleanup should occur
				}

				if len(backups) != expectedCount {
					t.Errorf("Expected %d backups after cleanup, got %d", expectedCount, len(backups))
				}
			}
		})
	}
}

func TestBackupLifecycle(t *testing.T) {
	mock := NewMockDockerManager()
	containerID := "lifecycle-container"
	gameserverName := "test-server"

	// 1. Initially no backups
	backups := mock.GetBackups(containerID)
	if len(backups) != 0 {
		t.Errorf("Expected 0 initial backups, got %d", len(backups))
	}

	// 2. Create first backup
	err := mock.CreateBackup(containerID, gameserverName)
	if err != nil {
		t.Fatalf("Failed to create first backup: %v", err)
	}

	backups = mock.GetBackups(containerID)
	if len(backups) != 1 {
		t.Errorf("Expected 1 backup after first creation, got %d", len(backups))
	}

	// 3. Create second backup
	err = mock.CreateBackup(containerID, gameserverName)
	if err != nil {
		t.Fatalf("Failed to create second backup: %v", err)
	}

	backups = mock.GetBackups(containerID)
	if len(backups) != 2 {
		t.Errorf("Expected 2 backups after second creation, got %d", len(backups))
	}

	// 4. Test restore with existing backup
	backupFilename := backups[0]
	err = mock.RestoreBackup(containerID, backupFilename)
	if err != nil {
		t.Errorf("Failed to restore backup: %v", err)
	}

	// 5. Create more backups for cleanup test
	for i := 0; i < 3; i++ {
		err = mock.CreateBackup(containerID, gameserverName)
		if err != nil {
			t.Fatalf("Failed to create backup %d: %v", i+3, err)
		}
	}

	backups = mock.GetBackups(containerID)
	if len(backups) != 5 {
		t.Errorf("Expected 5 backups before cleanup, got %d", len(backups))
	}

	// 6. Cleanup to keep only 3 backups
	err = mock.CleanupOldBackups(containerID, 3)
	if err != nil {
		t.Errorf("Failed to cleanup backups: %v", err)
	}

	backups = mock.GetBackups(containerID)
	if len(backups) != 3 {
		t.Errorf("Expected 3 backups after cleanup, got %d", len(backups))
	}
}

func TestBackupFilenameParsing(t *testing.T) {
	mock := NewMockDockerManager()
	containerID := "test-container"

	// Create a backup
	err := mock.CreateBackup(containerID, "test-server")
	if err != nil {
		t.Fatalf("Failed to create backup: %v", err)
	}

	backups := mock.GetBackups(containerID)
	if len(backups) != 1 {
		t.Fatalf("Expected 1 backup, got %d", len(backups))
	}

	filename := backups[0]

	// Check filename format: backup-YYYY-MM-DD_HH-MM-SS.tar.gz
	if len(filename) != 33 { // "backup-2024-01-01_12-00-00.tar.gz" = 33 chars
		t.Errorf("Unexpected backup filename length: %d (filename: %s)", len(filename), filename)
	}

	if filename[:7] != "backup-" {
		t.Errorf("Backup filename should start with 'backup-', got: %s", filename[:7])
	}

	if filename[len(filename)-7:] != ".tar.gz" {
		t.Errorf("Backup filename should end with '.tar.gz', got: %s", filename[len(filename)-7:])
	}
}

func TestMultipleContainerBackups(t *testing.T) {
	mock := NewMockDockerManager()

	containers := []string{"container-1", "container-2", "container-3"}

	// Create backups for each container
	for i, containerID := range containers {
		for j := 0; j < i+1; j++ { // Create 1, 2, 3 backups respectively
			err := mock.CreateBackup(containerID, "server-"+containerID)
			if err != nil {
				t.Errorf("Failed to create backup for %s: %v", containerID, err)
			}
		}
	}

	// Verify backup counts
	for i, containerID := range containers {
		backups := mock.GetBackups(containerID)
		expectedCount := i + 1
		if len(backups) != expectedCount {
			t.Errorf("Container %s: expected %d backups, got %d", containerID, expectedCount, len(backups))
		}
	}

	// Cleanup one container's backups
	err := mock.CleanupOldBackups("container-3", 2)
	if err != nil {
		t.Errorf("Failed to cleanup backups: %v", err)
	}

	// Verify only container-3 was affected
	backups1 := mock.GetBackups("container-1")
	backups2 := mock.GetBackups("container-2")
	backups3 := mock.GetBackups("container-3")

	if len(backups1) != 1 {
		t.Errorf("Container-1 should still have 1 backup, got %d", len(backups1))
	}
	if len(backups2) != 2 {
		t.Errorf("Container-2 should still have 2 backups, got %d", len(backups2))
	}
	if len(backups3) != 2 {
		t.Errorf("Container-3 should have 2 backups after cleanup, got %d", len(backups3))
	}
}
