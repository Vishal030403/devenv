package preflight

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"time"

	"devenv/teambeta/pkg/config"
)

type Result struct {
	RegistryPort int
}

func RunPreflightChecks() (*Result, error) {

	fmt.Println("[INFO] Running preflight checks")

	if _, err := exec.LookPath("docker"); err != nil {
		return nil, errors.New("docker not installed. please install docker")
	}

	if err := isDockerRunning(); err != nil {
		return nil, err
	}

	fmt.Println("[OK] Docker available")

	if _, err := exec.LookPath("kubectl"); err != nil {
		return nil, errors.New("kubectl not installed or not in PATH")
	}
	fmt.Println("[OK] kubectl is installed")

	if _, err := exec.LookPath("kind"); err != nil {
		return nil, errors.New("kind not installed or not in PATH")
	}
	fmt.Println("[OK] kind is installed")

	for _, port := range []int{80, 443} {
		if !isPortFree(port) {
			return nil, fmt.Errorf("port %d is in use. free port %d for ingress", port, port)
		}
	}

	fmt.Println("[OK] Ports 80 and 443 available")

	port := config.RegistryStartPort
	for !isPortFree(port) {
		port++
	}

	fmt.Printf("[OK] Registry port: %d\n", port)

	return &Result{
		RegistryPort: port,
	}, nil
}

func isDockerRunning() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "ps")

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return errors.New("docker not responding. restart docker")
		}
		return errors.New("docker daemon not running")
	}

	return nil
}

func isPortFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}
