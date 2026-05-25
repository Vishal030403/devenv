package utils

import (
	"os/exec"
	"strings"
)

type Deployment struct {
	Namespace string
	Name      string
}

var skippedNamespaces = map[string]bool{
	"kube-system":        true,
	"kube-public":        true,
	"kube-node-lease":    true,
	"local-path-storage": true,
	"ingress-nginx":      true,
}

func GetDeployments() ([]Deployment, error) {
	out, err := exec.Command(
		"kubectl", "get", "deployments", "-A", "--no-headers",
		"-o", "custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name",
	).Output()
	if err != nil {
		return nil, err
	}

	var deployments []Deployment
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ns, name := fields[0], fields[1]
		if skippedNamespaces[ns] {
			continue
		}
		deployments = append(deployments, Deployment{Namespace: ns, Name: name})
	}
	return deployments, nil
}
