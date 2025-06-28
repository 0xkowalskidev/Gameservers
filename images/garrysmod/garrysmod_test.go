package garrysmod_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/0xkowalskidev/gameserverquery/query"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// Test timeouts
	buildTimeout    = 10 * time.Minute // GMod server download takes longer
	startupTimeout  = 5 * time.Minute  // GMod takes longer to start than Minecraft
	shutdownTimeout = 30 * time.Second

	// Default test values
	testServerName   = "Test Garry's Mod Server"
	testRconPassword = "test123"
)

// getDockerfileContext returns the absolute path to the garrysmod image directory
func getDockerfileContext() string {
	wd, _ := os.Getwd()
	// We're already in the garrysmod directory, so return current directory
	return wd
}

// readLogsToString reads an io.Reader and returns its content as a string
func readLogsToString(reader io.Reader) (string, error) {
	var builder strings.Builder
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		builder.WriteString(scanner.Text())
		builder.WriteString("\n")
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return builder.String(), nil
}

// cleanupContainer ensures proper container cleanup with timeout and force removal
func cleanupContainer(t *testing.T, container testcontainers.Container) {
	if container == nil {
		return
	}

	t.Helper()

	// Use a separate context for cleanup to avoid cancellation issues
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// First try graceful termination
	if err := container.Terminate(cleanupCtx); err != nil {
		t.Logf("Warning: Failed to terminate container gracefully: %v", err)

		// If graceful termination fails, try forceful stop
		stopTimeout := 10 * time.Second
		if err := container.Stop(cleanupCtx, &stopTimeout); err != nil {
			t.Logf("Warning: Failed to stop container forcefully: %v", err)

			// Last resort: try to get container info for debugging
			if state, stateErr := container.State(cleanupCtx); stateErr == nil {
				t.Logf("Container state during cleanup failure: Running=%v, Status=%s",
					state.Running, state.Status)
			}
		} else {
			t.Logf("Container stopped successfully after terminate failed")
		}
	} else {
		t.Logf("Container terminated successfully")
	}
}

// setupTest performs pre-test cleanup to ensure clean environment
func setupTest(t *testing.T) {
	t.Helper()

	// Set up test-specific cleanup that runs even if test panics
	t.Cleanup(func() {
		// This runs after each test regardless of outcome
		cleanupTestResources(t)
	})
}

// cleanupTestResources performs comprehensive cleanup of test resources
func cleanupTestResources(t *testing.T) {
	t.Helper()

	// This function can be called to clean up any leaked resources
	// In practice, Testcontainers with Ryuk should handle most cleanup automatically
	// But this provides a hook for additional cleanup if needed
}

// TestGarrysModImage_Build tests that the Docker image builds successfully
func TestGarrysModImage_Build(t *testing.T) {
	setupTest(t)
	ctx := context.Background()

	// Build the image without starting a container
	_, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:       getDockerfileContext(),
				Dockerfile:    "Dockerfile",
				PrintBuildLog: true,
			},
		},
		Started: false, // Just build, don't start
	})

	if err != nil {
		t.Fatalf("Failed to build Garry's Mod Docker image: %v", err)
	}
}

// TestGarrysModImage_BasicStartup tests basic container startup with minimal configuration
func TestGarrysModImage_BasicStartup(t *testing.T) {
	setupTest(t)
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"27015/udp", "27015/tcp"},
			Env: map[string]string{
				"NAME":          testServerName,
				"RCON_PASSWORD": testRconPassword,
			},
			WaitingFor: wait.ForLog("Garry's Mod server started with PID").WithStartupTimeout(startupTimeout),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start Garry's Mod container: %v", err)
	}

	// Ensure cleanup happens regardless of test outcome
	defer cleanupContainer(t, container)

	// Verify container is running
	state, err := container.State(ctx)
	if err != nil {
		t.Fatalf("Failed to get container state: %v", err)
	}
	if !state.Running {
		t.Fatal("Container should be running")
	}

	// Get the mapped port for testing - we need the UDP port for query
	mappedPort, err := container.MappedPort(ctx, "27015/udp")
	if err != nil {
		t.Fatalf("Failed to get mapped UDP port: %v", err)
	}

	// Verify server is responding using GameserverQuery
	serverAddress := fmt.Sprintf("127.0.0.1:%s", mappedPort.Port())

	// Wait a bit for server to fully initialize
	time.Sleep(15 * time.Second)

	queryCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	serverInfo, err := query.Query(queryCtx, "garrys-mod", serverAddress)
	if err != nil {
		t.Fatalf("Failed to query Garry's Mod server: %v", err)
	}

	// Check server name matches what we set
	if !strings.Contains(serverInfo.Name, testServerName) {
		t.Errorf("Expected server name to contain '%s', got '%s'", testServerName, serverInfo.Name)
	}

	t.Logf("Server successfully started - Name: %s, Map: %s, Players: %d/%d",
		serverInfo.Name, serverInfo.Map, serverInfo.Players.Current, serverInfo.Players.Max)
}

