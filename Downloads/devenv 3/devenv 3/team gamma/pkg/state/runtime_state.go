package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RuntimeState holds the dynamic configuration for running services
type RuntimeState struct {
	Jenkins      JenkinsState                        `json:"jenkins"`
	Registry     RegistryState                       `json:"registry"`
	PortForwards map[string]PortForwardInfo          `json:"port_forwards"` // Active port-forwards
	Deployments  map[string]WorkloadDeploymentState  `json:"deployments"`   // key: namespace/deployment

	LastUpdated time.Time `json:"last_updated"`
	Version     string    `json:"version"`

	mu sync.RWMutex
}

// PortForwardInfo holds information about an active port-forward
type PortForwardInfo struct {
	Namespace   string    `json:"namespace"`
	ServiceName string    `json:"service_name"`
	AppName     string    `json:"app_name"`      // Optional application name
	LocalPort   int       `json:"local_port"`
	RemotePort  int       `json:"remote_port"`
	PID         int       `json:"pid"`
	StartTime   time.Time `json:"start_time"`
	Active      bool      `json:"active"`
}

// JenkinsState holds Jenkins runtime configuration
type JenkinsState struct {
	Enabled       bool   `json:"enabled"`
	UIPort        int    `json:"ui_port"`         // Host port for Jenkins UI
	AgentPort     int    `json:"agent_port"`      // Host port for Jenkins agents
	NodePort      int    `json:"node_port"`       // Kubernetes NodePort
	URL           string `json:"url"`             // Full Jenkins URL
	ContainerID   string `json:"container_id"`    // For Docker-based Jenkins
	PodName       string `json:"pod_name"`        // For K8s-based Jenkins
	Healthy       bool   `json:"healthy"`
	LastCheck     time.Time `json:"last_check"`
}

// RegistryState holds Docker Registry runtime configuration
type RegistryState struct {
	Enabled        bool      `json:"enabled"`
	HostPort       int       `json:"host_port"`        // Dynamic host port
	ContainerPort  int       `json:"container_port"`   // Always 5000
	URL            string    `json:"url"`              // Full registry URL
	ContainerID    string    `json:"container_id"`
	ContainerName  string    `json:"container_name"`
	Healthy        bool      `json:"healthy"`
	LastCheck      time.Time `json:"last_check"`
}

const (
	stateVersion  = "1.1.0"
	stateFileName = "runtime-state.json"

	canonicalJenkinsHostUIPort       = 8080 // kubectl port-forward — NEVER a Kubernetes NodePort
	kubernetesNodePortRangeMin       = 30000
	kubernetesNodePortRangeMax       = 32767
)

var (
	globalState     *RuntimeState
	globalStateLock sync.Mutex
)

// GetStateFilePath returns the path to the state file
func GetStateFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	
	stateDir := filepath.Join(homeDir, ".devenv", "team-gamma")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create state directory: %w", err)
	}
	
	return filepath.Join(stateDir, stateFileName), nil
}

// LoadRuntimeState loads the runtime state from disk
func LoadRuntimeState() (*RuntimeState, error) {
	globalStateLock.Lock()
	defer globalStateLock.Unlock()
	
	// Return cached state if available
	if globalState != nil {
		return globalState, nil
	}
	
	stateFile, err := GetStateFilePath()
	if err != nil {
		return nil, err
	}
	
	// If file doesn't exist, return new state
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		globalState = newRuntimeState()
		return globalState, nil
	}
	
	// Read existing state
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}
	
	state := &RuntimeState{}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	if state.Deployments == nil {
		state.Deployments = make(map[string]WorkloadDeploymentState)
	}

	if migrated := migrateLegacyJenkinsLocalExposure(state); migrated {
		state.mu.Lock()
		if errSave := persistRuntimeState(state); errSave != nil {
			state.mu.Unlock()
			return nil, fmt.Errorf("failed to migrate runtime state file: %w", errSave)
		}
		state.mu.Unlock()
	}

	globalState = state
	return state, nil
}

