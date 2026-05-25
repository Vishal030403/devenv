package registry

import (
	"devenv-gamma/pkg/netutil"
	"devenv-gamma/pkg/state"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	registryName          = "devenv-registry"
	registryDefaultPort   = 5000
	registryContainerPort = 5000
	registryImage         = "registry:2"
	maxRetries            = 3
	retryDelay            = 2 * time.Second
)

// StartRegistry starts the Docker registry with intelligent dynamic port allocation
func StartRegistry() error {
	fmt.Println("[INFO] Starting local Docker registry with dynamic port allocation")
	
	// Check if registry container already exists
	existing, running, currentPort, err := checkRegistryStatusDetailed()
	if err != nil {
		return fmt.Errorf("failed to check registry status: %w", err)
	}

	// Try to reuse existing container
	if existing {
		if running {
			fmt.Printf("[INFO] Registry container already running on port %d\n", currentPort)
			
			// Validate it's actually accessible
			if err := validateRegistryAccessOnPort(currentPort, 5*time.Second); err != nil {
				fmt.Printf("[WARN] Existing registry on port %d not accessible: %v\n", currentPort, err)
				fmt.Println("[INFO] Recreating unhealthy registry container...")
				if err := removeRegistry(); err != nil {
					return fmt.Errorf("failed to remove unhealthy registry: %w", err)
				}
				// Continue to create new container
			} else {
				// Update runtime state with current configuration
				if err := updateRegistryRuntimeState(currentPort, true); err != nil {
					fmt.Printf("[WARN] Failed to update runtime state: %v\n", err)
				}
				fmt.Printf("[INFO] Registry available at localhost:%d\n", currentPort)
				fmt.Printf("[OK] Registry ready at localhost:%d\n", currentPort)
				return nil
			}
		} else {
			fmt.Printf("[INFO] Registry container exists but stopped (was on port %d)\n", currentPort)
			fmt.Println("[INFO] Attempting to restart existing container...")
			
			if err := startExistingRegistry(); err != nil {
				fmt.Printf("[WARN] Failed to restart existing registry: %v\n", err)
				fmt.Println("[INFO] Recreating registry container...")
				if err := removeRegistry(); err != nil {
					return fmt.Errorf("failed to remove failed registry: %w", err)
				}
				// Continue to create new container
			} else {
				// Wait for it to be ready
				if err := waitForRegistryReadyOnPort(currentPort, 30*time.Second); err != nil {
					fmt.Printf("[WARN] Registry restarted but not accessible: %v\n", err)
					if err := removeRegistry(); err != nil {
						return fmt.Errorf("failed to remove inaccessible registry: %w", err)
					}
					// Continue to create new container
				} else {
					if err := updateRegistryRuntimeState(currentPort, true); err != nil {
						fmt.Printf("[WARN] Failed to update runtime state: %v\n", err)
					}
					fmt.Printf("[INFO] Registry available at localhost:%d\n", currentPort)
					fmt.Printf("[OK] Registry restarted at localhost:%d\n", currentPort)
					return nil
				}
			}
		}
	}

	// Allocate dynamic host port
	fmt.Println("[INFO] Allocating dynamic host port for registry...")
	hostPort, err := allocateRegistryPort()
	if err != nil {
		return fmt.Errorf("failed to allocate registry port: %w", err)
	}

	if hostPort != registryDefaultPort {
		fmt.Printf("[INFO] Port %d occupied, allocated port %d\n", registryDefaultPort, hostPort)
	} else {
		fmt.Printf("[INFO] Using preferred port %d\n", hostPort)
	}

	// Create new registry with retry
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			fmt.Printf("[INFO] Retry %d/%d: Creating registry on port %d...\n", attempt, maxRetries, hostPort)
			time.Sleep(retryDelay)
			
			// Try to find another port if this one failed
			newPort, err := allocateRegistryPort()
			if err != nil {
				lastErr = fmt.Errorf("failed to allocate new port on retry: %w", err)
				continue
			}
			if newPort != hostPort {
				fmt.Printf("[INFO] Switching to alternate port %d\n", newPort)
				hostPort = newPort
			}
		}

		if err := createRegistryWithPort(hostPort); err != nil {
			lastErr = err
			fmt.Printf("[WARN] Attempt %d failed: %v\n", attempt, err)
			// Clean up failed attempt
			_ = removeRegistry()
			continue
		}

		// Wait for registry to be ready
		if err := waitForRegistryReadyOnPort(hostPort, 30*time.Second); err != nil {
			lastErr = err
			fmt.Printf("[WARN] Registry created but not ready on port %d: %v\n", hostPort, err)
			// Clean up failed attempt
			_ = removeRegistry()
			continue
		}

		// Update runtime state with successful configuration
		if err := updateRegistryRuntimeState(hostPort, true); err != nil {
			fmt.Printf("[WARN] Failed to update runtime state: %v\n", err)
		}

		fmt.Printf("[INFO] Registry available at localhost:%d\n", hostPort)
		fmt.Printf("[OK] Registry running at localhost:%d\n", hostPort)
		fmt.Println("[OK] Registry persistent storage enabled")
		fmt.Printf("[OK] Registry URL: %s\n", GetRegistryURL())
		return nil
	}

	return fmt.Errorf("failed to start registry after %d attempts: %w", maxRetries, lastErr)
}

