package k8sutil

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// PortForwardConfig holds configuration for kubectl port-forward
type PortForwardConfig struct {
	Namespace   string
	ServiceName string
	LocalPort   int
	RemotePort  int
	Timeout     time.Duration
	Persistent  bool   // If true, keep port-forward alive after validation
	AppName     string // Optional: application name for tracking
}

// PortForward manages a kubectl port-forward process
type PortForward struct {
	cmd        *exec.Cmd
	config     PortForwardConfig
	persistent bool
	startTime  time.Time
}

// PortForwardManager manages multiple persistent port-forward processes
type PortForwardManager struct {
	forwards map[string]*PortForward // key: "namespace/service:localPort"
	mu       sync.RWMutex
}

var (
	// Global port-forward manager
	globalManager     *PortForwardManager
	globalManagerOnce sync.Once
)

// GetPortForwardManager returns the global port-forward manager singleton
func GetPortForwardManager() *PortForwardManager {
	globalManagerOnce.Do(func() {
		globalManager = &PortForwardManager{
			forwards: make(map[string]*PortForward),
		}
	})
	return globalManager
}

// StartPortForward creates a background kubectl port-forward tunnel
// Returns a PortForward object that must be cleaned up with Stop() (unless Persistent=true)
func StartPortForward(config PortForwardConfig) (*PortForward, error) {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	// Build port-forward command
	portMapping := fmt.Sprintf("%d:%d", config.LocalPort, config.RemotePort)
	
	// Listen on IPv4 loopback only so http://127.0.0.1:<local> reliably matches kubectl;
	// on some hosts "localhost" resolves to [::1] first while kubectl listens on 127.0.0.1 only.
	cmd := exec.Command("kubectl", "port-forward",
		"--address", "127.0.0.1",
		"-n", config.Namespace,
		fmt.Sprintf("svc/%s", config.ServiceName),
		portMapping,
	)

	// Start the process in background
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start port-forward: %w", err)
	}

	pf := &PortForward{
		cmd:        cmd,
		config:     config,
		persistent: config.Persistent,
		startTime:  time.Now(),
	}

	if config.Persistent {
		fmt.Printf("[INFO] Started persistent port-forward: %s/%s %d:%d (PID: %d)\n",
			config.Namespace, config.ServiceName, config.LocalPort, config.RemotePort, cmd.Process.Pid)
	} else {
		fmt.Printf("[INFO] Started temporary port-forward: %s/%s %d:%d (PID: %d)\n",
			config.Namespace, config.ServiceName, config.LocalPort, config.RemotePort, cmd.Process.Pid)
	}

	// Wait for tunnel to be ready
	if err := pf.WaitUntilReady(); err != nil {
		pf.Stop() // Clean up on failure
		return nil, err
	}

	// Register persistent port-forwards with manager
	if config.Persistent {
		manager := GetPortForwardManager()
		manager.Register(pf)
	}

	return pf, nil
}

// StartPersistentPortForward starts a port-forward that stays alive after validation
// This is the recommended way to expose applications to users
func StartPersistentPortForward(config PortForwardConfig) (*PortForward, error) {
	config.Persistent = true
	
	// Check if port-forward already exists
	manager := GetPortForwardManager()
	if existing := manager.Get(config.Namespace, config.ServiceName, config.LocalPort); existing != nil {
		fmt.Printf("[INFO] Reusing existing port-forward for %s/%s on port %d\n",
			config.Namespace, config.ServiceName, config.LocalPort)
		return existing, nil
	}
	
	return StartPortForward(config)
}