// migrateLegacyJenkinsLocalExposure fixes older deployments that stored Kubernetes NodePorts
// in ui_port / url instead of localhost kubectl port-forward semantics.
func migrateLegacyJenkinsLocalExposure(s *RuntimeState) bool {
	if s == nil || !s.Jenkins.Enabled {
		return false
	}
	var changed bool
	jp := s.Jenkins.UIPort
	if jp >= kubernetesNodePortRangeMin && jp <= kubernetesNodePortRangeMax {
		if s.Jenkins.NodePort == 0 {
			s.Jenkins.NodePort = jp
		}
		s.Jenkins.UIPort = canonicalJenkinsHostUIPort
		s.Jenkins.URL = fmt.Sprintf("http://127.0.0.1:%d", canonicalJenkinsHostUIPort)
		changed = true
	}

	if u := strings.TrimSpace(s.Jenkins.URL); u != "" {
		if hp := loopbackHTTPURLPortSuffix(u); hp >= kubernetesNodePortRangeMin && hp <= kubernetesNodePortRangeMax {
			if s.Jenkins.NodePort == 0 {
				s.Jenkins.NodePort = hp
			}
			s.Jenkins.UIPort = canonicalJenkinsHostUIPort
			s.Jenkins.URL = fmt.Sprintf("http://127.0.0.1:%d", canonicalJenkinsHostUIPort)
			changed = true
		}
	}
	return changed
}

func loopbackHTTPURLPortSuffix(u string) int {
	u = strings.TrimSpace(u)
	for _, pre := range []string{"http://localhost:", "https://localhost:", "http://127.0.0.1:", "https://127.0.0.1:"} {
		if !strings.HasPrefix(u, pre) {
			continue
		}
		rest := u[len(pre):]
		if idx := strings.IndexAny(rest, "/?#"); idx >= 0 {
			rest = rest[:idx]
		}
		p, err := strconv.Atoi(strings.TrimSpace(rest))
		if err != nil || p <= 0 {
			return -1
		}
		return p
	}
	return -1
}

// persistRuntimeState writes the given state to disk. The caller must hold state.mu
// exclusively when other goroutines may mutate this RuntimeState — serialization runs
// while the lock is held so marshaling observes a consistent snapshot.
func persistRuntimeState(state *RuntimeState) error {
	if state == nil {
		return fmt.Errorf("nil runtime state")
	}

	state.LastUpdated = time.Now()
	state.Version = stateVersion

	stateFile, err := GetStateFilePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	tempFile := stateFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp state file: %w", err)
	}

	if err := os.Rename(tempFile, stateFile); err != nil {
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	globalState = state
	return nil
}