// checkRegistryStatusDetailed checks registry status and extracts current port mapping
func checkRegistryStatusDetailed() (exists bool, running bool, hostPort int, err error) {
	// Check if container exists
	out, err := exec.Command("docker", "ps", "-a", "--filter", "name=^"+registryName+"$", "--format", "{{.Names}}").Output()
	if err != nil {
		return false, false, 0, err
	}

	if !strings.Contains(string(out), registryName) {
		return false, false, 0, nil
	}

	// Container exists, check if it's running
	out, err = exec.Command("docker", "ps", "--filter", "name=^"+registryName+"$", "--format", "{{.Names}}").Output()
	if err != nil {
		return true, false, 0, err
	}

	isRunning := strings.Contains(string(out), registryName)

	// Extract current port mapping
	portCmd := exec.Command("docker", "port", registryName, strconv.Itoa(registryContainerPort))
	portOut, err := portCmd.Output()
	if err == nil && len(portOut) > 0 {
		// Parse output like "0.0.0.0:5000"
		portStr := strings.TrimSpace(string(portOut))
		parts := strings.Split(portStr, ":")
		if len(parts) == 2 {
			if port, err := strconv.Atoi(parts[1]); err == nil {
				return true, isRunning, port, nil
			}
		}
	}

	// Default to registryDefaultPort if we can't determine
	return true, isRunning, registryDefaultPort, nil
}

// Legacy function for compatibility
func checkRegistryStatus() (exists bool, running bool, err error) {
	exists, running, _, err = checkRegistryStatusDetailed()
	return
}

