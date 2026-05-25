package preflight

import (
	"os/exec"
	"strings"

	"devenv/teamalpha/internal/log"
	"devenv/teamalpha/pkg/config"
)

func ValidateNoConflictingCluster() error {
	log.Running("Checking for conflicting kind clusters")

	out, err := exec.Command("kind", "get", "clusters").Output()
	if err != nil {
		log.OK("No existing kind clusters")
		return nil
	}

	clusters := strings.Split(strings.TrimSpace(string(out)), "\n")
	conflicts := 0

	for _, c := range clusters {
		name := strings.TrimSpace(c)
		if name == "" || name == config.ClusterName {
			continue
		}
		log.Warning("Found other cluster: " + name)
		conflicts++
	}

	if conflicts > 0 {
		log.Hint("Remove conflicting clusters with: kind delete cluster --name <name>")
	} else {
		log.OK("No conflicting clusters detected")
	}

	return nil
}
