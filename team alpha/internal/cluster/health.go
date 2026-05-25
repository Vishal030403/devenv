package cluster

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"devenv/teamalpha/internal/log"
)

func ValidateClusterHealth() error {
	if err := waitForNodes(120); err != nil {
		return err
	}
	return checkSystemPods()
}

func waitForNodes(timeoutSeconds int) error {
	log.Running("Verifying node readiness")

	deadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)

	for {
		ready, total, err := readNodeStatus()
		if err == nil && total > 0 && ready == total {
			log.OK(fmt.Sprintf("All %d nodes ready", total))
			return nil
		}

		if time.Now().After(deadline) {
			log.Failed("Nodes not ready in time")
			return errors.New("node readiness timeout")
		}

		time.Sleep(3 * time.Second)
	}
}

func readNodeStatus() (int, int, error) {
	out, err := exec.Command("kubectl", "get", "nodes", "--no-headers").Output()
	if err != nil {
		return 0, 0, err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	ready, total := 0, 0

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		total++
		if fields[1] == "Ready" {
			ready++
		}
	}

	return ready, total, nil
}

func checkSystemPods() error {
	log.Running("Checking kube-system pods")

	out, err := exec.Command("kubectl", "get", "pods", "-n", "kube-system", "--no-headers").Output()
	if err != nil {
		log.Failed("Unable to read kube-system pods")
		return err
	}

	unhealthy := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		status := fields[2]
		if status != "Running" && status != "Completed" {
			unhealthy++
		}
	}

	if unhealthy > 0 {
		log.Failed(fmt.Sprintf("%d kube-system pods unhealthy", unhealthy))
		return errors.New("kube-system unhealthy")
	}

	log.OK("kube-system pods healthy")
	return nil
}
