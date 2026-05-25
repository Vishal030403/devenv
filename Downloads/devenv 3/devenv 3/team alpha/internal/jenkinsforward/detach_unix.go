//go:build !windows

package jenkinsforward

import (
	"os/exec"
	"syscall"
)

func applyDetach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
}
