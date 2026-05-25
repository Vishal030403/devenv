package cleanup

import (
	"fmt"
	"os/exec"
	"strings"

	"devenv/teamalpha/internal/log"
)

var systemNamespaces = map[string]bool{
	"kube-system":        true,
	"kube-public":        true,
	"kube-node-lease":    true,
	"local-path-storage": true,
	"ingress-nginx":      true,
	"default":            true,
}

func VerifyNamespaceCleanup(namespace string) error {
	log.Running(fmt.Sprintf("Verifying namespace %s removal", namespace))

	if exec.Command("kubectl", "get", "namespace", namespace).Run() != nil {
		log.OK(fmt.Sprintf("Namespace %s removed", namespace))
		return nil
	}

	leftover := summarizeNamespace(namespace)
	if leftover > 0 {
		log.Warning(fmt.Sprintf("Namespace %s still has %d resources", namespace, leftover))
		log.Hint(fmt.Sprintf("Inspect with: kubectl get all -n %s", namespace))
	} else {
		log.Warning(fmt.Sprintf("Namespace %s pending removal", namespace))
	}
	return nil
}

func VerifyIngressCleanup() error {
	log.Running("Verifying ingress cleanup")

	out, err := exec.Command("kubectl", "get", "ingress", "-A", "--no-headers").Output()
	if err != nil {
		log.OK("No ingress controller reachable")
		return nil
	}

	if countUserResources(string(out), 0) > 0 {
		log.Warning(fmt.Sprintf("%d ingress resource(s) remaining", countUserResources(string(out), 0)))
	} else {
		log.OK("Ingress cleanup verified")
	}
	return nil
}

func VerifyServiceCleanup() error {
	log.Running("Verifying service cleanup")

	out, err := exec.Command("kubectl", "get", "svc", "-A", "--no-headers").Output()
	if err != nil {
		log.OK("No services reachable")
		return nil
	}

	if countUserResources(string(out), 0) > 0 {
		log.Warning(fmt.Sprintf("%d service(s) remaining in user namespaces", countUserResources(string(out), 0)))
	} else {
		log.OK("Service cleanup verified")
	}
	return nil
}

func summarizeNamespace(namespace string) int {
	out, err := exec.Command("kubectl", "get", "all", "-n", namespace, "--no-headers").Output()
	if err != nil {
		return 0
	}
	text := strings.TrimSpace(string(out))
	if text == "" {
		return 0
	}
	return len(strings.Split(text, "\n"))
}

func countUserResources(output string, nsField int) int {
	text := strings.TrimSpace(output)
	if text == "" {
		return 0
	}
	count := 0
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) <= nsField {
			continue
		}
		if !systemNamespaces[fields[nsField]] {
			count++
		}
	}
	return count
}
