package netutil

import (
	"fmt"
	"net"
	"time"
)

// PortRange defines the range for dynamic port allocation
type PortRange struct {
	Start int
	End   int
}

// DefaultPortRanges defines sensible defaults for different services
var (
	// JenkinsPortRange is the range for Jenkins HTTP ports
	JenkinsPortRange = PortRange{Start: 8080, End: 8100}
	
	// JenkinsAgentPortRange is the range for Jenkins agent ports
	JenkinsAgentPortRange = PortRange{Start: 50000, End: 50020}
	
	// RegistryPortRange is the range for Docker registry ports
	RegistryPortRange = PortRange{Start: 5000, End: 5020}
	
	// KubernetesNodePortRange is the range for Kubernetes NodePorts
	KubernetesNodePortRange = PortRange{Start: 30000, End: 32767}
)

// IsPortAvailable checks if a specific port is available for binding
// Returns true if the port can be bound, false otherwise
func IsPortAvailable(port int) bool {
	address := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	defer listener.Close()
	return true
}

// IsPortAvailableWithTimeout checks if a port is available with a timeout
// Useful for checking ports that might be in TIME_WAIT state
func IsPortAvailableWithTimeout(port int, timeout time.Duration) bool {
	address := fmt.Sprintf(":%d", port)
	
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		// Connection failed, port is likely available
		return IsPortAvailable(port)
	}
	
	// Connection succeeded, port is occupied
	conn.Close()
	return false
}

// FindAvailablePort searches for an available port within a given range
// Returns the first available port, or -1 if none found
func FindAvailablePort(startPort, endPort int) int {
	for port := startPort; port <= endPort; port++ {
		if IsPortAvailable(port) {
			return port
		}
	}
	return -1
}

// FindAvailablePortWithPreference tries the preferred port first,
// then searches the range if preferred is not available
func FindAvailablePortWithPreference(preferredPort int, searchRange PortRange) (int, error) {
	// Try preferred port first
	if IsPortAvailable(preferredPort) {
		return preferredPort, nil
	}
	
	// Preferred port not available, search range
	port := FindAvailablePort(searchRange.Start, searchRange.End)
	if port == -1 {
		return -1, fmt.Errorf("no available ports in range %d-%d", searchRange.Start, searchRange.End)
	}
	
	return port, nil
}

// AllocateDynamicHostPort intelligently allocates a host port
// with retry logic and validation
type PortAllocationConfig struct {
	PreferredPort int
	SearchRange   PortRange
	MaxRetries    int
	RetryDelay    time.Duration
	ServiceName   string
}

// AllocateDynamicHostPort attempts to allocate a port with retry logic
func AllocateDynamicHostPort(config PortAllocationConfig) (int, error) {
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 500 * time.Millisecond
	}
	
	var lastErr error
	for attempt := 1; attempt <= config.MaxRetries; attempt++ {
		port, err := FindAvailablePortWithPreference(config.PreferredPort, config.SearchRange)
		if err != nil {
			lastErr = err
			if attempt < config.MaxRetries {
				time.Sleep(config.RetryDelay)
			}
			continue
		}
		
		// Double-check port is still available (race condition mitigation)
		if IsPortAvailable(port) {
			return port, nil
		}
		
		// Port was taken between check and now, retry
		lastErr = fmt.Errorf("port %d was allocated by another process", port)
		if attempt < config.MaxRetries {
			time.Sleep(config.RetryDelay)
		}
	}
	
	return -1, fmt.Errorf("failed to allocate port for %s after %d attempts: %w", 
		config.ServiceName, config.MaxRetries, lastErr)
}

// GetPortsInUse returns a list of ports currently in use on the system
// This is useful for diagnostics and debugging
func GetPortsInUse(portRange PortRange) []int {
	var inUse []int
	for port := portRange.Start; port <= portRange.End; port++ {
		if !IsPortAvailable(port) {
			inUse = append(inUse, port)
		}
	}
	return inUse
}

// WaitForPortAvailable waits for a port to become available
// Useful when waiting for services to shut down
func WaitForPortAvailable(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	checkInterval := 500 * time.Millisecond
	
	for time.Now().Before(deadline) {
		if IsPortAvailable(port) {
			return nil
		}
		time.Sleep(checkInterval)
	}
	
	return fmt.Errorf("port %d did not become available within %v", port, timeout)
}

// WaitForPortOccupied waits for a port to become occupied (service started)
// Useful when waiting for services to start
func WaitForPortOccupied(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	checkInterval := 500 * time.Millisecond
	
	for time.Now().Before(deadline) {
		if !IsPortAvailable(port) {
			return nil
		}
		time.Sleep(checkInterval)
	}
	
	return fmt.Errorf("port %d did not become occupied within %v", port, timeout)
}

// ValidatePortRange checks if a port range is valid
func ValidatePortRange(portRange PortRange) error {
	if portRange.Start < 1 || portRange.Start > 65535 {
		return fmt.Errorf("invalid start port: %d (must be 1-65535)", portRange.Start)
	}
	if portRange.End < 1 || portRange.End > 65535 {
		return fmt.Errorf("invalid end port: %d (must be 1-65535)", portRange.End)
	}
	if portRange.Start > portRange.End {
		return fmt.Errorf("start port %d is greater than end port %d", portRange.Start, portRange.End)
	}
	return nil
}

// GetAvailablePortCount returns the number of available ports in a range
func GetAvailablePortCount(portRange PortRange) int {
	count := 0
	for port := portRange.Start; port <= portRange.End; port++ {
		if IsPortAvailable(port) {
			count++
		}
	}
	return count
}