// WaitUntilReady waits for the port-forward tunnel to be established
func (pf *PortForward) WaitUntilReady() error {
	deadline := time.Now().Add(pf.config.Timeout)
	checkInterval := 500 * time.Millisecond

	fmt.Printf("[INFO] Waiting for port-forward tunnel to be ready (timeout: %v)...\n", pf.config.Timeout)

	for time.Now().Before(deadline) {
		// Check if process is still running
		if pf.cmd.ProcessState != nil && pf.cmd.ProcessState.Exited() {
			return fmt.Errorf("port-forward process exited prematurely")
		}

		// Try to connect to the local port
		if isLocalPortResponding(pf.config.LocalPort) {
			fmt.Println("[OK] Port-forward tunnel ready")
			return nil
		}

		time.Sleep(checkInterval)
	}

	return fmt.Errorf("port-forward tunnel did not become ready within %v", pf.config.Timeout)
}

// Stop terminates the port-forward process gracefully
// If the port-forward is persistent, it will be unregistered from the manager
func (pf *PortForward) Stop() error {
	if pf.cmd == nil || pf.cmd.Process == nil {
		return nil
	}

	if pf.persistent {
		fmt.Printf("[INFO] Stopping persistent port-forward (PID: %d)\n", pf.cmd.Process.Pid)
	} else {
		fmt.Printf("[INFO] Stopping temporary port-forward (PID: %d)\n", pf.cmd.Process.Pid)
	}

	// Try graceful termination first
	if err := pf.cmd.Process.Kill(); err != nil {
		// Process might have already exited
		if !strings.Contains(err.Error(), "process already finished") {
			return fmt.Errorf("failed to stop port-forward: %w", err)
		}
	}

	// Wait for process to exit
	_ = pf.cmd.Wait()

	// Unregister from manager if persistent
	if pf.persistent {
		manager := GetPortForwardManager()
		manager.Unregister(pf)
	}

	fmt.Println("[OK] Port-forward stopped")
	return nil
}

// IsPersistent returns true if this port-forward is persistent
func (pf *PortForward) IsPersistent() bool {
	return pf.persistent
}

// GetPID returns the process ID of the port-forward
func (pf *PortForward) GetPID() int {
	if pf.cmd != nil && pf.cmd.Process != nil {
		return pf.cmd.Process.Pid
	}
	return 0
}

// IsRunning checks if the port-forward process is still running
func (pf *PortForward) IsRunning() bool {
	if pf.cmd == nil || pf.cmd.Process == nil {
		return false
	}
	if pf.cmd.ProcessState != nil && pf.cmd.ProcessState.Exited() {
		return false
	}
	return true
}

// GetUptime returns how long the port-forward has been running
func (pf *PortForward) GetUptime() time.Duration {
	return time.Since(pf.startTime)
}

// GetLocalURL returns the local URL for accessing the forwarded service
func (pf *PortForward) GetLocalURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", pf.config.LocalPort)
}

// isLocalPortResponding checks if a local port is accepting connections
func isLocalPortResponding(port int) bool {
	client := &http.Client{Timeout: 1 * time.Second}
	
	// Try to make a simple request
	// We don't care about the response, just whether the port is open
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d", port))
	if err == nil {
		resp.Body.Close()
		return true
	}

	// Check if it's a connection error vs other errors
	// If we get any HTTP response (even error), port is responding
	return !strings.Contains(err.Error(), "connection refused") &&
		!strings.Contains(err.Error(), "connect: connection refused")
}

// ValidateHTTPEndpoint validates that an HTTP endpoint is accessible and responding
func ValidateHTTPEndpoint(url string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("endpoint not accessible: %w", err)
	}
	defer resp.Body.Close()

	// Accept any HTTP response (200, 403, etc.) as valid
	// We just want to confirm the service is responding
	if resp.StatusCode >= 500 {
		return fmt.Errorf("endpoint returned server error: HTTP %d", resp.StatusCode)
	}

	return nil
}

// RetryPortForward attempts to start port-forward with retries
func RetryPortForward(config PortForwardConfig, maxRetries int, retryDelay time.Duration) (*PortForward, error) {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			fmt.Printf("[INFO] Retry %d/%d: Starting port-forward...\n", attempt, maxRetries)
			time.Sleep(retryDelay)
		}

		pf, err := StartPortForward(config)
		if err != nil {
			lastErr = err
			fmt.Printf("[WARN] Attempt %d failed: %v\n", attempt, err)
			continue
		}

		return pf, nil
	}

	return nil, fmt.Errorf("failed to start port-forward after %d attempts: %w", maxRetries, lastErr)
}

