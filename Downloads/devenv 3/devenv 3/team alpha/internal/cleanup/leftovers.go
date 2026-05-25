package cleanup

import (
	"fmt"
	"os/exec"
	"strings"

	appconfig "devenv/teamalpha/internal/config"
	"devenv/teamalpha/internal/log"
)

func CleanupLeftovers() error {
	log.Running("Scanning for leftover resources")

	containers := danglingContainers(appconfig.TargetClusterName())
	if len(containers) > 0 {
		log.Warning(fmt.Sprintf("%d kind container(s) lingering", len(containers)))
		for _, c := range containers {
			log.Hint("Remove with: docker rm -f " + c)
		}
	} else {
		log.OK("No dangling kind containers")
	}

	volumes := danglingVolumes(appconfig.TargetClusterName())
	if len(volumes) > 0 {
		log.Warning(fmt.Sprintf("%d kind volume(s) lingering", len(volumes)))
		for _, v := range volumes {
			log.Hint("Remove with: docker volume rm " + v)
		}
	} else {
		log.OK("No dangling kind volumes")
	}

	contexts := leftoverContexts(appconfig.TargetClusterName())
	for _, ctx := range contexts {
		log.Warning("Leftover kubeconfig context: " + ctx)
		log.Hint("Remove with: kubectl config delete-context " + ctx)
	}
	if len(contexts) == 0 {
		log.OK("No leftover kubeconfig contexts")
	}

	return nil
}

func danglingContainers(clusterName string) []string {
	out, err := exec.Command(
		"docker", "ps", "-a",
		"--filter", "label=io.x-k8s.kind.cluster="+clusterName,
		"--format", "{{.Names}}",
	).Output()
	if err != nil {
		return nil
	}
	return splitNonEmpty(string(out))
}

func danglingVolumes(clusterName string) []string {
	out, err := exec.Command(
		"docker", "volume", "ls",
		"--filter", "label=io.x-k8s.kind.cluster="+clusterName,
		"--format", "{{.Name}}",
	).Output()
	if err != nil {
		return nil
	}
	return splitNonEmpty(string(out))
}

func leftoverContexts(clusterName string) []string {
	out, err := exec.Command("kubectl", "config", "get-contexts", "-o", "name").Output()
	if err != nil {
		return nil
	}

	expected := "kind-" + clusterName
	var leftover []string
	for _, c := range splitNonEmpty(string(out)) {
		if c == expected {
			leftover = append(leftover, c)
		}
	}
	return leftover
}

func splitNonEmpty(s string) []string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if t := strings.TrimSpace(line); t != "" {
			out = append(out, t)
		}
	}
	return out
}