// TestGarrysModImage_EnvironmentVariables tests that environment variables are properly applied
func TestGarrysModImage_EnvironmentVariables(t *testing.T) {
	setupTest(t)
	ctx := context.Background()

	testCases := []struct {
		name     string
		env      map[string]string
		checkFn  func(t *testing.T, container testcontainers.Container, ctx context.Context)
	}{
		{
			name: "Server name and RCON password",
			env: map[string]string{
				"NAME":          testServerName,
				"RCON_PASSWORD": testRconPassword,
			},
			checkFn: func(t *testing.T, container testcontainers.Container, ctx context.Context) {
				// Check logs for server name in launch command
				logs, err := container.Logs(ctx)
				if err != nil {
					t.Fatalf("Failed to get logs: %v", err)
				}
				defer logs.Close()

				logStr, err := readLogsToString(logs)
				if err != nil {
					t.Fatalf("Failed to read logs: %v", err)
				}

				if !strings.Contains(logStr, fmt.Sprintf(`+hostname "%s"`, testServerName)) {
					t.Errorf("Server name '%s' should be in launch command", testServerName)
				}

				if !strings.Contains(logStr, fmt.Sprintf(`+rcon_password "%s"`, testRconPassword)) {
					t.Errorf("RCON password should be in launch command")
				}
			},
		},
		{
			name: "Max players setting",
			env: map[string]string{
				"NAME":       testServerName,
				"MAXPLAYERS": "32",
			},
			checkFn: func(t *testing.T, container testcontainers.Container, ctx context.Context) {
				logs, err := container.Logs(ctx)
				if err != nil {
					t.Fatalf("Failed to get logs: %v", err)
				}
				defer logs.Close()

				logStr, err := readLogsToString(logs)
				if err != nil {
					t.Fatalf("Failed to read logs: %v", err)
				}

				if !strings.Contains(logStr, `+maxplayers "32"`) {
					t.Errorf("Max players should be set to 32")
				}
			},
		},
		{
			name: "Map and gamemode setting",
			env: map[string]string{
				"NAME":     testServerName,
				"MAP":      "gm_flatgrass",
				"GAMEMODE": "darkrp",
			},
			checkFn: func(t *testing.T, container testcontainers.Container, ctx context.Context) {
				logs, err := container.Logs(ctx)
				if err != nil {
					t.Fatalf("Failed to get logs: %v", err)
				}
				defer logs.Close()

				logStr, err := readLogsToString(logs)
				if err != nil {
					t.Fatalf("Failed to read logs: %v", err)
				}

				if !strings.Contains(logStr, `+map "gm_flatgrass"`) {
					t.Errorf("Map should be set to gm_flatgrass")
				}

				if !strings.Contains(logStr, `+gamemode "darkrp"`) {
					t.Errorf("Gamemode should be set to darkrp")
				}
			},
		},
		{
			name: "Server password",
			env: map[string]string{
				"NAME":     testServerName,
				"PASSWORD": "secretpass",
			},
			checkFn: func(t *testing.T, container testcontainers.Container, ctx context.Context) {
				logs, err := container.Logs(ctx)
				if err != nil {
					t.Fatalf("Failed to get logs: %v", err)
				}
				defer logs.Close()

				logStr, err := readLogsToString(logs)
				if err != nil {
					t.Fatalf("Failed to read logs: %v", err)
				}

				if !strings.Contains(logStr, `+sv_password "secretpass"`) {
					t.Errorf("Server password should be set")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				ContainerRequest: testcontainers.ContainerRequest{
					FromDockerfile: testcontainers.FromDockerfile{
						Context:    getDockerfileContext(),
						Dockerfile: "Dockerfile",
					},
					ExposedPorts: []string{"27015/udp", "27015/tcp"},
					Env:          tc.env,
					WaitingFor:   wait.ForLog("Garry's Mod server started with PID").WithStartupTimeout(startupTimeout),
				},
				Started: true,
			})
			if err != nil {
				t.Fatalf("Failed to start container: %v", err)
			}
			defer cleanupContainer(t, container)

			// Run the test-specific check
			tc.checkFn(t, container, ctx)
		})
	}
}