func startExistingRegistry() error {
	cmd := exec.Command("docker", "start", registryName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// allocateRegistryPort finds an available port for the registry
func allocateRegistryPort() (int, error) {
	// Try to use port from previous runtime state first
	runtimeState, err := state.GetRuntimeState()
	if err == nil && runtimeState.Registry.Enabled && runtimeState.Registry.HostPort > 0 {
		if netutil.IsPortAvailable(runtimeState.Registry.HostPort) {
			fmt.Printf("[INFO] Reusing previous port %d from runtime state\n", runtimeState.Registry.HostPort)
			return runtimeState.Registry.HostPort, nil
		}
	}

	// Allocate new port dynamically
	config := netutil.PortAllocationConfig{
		PreferredPort: registryDefaultPort,
		SearchRange:   netutil.RegistryPortRange,
		MaxRetries:    3,
		RetryDelay:    500 * time.Millisecond,
		ServiceName:   "Docker Registry",
	}

	port, err := netutil.AllocateDynamicHostPort(config)
	if err != nil {
		return 0, fmt.Errorf("failed to allocate port: %w", err)
	}

	return port, nil
}

// createRegistryWithPort creates a registry container with specified host port
func createRegistryWithPort(hostPort int) error {
	// Double-check port is available
	if !netutil.IsPortAvailable(hostPort) {
		return fmt.Errorf("port %d is already in use", hostPort)
	}

	// Create volume for persistent storage
	volumeName := registryName + "-data"
	_ = exec.Command("docker", "volume", "create", volumeName).Run()

	portMapping := fmt.Sprintf("%d:%d", hostPort, registryContainerPort)

	cmd := exec.Command("docker", "run", "-d",
		"--name", registryName,
		"-p", portMapping,
		"-v", volumeName+":/var/lib/registry",
		"--restart", "unless-stopped",
		registryImage,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create registry container: %w", err)
	}

	fmt.Printf("[INFO] Created registry container with port mapping %s\n", portMapping)
	return nil
}

// Legacy function for backward compatibility
func createRegistry(port string) error {
	portInt, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid port: %s", port)
	}
	return createRegistryWithPort(portInt)
}

func removeRegistry() error {
	// Stop if running
	_ = exec.Command("docker", "stop", registryName).Run()
	
	// Remove container
	cmd := exec.Command("docker", "rm", "-f", registryName)
	return cmd.Run()
}

// updateRegistryRuntimeState updates the runtime state with registry configuration
func updateRegistryRuntimeState(hostPort int, healthy bool) error {
	containerID, _ := exec.Command("docker", "inspect", registryName, "--format", "{{.Id}}").Output()
	
	return state.UpdateRegistryState(func(rs *state.RegistryState) {
		rs.Enabled = true
		rs.HostPort = hostPort
		rs.ContainerPort = registryContainerPort
		rs.URL = fmt.Sprintf("localhost:%d", hostPort)
		rs.ContainerID = strings.TrimSpace(string(containerID))
		rs.ContainerName = registryName
		rs.Healthy = healthy
		rs.LastCheck = time.Now()
	})
}

func waitForRegistryReadyOnPort(hostPort int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	checkInterval := 1 * time.Second

	for time.Now().Before(deadline) {
		if err := validateRegistryAccessOnPort(hostPort, 2*time.Second); err == nil {
			return nil
		}
		time.Sleep(checkInterval)
	}

	return fmt.Errorf("registry did not become ready on port %d within %v", hostPort, timeout)
}

func validateRegistryAccessOnPort(hostPort int, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}
	url := fmt.Sprintf("http://127.0.0.1:%d/v2/", hostPort)
	
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("registry not accessible on port %d: %w", hostPort, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registry on port %d returned status %d", hostPort, resp.StatusCode)
	}

	return nil
}

// Legacy functions for backward compatibility
func isPortAvailable(port string) bool {
	portInt, err := strconv.Atoi(port)
	if err != nil {
		return false
	}
	return netutil.IsPortAvailable(portInt)
}

func waitForRegistryReady(port string, timeout time.Duration) error {
	portInt, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid port: %s", port)
	}
	return waitForRegistryReadyOnPort(portInt, timeout)
}

func validateRegistryAccess(port string) error {
	portInt, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid port: %s", port)
	}
	return validateRegistryAccessOnPort(portInt, 5*time.Second)
}

func validateRegistryAccessWithClient(port string, client *http.Client) error {
	return validateRegistryAccess(port)
}
// CheckRegistryStatus validates registry health with detailed diagnostics
func CheckRegistryStatus() error {
	fmt.Println("[INFO] Checking Docker registry status")
	
	exists, running, err := checkRegistryStatus()
	if err != nil {
		return fmt.Errorf("failed to check registry: %w", err)
	}

	if !exists {
		return errors.New("registry container does not exist - run 'devenv setup' first")
	}

	if !running {
		return errors.New("registry container exists but is not running")
	}

	// Get detailed status
	out, err := exec.Command("docker", "inspect", registryName, "--format", "{{.State.Status}}").Output()
	if err != nil {
		fmt.Println("[WARN] Could not inspect registry:", err)
	} else {
		fmt.Println("[OK] Registry container status:", strings.TrimSpace(string(out)))
	}

	// Check uptime
	out, err = exec.Command("docker", "inspect", registryName, "--format", "{{.State.StartedAt}}").Output()
	if err == nil {
		fmt.Println("[OK] Registry started at:", strings.TrimSpace(string(out)))
	}

	// Get actual port from runtime state or container
	hostPort := registryDefaultPort
	runtimeState, err := state.GetRuntimeState()
	if err == nil && runtimeState.Registry.Enabled && runtimeState.Registry.HostPort > 0 {
		hostPort = runtimeState.Registry.HostPort
	} else {
		// Try to get from container
		_, _, port, err := checkRegistryStatusDetailed()
		if err == nil && port > 0 {
			hostPort = port
		}
	}

	// Validate API access
	if err := validateRegistryAccessOnPort(hostPort, 5*time.Second); err != nil {
		return fmt.Errorf("registry is running but API not accessible: %w", err)
	}

	fmt.Printf("[OK] Registry API responding at http://127.0.0.1:%d/v2/\n", hostPort)
	
	// Check volume
	out, err = exec.Command("docker", "inspect", registryName, "--format", "{{range .Mounts}}{{.Name}}{{end}}").Output()
	if err == nil && strings.Contains(string(out), registryName+"-data") {
		fmt.Println("[OK] Persistent storage mounted")
	}

	return nil
}

