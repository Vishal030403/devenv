package preflight

import (
	"fmt"
	"os/exec"
	"strings"

	"devenv/teamalpha/internal/log"
)

func ValidatePorts(ports []int) error {
	for _, port := range ports {
		log.Running(fmt.Sprintf("Checking port %d", port))

		if isPortFree(port) {
			log.OK(fmt.Sprintf("Port %d available", port))
			continue
		}

		holder := portHolder(port)
		if holder != "" {
			log.Failed(fmt.Sprintf("Port %d in use by container %s", port, holder))
			log.Hint(fmt.Sprintf("Stop with: docker stop %s", holder))
		} else {
			log.Failed(fmt.Sprintf("Port %d in use", port))
		}

		return fmt.Errorf("port %d unavailable", port)
	}
	return nil
}

func portHolder(port int) string {
	out, err := exec.Command(
		"docker", "ps",
		"--filter", fmt.Sprintf("publish=%d", port),
		"--format", "{{.Names}}",
	).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
