package preflight

import (
	"fmt"
	"os/exec"
	"strings"

	"devenv/teamalpha/internal/log"
)

func ValidateContext() error {
	log.Running("Validating Kubernetes context")

	out, err := exec.Command("kubectl", "config", "current-context").Output()
	if err != nil {
		return err
	}

	current := strings.TrimSpace(string(out))
	log.Info(fmt.Sprintf("Current context: %s", current))

	if current != "kind-dev-cluster" {
		log.Failed("Kubernetes context mismatch")
		return fmt.Errorf("invalid context: %s", current)
	}

	log.OK("Kubernetes context validated")
	return nil
}
