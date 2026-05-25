package cluster

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"devenv/teamalpha/internal/log"
)

func EnsureNamespace(name string) error {
	log.Running(fmt.Sprintf("Ensuring namespace %s", name))

	if namespaceExists(name) {
		log.OK(fmt.Sprintf("Namespace %s already present", name))
		return nil
	}

	if err := exec.Command("kubectl", "create", "namespace", name).Run(); err != nil {
		log.Failed(fmt.Sprintf("Failed to create namespace %s", name))
		return errors.New("namespace creation failed")
	}

	log.OK(fmt.Sprintf("Namespace %s created", name))
	return nil
}

func ValidateNamespace(name string) error {
	log.Running(fmt.Sprintf("Validating namespace %s", name))

	phase, err := namespacePhase(name)
	if err != nil {
		log.Failed(fmt.Sprintf("Namespace %s not found", name))
		return err
	}

	if phase == "Terminating" {
		log.Warning(fmt.Sprintf("Namespace %s stuck terminating", name))
		log.Hint(fmt.Sprintf("Inspect with: kubectl get ns %s -o yaml", name))
		return errors.New("namespace terminating")
	}

	if phase != "Active" {
		log.Failed(fmt.Sprintf("Namespace %s phase: %s", name, phase))
		return errors.New("namespace not active")
	}

	log.OK(fmt.Sprintf("Namespace %s active", name))
	return nil
}

func namespaceExists(name string) bool {
	return exec.Command("kubectl", "get", "namespace", name).Run() == nil
}

func namespacePhase(name string) (string, error) {
	out, err := exec.Command(
		"kubectl", "get", "namespace", name, "-o", "jsonpath={.status.phase}",
	).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
