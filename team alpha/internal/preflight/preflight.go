package preflight

import (
	"errors"
	"fmt"
	"net"
	"os/exec"

	"devenv/teamalpha/internal/installer"
	"devenv/teamalpha/internal/log"
	"devenv/teamalpha/pkg/config"
)

type Result struct {
	RegistryPort int
}

func RunPreflightChecks() (*Result, error) {
	log.Info("Running preflight checks")

	if err := ValidateDocker(); err != nil {
		return nil, err
	}

	if _, err := exec.LookPath("kubectl"); err != nil {

		log.Warn("kubectl not installed — attempting installation")

		if err := installer.InstallKubectl(); err != nil {

			return nil, errors.New("kubectl installation failed")
		}

		log.OK("kubectl installed successfully")
	}
	log.OK("kubectl available")

	if _, err := exec.LookPath("kind"); err != nil {

		log.Warn("kind not installed — attempting installation")

		if err := installer.InstallKind(); err != nil {

			return nil, errors.New("kind installation failed")
		}

		log.OK("kind installed successfully")
	}
	log.OK("kind available")

	if err := ValidateHelm(); err != nil {
		return nil, err
	}

	if err := ValidateInternet(); err != nil {
		return nil, err
	}

	if err := ValidatePorts([]int{80, 443}); err != nil {
		return nil, err
	}

	port := config.RegistryStartPort
	for !isPortFree(port) {
		port++
	}
	log.OK(fmt.Sprintf("Registry port: %d", port))

	if err := ValidateNoConflictingCluster(); err != nil {
		return nil, err
	}

	return &Result{RegistryPort: port}, nil
}

func isPortFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}
