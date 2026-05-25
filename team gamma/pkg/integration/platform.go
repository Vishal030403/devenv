package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"devenv-gamma/pkg/state"
)

const (
	PlatformNamespace = "devenv-system"
	PlatformConfigMap = "devenv-platform-config"
)

// PlatformConfig is shared across Team Alpha (devenv) and Jenkins (in-cluster).
type PlatformConfig struct {
	RegistryHostPort int    `json:"registry_host_port"`
	RegistryPullHost string `json:"registry_pull_host"` // host.docker.internal:PORT — Kind nodes + Jenkins DinD push/pull
	RegistryPushHost string `json:"registry_push_host"` // 127.0.0.1:PORT — host devenv run docker push
	RegistryInClusterHost string `json:"registry_in_cluster_host"` // same as pull host; Jenkins agent pods
	ClusterName      string `json:"cluster_name"`
	JenkinsURL       string `json:"jenkins_url"`
	JenkinsJob       string `json:"jenkins_job"`
	DefaultAppName   string `json:"default_app_name"`
	ProjectPath      string `json:"project_path"`
}

// RegistryPullHostForK8s returns the registry hostname kind nodes use to pull images.
func RegistryPullHostForK8s(hostPort int) string {
	if h := strings.TrimSpace(os.Getenv("DEVENV_REGISTRY_PULL_HOST")); h != "" {
		return h
	}
	if h := strings.TrimSpace(os.Getenv("DEVENV_REGISTRY")); h != "" {
		return h
	}
	switch runtime.GOOS {
	case "darwin", "windows":
		return fmt.Sprintf("host.docker.internal:%d", hostPort)
	default:
		return fmt.Sprintf("localhost:%d", hostPort)
	}
}

// LoadPlatformConfigFromState builds config from Gamma runtime-state.json on the host.
func LoadPlatformConfigFromState(projectPath, clusterName, defaultApp string) (PlatformConfig, error) {
	cfg := PlatformConfig{
		ClusterName:    clusterName,
		JenkinsURL:     "http://127.0.0.1:8080",
		JenkinsJob:     "devenv/local-ci-cd",
		DefaultAppName: defaultApp,
		ProjectPath:    projectPath,
	}

	if port, err := state.GetRegistryPort(); err == nil && port > 0 {
		cfg.RegistryHostPort = port
		cfg.RegistryPushHost = fmt.Sprintf("127.0.0.1:%d", port)
	}
	if jenkinsURL, err := state.GetJenkinsURL(); err == nil && strings.TrimSpace(jenkinsURL) != "" {
		cfg.JenkinsURL = jenkinsURL
	}

	if cfg.RegistryHostPort == 0 {
		cfg.RegistryHostPort = 5000
		cfg.RegistryPushHost = "127.0.0.1:5000"
	}
	cfg.RegistryPullHost = RegistryPullHostForK8s(cfg.RegistryHostPort)
	cfg.RegistryInClusterHost = cfg.RegistryPullHost
	return cfg, nil
}

// SyncPlatformConfigToCluster writes/updates the shared ConfigMap consumed by Jenkins agents and jobs.
func SyncPlatformConfigToCluster(cfg PlatformConfig) error {
	if err := ensureNamespace(PlatformNamespace); err != nil {
		return err
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	manifest := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
  labels:
    app.kubernetes.io/part-of: devenv
data:
  platform.json: |
%s
`, PlatformConfigMap, PlatformNamespace, indentJSON(string(data)))

	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("apply platform configmap: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func indentJSON(s string) string {
	lines := strings.Split(s, "\n")
	var b strings.Builder
	for _, line := range lines {
		b.WriteString("    ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func ensureNamespace(ns string) error {
	cmd := exec.Command("kubectl", "get", "namespace", ns)
	if err := cmd.Run(); err == nil {
		return nil
	}
	cmd = exec.Command("kubectl", "create", "namespace", ns)
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "AlreadyExists") {
		return fmt.Errorf("create namespace %s: %s", ns, strings.TrimSpace(string(out)))
	}
	return nil
}
