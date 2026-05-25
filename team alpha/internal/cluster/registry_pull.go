package cluster

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"devenv/teamalpha/internal/log"
	"devenv/teamalpha/pkg/config"
)

// EnsureKindRegistryHTTPPull configures each kind node's containerd to pull
// images from host.docker.internal over plain HTTP. Required on Docker Desktop
// because kubelet resolves "localhost" inside the node, not the Mac host.
func EnsureKindRegistryHTTPPull(hostPort int) error {
	if hostPort <= 0 {
		return nil
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		return nil
	}

	reg := fmt.Sprintf("host.docker.internal:%d", hostPort)
	hostsToml := fmt.Sprintf(`server = "http://%s"

[host."http://%s"]
  capabilities = ["pull", "resolve"]
`, reg, reg)

	dir := "/etc/containerd/certs.d/" + reg
	tmp, err := os.CreateTemp("", "devenv-hosts.toml-*")
	if err != nil {
		return fmt.Errorf("temp hosts.toml: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.WriteString(hostsToml); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	defer os.Remove(tmpPath)

	out, err := exec.Command("kind", "get", "nodes", "--name", config.ClusterName).Output()
	if err != nil {
		return fmt.Errorf("kind get nodes: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		node := strings.TrimSpace(strings.TrimSuffix(line, "*"))
		if node == "" {
			continue
		}
		if err := exec.Command("docker", "exec", node, "mkdir", "-p", dir).Run(); err != nil {
			return fmt.Errorf("mkdir %s on %s: %w", dir, node, err)
		}
		dest := node + ":" + dir + "/hosts.toml"
		if err := exec.Command("docker", "cp", tmpPath, dest).Run(); err != nil {
			return fmt.Errorf("docker cp hosts.toml to %s: %w", node, err)
		}
		// Best-effort reload; kind nodes may not have pidof.
		_ = exec.Command("docker", "exec", node, "sh", "-c",
			"kill -HUP $(pidof containerd 2>/dev/null) $(pgrep containerd 2>/dev/null | head -1) 2>/dev/null || true",
		).Run()
	}
	log.OK(fmt.Sprintf("kind nodes configured for registry pull: %s (http)", reg))
	return nil
}