// TestGarrysModImage_RconInterface tests the RCON command interface
func TestGarrysModImage_RconInterface(t *testing.T) {
	setupTest(t)
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"27015/udp", "27015/tcp"},
			Env: map[string]string{
				"NAME":          testServerName,
				"RCON_PASSWORD": testRconPassword,
			},
			WaitingFor: wait.ForLog("Garry's Mod server started with PID").WithStartupTimeout(startupTimeout),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}
	defer cleanupContainer(t, container)

	// Wait for server to fully initialize
	time.Sleep(10 * time.Second)

	// Test sending a command through RCON
	exitCode, outputReader, err := container.Exec(ctx, []string{"/data/scripts/send-command.sh", "status"})
	if err != nil {
		t.Fatalf("Failed to send RCON command: %v", err)
	}
	if exitCode != 0 {
		output, _ := readLogsToString(outputReader)
		t.Fatalf("Failed to send RCON command, exit code: %d, output: %s", exitCode, output)
	}

	// Check that rcon-cli binary is available
	exitCode, outputReader, err = container.Exec(ctx, []string{"which", "rcon-cli"})
	if err != nil {
		t.Fatalf("Failed to check for rcon-cli: %v", err)
	}
	if exitCode != 0 {
		t.Fatal("rcon-cli should be available in the container")
	}
}

// TestGarrysModImage_PortConfiguration tests that ports are configured correctly
func TestGarrysModImage_PortConfiguration(t *testing.T) {
	setupTest(t)
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"27015/udp", "27015/tcp"},
			Env: map[string]string{
				"NAME":          testServerName,
				"RCON_PASSWORD": testRconPassword,
			},
			WaitingFor: wait.ForLog("Garry's Mod server started with PID").WithStartupTimeout(startupTimeout),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}
	defer cleanupContainer(t, container)

	// Check that the server is listening on the correct port
	logs, err := container.Logs(ctx)
	if err != nil {
		t.Fatalf("Failed to get logs: %v", err)
	}
	defer logs.Close()

	logStr, err := readLogsToString(logs)
	if err != nil {
		t.Fatalf("Failed to read logs: %v", err)
	}

	// Check for port configuration in launch command
	if !strings.Contains(logStr, "-port 27015") {
		t.Error("Server should be configured to listen on port 27015")
	}

	// Check for IP binding
	if !strings.Contains(logStr, "-ip 0.0.0.0") {
		t.Error("Server should be configured to bind to all interfaces")
	}
}

