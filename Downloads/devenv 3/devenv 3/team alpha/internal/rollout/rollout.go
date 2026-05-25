package rollout

import (
	"fmt"
	"os/exec"
	"time"

	"devenv/teamalpha/internal/log"
)

func MonitorRollout(deployment, namespace string) error {
	log.Running(fmt.Sprintf("Monitoring %s rollout", deployment))

	start := time.Now()

	cmd := exec.Command(
		"kubectl", "rollout", "status",
		fmt.Sprintf("deployment/%s", deployment),
		"-n", namespace,
		"--timeout=120s",
	)

	if err := cmd.Run(); err != nil {
		log.Failed(fmt.Sprintf("%s rollout failed", deployment))
		return err
	}

	log.Success(fmt.Sprintf("%s rollout completed in %s", deployment, time.Since(start)))
	return nil
}
