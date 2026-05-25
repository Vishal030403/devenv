package cluster

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"devenv/teamalpha/internal/log"
)

func ValidateStorage() error {
	log.Running("Validating storage")

	pvTotal, pvFailed, _ := scanPV()
	pvcTotal, pvcPending, _ := scanPVC()

	log.Info(fmt.Sprintf("PV: %d total, %d failed", pvTotal, pvFailed))
	log.Info(fmt.Sprintf("PVC: %d total, %d pending", pvcTotal, pvcPending))

	if pvFailed > 0 {
		log.Failed(fmt.Sprintf("%d PV(s) failed", pvFailed))
		return errors.New("storage failures detected")
	}

	if pvcPending > 0 {
		log.Warning(fmt.Sprintf("%d PVC(s) pending", pvcPending))
		log.Hint("Inspect with: kubectl get pvc -A")
	}

	log.OK("Storage validated")
	return nil
}

func scanPV() (int, int, error) {
	out, err := exec.Command("kubectl", "get", "pv", "--no-headers").Output()
	if err != nil {
		return 0, 0, err
	}

	text := strings.TrimSpace(string(out))
	if text == "" {
		return 0, 0, nil
	}

	total, failed := 0, 0
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		total++
		if fields[4] == "Failed" {
			failed++
		}
	}
	return total, failed, nil
}

func scanPVC() (int, int, error) {
	out, err := exec.Command("kubectl", "get", "pvc", "-A", "--no-headers").Output()
	if err != nil {
		return 0, 0, err
	}

	text := strings.TrimSpace(string(out))
	if text == "" {
		return 0, 0, nil
	}

	total, pending := 0, 0
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		total++
		if fields[2] == "Pending" {
			pending++
		}
	}
	return total, pending, nil
}