// StopRegistry stops and removes the registry with cleanup
func StopRegistry() error {
	fmt.Println("[INFO] Removing Docker registry")
	
	exists, _, err := checkRegistryStatus()
	if err != nil {
		return fmt.Errorf("failed to check registry: %w", err)
	}

	if !exists {
		fmt.Println("[INFO] Registry container not found, nothing to remove")
		
		// Clear runtime state anyway
		_ = state.UpdateRegistryState(func(rs *state.RegistryState) {
			rs.Enabled = false
			rs.Healthy = false
		})
		
		return nil
	}

	// Stop gracefully
	fmt.Println("[INFO] Stopping registry container...")
	stopCmd := exec.Command("docker", "stop", "-t", "10", registryName)
	if err := stopCmd.Run(); err != nil {
		fmt.Println("[WARN] Failed to stop gracefully, forcing...")
	}

	// Remove container
	fmt.Println("[INFO] Removing registry container...")
	cmd := exec.Command("docker", "rm", "-f", registryName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove registry container: %w", err)
	}

	// Update runtime state
	_ = state.UpdateRegistryState(func(rs *state.RegistryState) {
		rs.Enabled = false
		rs.Healthy = false
	})

	fmt.Println("[OK] Registry removed")
	
	// Optionally clean up volume (keep it for persistence by default)
	fmt.Println("[INFO] Registry data volume preserved for reuse")
	
	return nil
}

// StartRegistryWithPort starts the registry on a specific port (backward compatibility)
// Note: This function is deprecated in favor of StartRegistry() with dynamic allocation
func StartRegistryWithPort(port string) error {
	portInt, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid port: %s", port)
	}
	
	// Check if port is available
	if !netutil.IsPortAvailable(portInt) {
		return fmt.Errorf("port %d is not available", portInt)
	}
	
	// Use the new dynamic allocation system but with the specific port
	fmt.Printf("[INFO] Starting registry on specified port %d\n", portInt)
	
	// Remove existing container if any
	_, _, _, err = checkRegistryStatusDetailed()
	if err == nil {
		_ = removeRegistry()
	}
	
	// Create with specific port
	if err := createRegistryWithPort(portInt); err != nil {
		return err
	}
	
	// Wait for readiness
	if err := waitForRegistryReadyOnPort(portInt, 30*time.Second); err != nil {
		return err
	}
	
	// Update runtime state
	if err := updateRegistryRuntimeState(portInt, true); err != nil {
		fmt.Printf("[WARN] Failed to update runtime state: %v\n", err)
	}
	
	fmt.Printf("[OK] Registry running at localhost:%d\n", portInt)
	return nil
}

// GetRegistryPort returns the current registry host port (from runtime state or default)
func GetRegistryPort() int {
	// Try to get from runtime state
	port, err := state.GetRegistryPort()
	if err == nil && port > 0 {
		return port
	}
	
	// Try to get from running container
	_, _, hostPort, err := checkRegistryStatusDetailed()
	if err == nil && hostPort > 0 {
		return hostPort
	}
	
	// Fall back to default
	return registryDefaultPort
}

// GetRegistryURL returns the full registry URL (from runtime state or default)
func GetRegistryURL() string {
	// Try to get from runtime state
	url, err := state.GetRegistryURL()
	if err == nil && url != "" {
		return url
	}
	
	// Build from current port
	port := GetRegistryPort()
	return fmt.Sprintf("localhost:%d", port)
}

// GetRegistryPortString returns the port as a string (for backward compatibility)
func GetRegistryPortString() string {
	return strconv.Itoa(GetRegistryPort())
}