// ===== PortForwardManager Methods =====

// makeKey generates a unique key for a port-forward
func (m *PortForwardManager) makeKey(namespace, serviceName string, localPort int) string {
	return fmt.Sprintf("%s/%s:%d", namespace, serviceName, localPort)
}

// Register adds a port-forward to the manager
func (m *PortForwardManager) Register(pf *PortForward) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	key := m.makeKey(pf.config.Namespace, pf.config.ServiceName, pf.config.LocalPort)
	m.forwards[key] = pf
	
	fmt.Printf("[INFO] Registered persistent port-forward: %s (PID: %d)\n", key, pf.GetPID())
}

// Unregister removes a port-forward from the manager
func (m *PortForwardManager) Unregister(pf *PortForward) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	key := m.makeKey(pf.config.Namespace, pf.config.ServiceName, pf.config.LocalPort)
	delete(m.forwards, key)
	
	fmt.Printf("[INFO] Unregistered port-forward: %s\n", key)
}

// Get retrieves an existing port-forward
func (m *PortForwardManager) Get(namespace, serviceName string, localPort int) *PortForward {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	key := m.makeKey(namespace, serviceName, localPort)
	pf, exists := m.forwards[key]
	if !exists {
		return nil
	}
	
	// Check if still running
	if !pf.IsRunning() {
		// Port-forward died, clean it up
		go func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			delete(m.forwards, key)
			fmt.Printf("[WARN] Port-forward %s is no longer running, removed from manager\n", key)
		}()
		return nil
	}
	
	return pf
}

// List returns all active port-forwards
func (m *PortForwardManager) List() []*PortForward {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make([]*PortForward, 0, len(m.forwards))
	for _, pf := range m.forwards {
		if pf.IsRunning() {
			result = append(result, pf)
		}
	}
	return result
}

// StopAll stops all managed port-forwards
func (m *PortForwardManager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	fmt.Printf("[INFO] Stopping all persistent port-forwards (%d active)\n", len(m.forwards))
	
	var errors []error
	for key, pf := range m.forwards {
		fmt.Printf("[INFO] Stopping port-forward: %s\n", key)
		if err := pf.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", key, err))
		}
	}
	
	// Clear the map
	m.forwards = make(map[string]*PortForward)
	
	if len(errors) > 0 {
		return fmt.Errorf("errors stopping port-forwards: %v", errors)
	}
	
	fmt.Println("[OK] All port-forwards stopped")
	return nil
}

// CleanupStale removes port-forwards that are no longer running
func (m *PortForwardManager) CleanupStale() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	staleCount := 0
	for key, pf := range m.forwards {
		if !pf.IsRunning() {
			delete(m.forwards, key)
			staleCount++
			fmt.Printf("[INFO] Removed stale port-forward: %s\n", key)
		}
	}
	
	if staleCount > 0 {
		fmt.Printf("[INFO] Cleaned up %d stale port-forward(s)\n", staleCount)
	}
	
	return staleCount
}

// PrintStatus prints the status of all managed port-forwards
func (m *PortForwardManager) PrintStatus() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if len(m.forwards) == 0 {
		fmt.Println("[INFO] No active port-forwards")
		return
	}
	
	fmt.Printf("\n=== Active Port-Forwards (%d) ===\n", len(m.forwards))
	for key, pf := range m.forwards {
		status := "❌ Stopped"
		if pf.IsRunning() {
			status = "✅ Running"
		}
		uptime := pf.GetUptime().Round(time.Second)
		fmt.Printf("  %s → localhost:%d\n", key, pf.config.LocalPort)
		fmt.Printf("    Status: %s | PID: %d | Uptime: %v\n", status, pf.GetPID(), uptime)
		fmt.Printf("    URL: %s\n", pf.GetLocalURL())
	}
	fmt.Println("================================")
}
