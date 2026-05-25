package gammastate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// runtimeStateFile mirrors the subset of Team Gamma's runtime-state.json
// that Alpha needs (same path as devenv-gamma/pkg/state).
type runtimeStateFile struct {
	Registry struct {
		Enabled  bool `json:"enabled"`
		HostPort int  `json:"host_port"`
	} `json:"registry"`
	Jenkins struct {
		Enabled bool   `json:"enabled"`
		UIPort  int    `json:"ui_port"`
		URL     string `json:"url"`
	} `json:"jenkins"`
}

func readTeamGammaState() (runtimeStateFile, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return runtimeStateFile{}, false
	}
	p := filepath.Join(home, ".devenv", "team-gamma", "runtime-state.json")
	data, err := os.ReadFile(p)
	if err != nil {
		return runtimeStateFile{}, false
	}
	var f runtimeStateFile
	if err := json.Unmarshal(data, &f); err != nil {
		return runtimeStateFile{}, false
	}
	return f, true
}

// RegistryHostPort returns the host port Team Gamma recorded for the Docker registry.
// ok is false if the file is missing, invalid, or registry is not enabled.
func RegistryHostPort() (port int, ok bool) {
	f, ok := readTeamGammaState()
	if !ok || !f.Registry.Enabled || f.Registry.HostPort <= 0 {
		return 0, false
	}
	return f.Registry.HostPort, true
}

// JenkinsUIPort returns the localhost port for the Jenkins UI (Team Gamma port-forward), if recorded.
func JenkinsUIPort() (port int, ok bool) {
	f, ok := readTeamGammaState()
	if !ok || !f.Jenkins.Enabled {
		return 0, false
	}
	if f.Jenkins.UIPort > 0 {
		return f.Jenkins.UIPort, true
	}
	return 0, false
}

// JenkinsAccessURL returns the Jenkins URL from Gamma runtime state when present (typically http://127.0.0.1:8080).
func JenkinsAccessURL() (url string, ok bool) {
	f, ok := readTeamGammaState()
	if !ok || !f.Jenkins.Enabled || f.Jenkins.URL == "" {
		return "", false
	}
	return f.Jenkins.URL, true
}

const (
	canonicalJenkinsLocalHost  = "127.0.0.1"
	canonicalJenkinsLocalPort = 8080 // Team Gamma kubectl UI port-forward; matches pkg/config JenkinsUIPort
	kubernetesNodePortMin       = 30000
	kubernetesNodePortMax       = 32767
)

// EffectiveJenkinsLocalPort is the host port Alpha should validate and advertise for Jenkins in a browser.
// Older Gamma releases persisted Kubernetes NodePorts (30000–32767) in ui_port — those must not drive localhost UX.
func EffectiveJenkinsLocalPort() int {
	p, ok := JenkinsUIPort()
	if !ok || p <= 0 {
		return canonicalJenkinsLocalPort
	}
	if p >= kubernetesNodePortMin && p <= kubernetesNodePortMax {
		return canonicalJenkinsLocalPort
	}
	return p
}

// EffectiveJenkinsLocalURL is the canonical Jenkins UI URL (never a Kubernetes NodePort URL).
func EffectiveJenkinsLocalURL() string {
	return fmt.Sprintf("http://%s:%d", canonicalJenkinsLocalHost, EffectiveJenkinsLocalPort())
}

