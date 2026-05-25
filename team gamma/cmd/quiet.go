package cmd

import (
	"fmt"
	"os"
)

// When Team Alpha runs Gamma via go run, it sets DEVENV_QUIET_SUBPROCESS=1
// so high-level "Team Gamma" banners do not duplicate Alpha's own step labels.
func subProcessPrintln(msg string) {
	if os.Getenv("DEVENV_QUIET_SUBPROCESS") == "1" {
		return
	}
	fmt.Println(msg)
}

// subProcessVerbose is true when stdout should include decorative banners
// (false when Team Alpha runs Gamma with DEVENV_QUIET_SUBPROCESS=1).
func subProcessVerbose() bool {
	return os.Getenv("DEVENV_QUIET_SUBPROCESS") != "1"
}
