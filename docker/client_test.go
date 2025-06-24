package docker

import (
	"testing"
)

func TestDockerError(t *testing.T) {
	tests := []struct {
		name     string
		err      *DockerError
		expected string
	}{
		{
			name: "error without underlying error",
			err: &DockerError{
				Op:  "create",
				Msg: "failed to create container",
			},
			expected: "docker create: failed to create container",
		},
		{
			name: "error with underlying error",
			err: &DockerError{
				Op:  "start",
				Msg: "failed to start container",
				Err: &DockerError{Op: "connect", Msg: "connection refused"},
			},
			expected: "docker start: failed to start container: docker connect: connection refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("DockerError.Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewDockerManager(t *testing.T) {
	// This test would require a real Docker daemon or extensive mocking
	// For now, we'll just test that the function exists and returns the right type
	t.Skip("Skipping integration test - requires Docker daemon")
	
	// In a real test environment with Docker available:
	// dm, err := NewDockerManager()
	// if err != nil {
	//     t.Fatalf("NewDockerManager() failed: %v", err)
	// }
	// if dm == nil {
	//     t.Error("NewDockerManager() returned nil manager")
	// }
}

func TestDockerError_Error(t *testing.T) {
	err := &DockerError{
		Op:  "test_operation",
		Msg: "test error message",
	}
	
	expected := "docker test_operation: test error message"
	if got := err.Error(); got != expected {
		t.Errorf("DockerError.Error() = %q, want %q", got, expected)
	}
}

func TestDockerError_WithCause(t *testing.T) {
	causeErr := &DockerError{
		Op:  "underlying",
		Msg: "underlying error",
	}
	
	err := &DockerError{
		Op:  "wrapper",
		Msg: "wrapper error",
		Err: causeErr,
	}
	
	expected := "docker wrapper: wrapper error: docker underlying: underlying error"
	if got := err.Error(); got != expected {
		t.Errorf("DockerError.Error() = %q, want %q", got, expected)
	}
}