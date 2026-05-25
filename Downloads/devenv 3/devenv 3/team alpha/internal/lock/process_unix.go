//go:build !windows

package lock

import (
	"os"
	"syscall"
)

func processIsRunning(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}