// TestGarrysModImage_GracefulShutdown tests that the container shuts down gracefully
func TestGarrysModImage_GracefulShutdown(t *testing.T) {
	setupTest(t)
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"27015/udp", "27015/tcp"},
			Env: map[string]string{
				"NAME":          testServerName,
				"RCON_PASSWORD": testRconPassword,
			},
			WaitingFor: wait.ForLog("Garry's Mod server started with PID").WithStartupTimeout(startupTimeout),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	// Send SIGTERM to the container (graceful shutdown)
	stopTimeout := shutdownTimeout
	if err := container.Stop(ctx, &stopTimeout); err != nil {
		t.Fatalf("Failed to stop container gracefully: %v", err)
	}

	// Get final logs to check for graceful shutdown messages
	logs, err := container.Logs(ctx)
	if err != nil {
		t.Fatalf("Failed to get logs: %v", err)
	}
	defer logs.Close()

	// Read logs into string
	logStr, err := readLogsToString(logs)
	if err != nil {
		t.Fatalf("Failed to read logs: %v", err)
	}

	// Check for graceful shutdown indicators
	gracefulShutdownPatterns := []string{
		"Received SIGTERM",
		"stopping Garry's Mod server gracefully",
		"Sending quit command via RCON",
		"Garry's Mod server stopped gracefully",
	}

	foundPatterns := 0
	for _, pattern := range gracefulShutdownPatterns {
		if strings.Contains(logStr, pattern) {
			foundPatterns++
		}
	}

	if foundPatterns == 0 {
		t.Error("Expected at least one graceful shutdown message in logs")
		t.Logf("Container logs:\n%s", logStr)
	}

	// Verify container actually stopped
	state, err := container.State(ctx)
	if err != nil {
		t.Fatalf("Failed to get final container state: %v", err)
	}
	if state.Running {
		t.Error("Container should have stopped after SIGTERM")
	}

	// Cleanup
	if err := container.Terminate(ctx); err != nil {
		t.Logf("Failed to terminate container: %v", err)
	}
}

// TestGarrysModImage_FileStructure tests that the expected file structure is created
func TestGarrysModImage_FileStructure(t *testing.T) {
	setupTest(t)
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"27015/udp", "27015/tcp"},
			Env: map[string]string{
				"NAME":          testServerName,
				"RCON_PASSWORD": testRconPassword,
			},
			WaitingFor: wait.ForLog("Garry's Mod server started with PID").WithStartupTimeout(startupTimeout),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}
	defer cleanupContainer(t, container)

	// Check required files and directories exist
	requiredPaths := []string{
		"/data/server/srcds_run",
		"/data/server/garrysmod",
		"/data/scripts/start.sh",
		"/data/scripts/send-command.sh",
		"/data/steamcmd/steamcmd.sh",
		"/usr/local/bin/rcon-cli",
	}

	for _, path := range requiredPaths {
		exitCode, outputReader, err := container.Exec(ctx, []string{"ls", "-la", path})
		if err != nil {
			t.Fatalf("Failed to check path %s: %v", path, err)
		}
		if exitCode != 0 {
			output, _ := readLogsToString(outputReader)
			t.Errorf("Required path %s not found, exit code: %d, output: %s", path, exitCode, output)
		}
	}

	// Check that scripts are executable
	executableScripts := []string{
		"/data/scripts/start.sh",
		"/data/scripts/send-command.sh",
		"/data/server/srcds_run",
		"/usr/local/bin/rcon-cli",
	}

	for _, script := range executableScripts {
		exitCode, outputReader, err := container.Exec(ctx, []string{"test", "-x", script})
		if err != nil {
			t.Fatalf("Failed to test executable %s: %v", script, err)
		}
		if exitCode != 0 {
			output, _ := readLogsToString(outputReader)
			t.Errorf("Script %s should be executable, exit code: %d, output: %s", script, exitCode, output)
		}
	}
}

// TestGarrysModImage_SteamCMDIntegration tests SteamCMD functionality
func TestGarrysModImage_SteamCMDIntegration(t *testing.T) {
	setupTest(t)
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"27015/udp", "27015/tcp"},
			Env: map[string]string{
				"NAME":          testServerName,
				"RCON_PASSWORD": testRconPassword,
			},
			WaitingFor: wait.ForLog("Garry's Mod server started with PID").WithStartupTimeout(startupTimeout),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}
	defer cleanupContainer(t, container)

	// Check that SteamCMD is working
	exitCode, outputReader, err := container.Exec(ctx, []string{"/data/steamcmd/steamcmd.sh", "+quit"})
	if err != nil {
		t.Fatalf("Failed to test SteamCMD: %v", err)
	}
	if exitCode != 0 {
		output, _ := readLogsToString(outputReader)
		t.Errorf("SteamCMD should be functional, exit code: %d, output: %s", exitCode, output)
	}

	// Verify Garry's Mod server files were downloaded
	exitCode, outputReader, err = container.Exec(ctx, []string{"ls", "/data/server/garrysmod/gameinfo.txt"})
	if err != nil {
		t.Fatalf("Failed to check GMod files: %v", err)
	}
	if exitCode != 0 {
		t.Error("Garry's Mod server files should be present")
	}
}