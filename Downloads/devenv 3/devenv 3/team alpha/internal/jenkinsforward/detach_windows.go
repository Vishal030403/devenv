//go:build windows

package jenkinsforward

import (
	"os/exec"
	"syscall"
)

// Windows process creation flags (winbase.h) — use numeric values for compatibility across Go versions.
const (
	createDetachedProcess    = 0x00000008 // DETACHED_PROCESS
	createNewProcessGroup    = 0x00000200 // CREATE_NEW_PROCESS_GROUP
	createBreakawayFromJob   = 0x01000000 // CREATE_BREAKAWAY_FROM_JOB
)

// applyDetach keeps kubectl port-forward alive after the devenv process exits on Windows.
func applyDetach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
		CreationFlags: createDetachedProcess |
			createNewProcessGroup |
			createBreakawayFromJob,
	}
}
