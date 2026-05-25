package kubeport

import (
	"os/exec"
	"strconv"
	"strings"
)

// KubernetesServicePort returns Service.spec.ports[0].port for kubectl port-forward.
// Fallback 80 when the cluster is unreachable or parsing fails — matches typical nginx ingress patterns.
func KubernetesServicePort(namespace, serviceName string) int {
	cmd := exec.Command("kubectl", "get", "svc", serviceName,
		"-n", namespace,
		"-o", "jsonpath={.spec.ports[0].port}")
	out, err := cmd.Output()
	if err != nil {
		return 80
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return 80
	}
	port, err := strconv.Atoi(s)
	if err != nil || port <= 0 || port > 65535 {
		return 80
	}
	return port
}
