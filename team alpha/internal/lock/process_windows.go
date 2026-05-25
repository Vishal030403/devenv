//go:build windows

package lock

import (
	"os/exec"
	"strconv"
	"strings"
)

func processIsRunning(pid int) bool {
	out, err := exec.Command(
		"tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/NH", "/FO", "CSV",
	).Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), strconv.Itoa(pid))
}