// SaveRuntimeState persists the runtime state to disk. It acquires state.mu — do not
// call while already holding state.mu (use persistRuntimeState from internal helpers).
func SaveRuntimeState(state *RuntimeState) error {
	if state == nil {
		return fmt.Errorf("nil runtime state")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return persistRuntimeState(state)
}

// GetRuntimeState returns the current runtime state (loads if necessary)
func GetRuntimeState() (*RuntimeState, error) {
	return LoadRuntimeState()
}

// UpdateJenkinsState updates the Jenkins portion of runtime state
func UpdateJenkinsState(updates func(*JenkinsState)) error {
	state, err := GetRuntimeState()
	if err != nil {
		return err
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	updates(&state.Jenkins)
	return persistRuntimeState(state)
}

// UpdateRegistryState updates the Registry portion of runtime state
func UpdateRegistryState(updates func(*RegistryState)) error {
	state, err := GetRuntimeState()
	if err != nil {
		return err
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	updates(&state.Registry)
	return persistRuntimeState(state)
}

// GetJenkinsURL returns the current Jenkins URL from state
func GetJenkinsURL() (string, error) {
	state, err := GetRuntimeState()
	if err != nil {
		return "", err
	}
	
	state.mu.RLock()
	defer state.mu.RUnlock()
	
	if !state.Jenkins.Enabled || state.Jenkins.URL == "" {
		return "", fmt.Errorf("Jenkins not configured in runtime state")
	}
	
	return state.Jenkins.URL, nil
}

// GetRegistryURL returns the current Registry URL from state
func GetRegistryURL() (string, error) {
	state, err := GetRuntimeState()
	if err != nil {
		return "", err
	}
	
	state.mu.RLock()
	defer state.mu.RUnlock()
	
	if !state.Registry.Enabled || state.Registry.URL == "" {
		return "", fmt.Errorf("Registry not configured in runtime state")
	}
	
	return state.Registry.URL, nil
}

// GetJenkinsPort returns the current Jenkins UI port from state
func GetJenkinsPort() (int, error) {
	state, err := GetRuntimeState()
	if err != nil {
		return 0, err
	}
	
	state.mu.RLock()
	defer state.mu.RUnlock()
	
	if !state.Jenkins.Enabled || state.Jenkins.UIPort == 0 {
		return 0, fmt.Errorf("Jenkins port not configured in runtime state")
	}
	
	return state.Jenkins.UIPort, nil
}

// GetRegistryPort returns the current Registry port from state
func GetRegistryPort() (int, error) {
	state, err := GetRuntimeState()
	if err != nil {
		return 0, err
	}
	
	state.mu.RLock()
	defer state.mu.RUnlock()
	
	if !state.Registry.Enabled || state.Registry.HostPort == 0 {
		return 0, fmt.Errorf("Registry port not configured in runtime state")
	}
	
	return state.Registry.HostPort, nil
}

// ClearRuntimeState removes the runtime state file
func ClearRuntimeState() error {
	globalStateLock.Lock()
	defer globalStateLock.Unlock()
	
	stateFile, err := GetStateFilePath()
	if err != nil {
		return err
	}
	
	if err := os.Remove(stateFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove state file: %w", err)
	}
	
	globalState = nil
	return nil
}

// newRuntimeState creates a new empty runtime state
func newRuntimeState() *RuntimeState {
	return &RuntimeState{
		Jenkins: JenkinsState{
			Enabled:   false,
			Healthy:   false,
		},
		Registry: RegistryState{
			Enabled:       false,
			ContainerPort: 5000, // Always 5000 internally
			Healthy:       false,
		},
		PortForwards: make(map[string]PortForwardInfo),
		Deployments:  make(map[string]WorkloadDeploymentState),
		LastUpdated:  time.Now(),
		Version:      stateVersion,
	}
}

// RegisterPortForward adds a port-forward to the runtime state
func RegisterPortForward(namespace, serviceName, appName string, localPort, remotePort, pid int) error {
	state, err := GetRuntimeState()
	if err != nil {
		return err
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	key := fmt.Sprintf("%s/%s:%d", namespace, serviceName, localPort)
	state.PortForwards[key] = PortForwardInfo{
		Namespace:   namespace,
		ServiceName: serviceName,
		AppName:     appName,
		LocalPort:   localPort,
		RemotePort:  remotePort,
		PID:         pid,
		StartTime:   time.Now(),
		Active:      true,
	}

	return persistRuntimeState(state)
}

// UnregisterPortForward removes a port-forward from the runtime state
func UnregisterPortForward(namespace, serviceName string, localPort int) error {
	state, err := GetRuntimeState()
	if err != nil {
		return err
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	key := fmt.Sprintf("%s/%s:%d", namespace, serviceName, localPort)
	delete(state.PortForwards, key)

	return persistRuntimeState(state)
}

// GetActivePortForwards returns all active port-forwards from runtime state
func GetActivePortForwards() ([]PortForwardInfo, error) {
	state, err := GetRuntimeState()
	if err != nil {
		return nil, err
	}
	
	state.mu.RLock()
	defer state.mu.RUnlock()
	
	result := make([]PortForwardInfo, 0, len(state.PortForwards))
	for _, pf := range state.PortForwards {
		if pf.Active {
			result = append(result, pf)
		}
	}
	
	return result, nil
}

// PrintRuntimeState prints the current runtime state in a human-readable format
func PrintRuntimeState() error {
	state, err := GetRuntimeState()
	if err != nil {
		return err
	}
	
	state.mu.RLock()
	defer state.mu.RUnlock()
	
	fmt.Println("=== Team Gamma Runtime State ===")
	fmt.Printf("Version: %s\n", state.Version)
	fmt.Printf("Last Updated: %s\n\n", state.LastUpdated.Format(time.RFC3339))
	
	fmt.Println("Jenkins:")
	fmt.Printf("  Enabled: %v\n", state.Jenkins.Enabled)
	if state.Jenkins.Enabled {
		fmt.Printf("  URL: %s\n", state.Jenkins.URL)
		fmt.Printf("  UI Port (localhost kubectl port-forward): %d\n", state.Jenkins.UIPort)
		fmt.Printf("  Agent Port: %d\n", state.Jenkins.AgentPort)
		if state.Jenkins.NodePort != 0 {
			fmt.Printf("  NodePort (Kubernetes, internal-only): %d\n", state.Jenkins.NodePort)
		} else {
			fmt.Printf("  NodePort: — (ClusterIP — use UI port / URL)\n")
		}
		fmt.Printf("  Healthy: %v\n", state.Jenkins.Healthy)
		fmt.Printf("  Last Check: %s\n", state.Jenkins.LastCheck.Format(time.RFC3339))
	}
	
	fmt.Println("\nRegistry:")
	fmt.Printf("  Enabled: %v\n", state.Registry.Enabled)
	if state.Registry.Enabled {
		fmt.Printf("  URL: %s\n", state.Registry.URL)
		fmt.Printf("  Host Port: %d\n", state.Registry.HostPort)
		fmt.Printf("  Container Port: %d\n", state.Registry.ContainerPort)
		fmt.Printf("  Container: %s\n", state.Registry.ContainerName)
		fmt.Printf("  Healthy: %v\n", state.Registry.Healthy)
		fmt.Printf("  Last Check: %s\n", state.Registry.LastCheck.Format(time.RFC3339))
	}

	if len(state.Deployments) > 0 {
		fmt.Println("\nDeployments (artifact / rollback):")
		for key, d := range state.Deployments {
			fmt.Printf("  [%s]\n", key)
			fmt.Printf("    Current:  %s\n", d.CurrentImage)
			if d.PreviousImage != "" {
				fmt.Printf("    Previous: %s\n", d.PreviousImage)
			}
			if d.RollbackTarget != "" {
				fmt.Printf("    Rollback target: %s\n", d.RollbackTarget)
			}
			if len(d.StableHistory) > 0 {
				fmt.Printf("    Stable history: %s\n", strings.Join(d.StableHistory, ", "))
			}
			if len(d.RegistryTags) > 0 {
				fmt.Printf("    Registry tags: %s\n", strings.Join(d.RegistryTags, ", "))
			}
			fmt.Printf("    Last result: %s (%s)\n", d.LastResult, d.LastDeployedAt.Format(time.RFC3339))
		}
	}

	return nil
}

// ValidateRuntimeState checks if the runtime state is consistent with actual running services
func ValidateRuntimeState() error {
	state, err := GetRuntimeState()
	if err != nil {
		return err
	}
	
	state.mu.RLock()
	defer state.mu.RUnlock()
	
	// Check if ports in state are actually in use
	if state.Jenkins.Enabled && state.Jenkins.UIPort != 0 {
		// Port validation would go here
	}
	
	if state.Registry.Enabled && state.Registry.HostPort != 0 {
		// Port validation would go here
	}
	
	return nil
}
