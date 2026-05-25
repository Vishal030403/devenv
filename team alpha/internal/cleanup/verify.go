package cleanup

import (
	"os/exec"
	"strings"

	"devenv/teamalpha/internal/log"
)

func VerifyCleanup(clusterName string) error {
	log.Running("Checking leftover clusters")

	out, err := exec.Command("kind", "get", "clusters").Output()
	if err != nil {
		return err
	}

	if strings.Contains(string(out), clusterName) {
		log.Failed("Cluster cleanup incomplete")
		return nil
	}

	log.OK("Cleanup verification successful")
	return nil
}
