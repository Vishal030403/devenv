package installer

import (
	"fmt"
	"os/exec"
	"runtime"

	"devenv/teamalpha/internal/log"
)

func InstallHadolint() error {

	if _, err := exec.LookPath("hadolint"); err == nil {
		log.OK("Hadolint already installed")
		return nil
	}

	log.Info("Installing Hadolint")

	var cmd *exec.Cmd

	switch runtime.GOOS {

	case "darwin":

		cmd = exec.Command("brew", "install", "hadolint")

	case "linux":

		cmd = exec.Command(
			"sh",
			"-c",
			"sudo wget -O /usr/local/bin/hadolint https://github.com/hadolint/hadolint/releases/latest/download/hadolint-Linux-x86_64 && sudo chmod +x /usr/local/bin/hadolint",
		)

	case "windows":

		return fmt.Errorf("hadolint auto-install not supported on windows yet")

	default:

		return fmt.Errorf("unsupported OS")
	}

	out, err := cmd.CombinedOutput()

	if err != nil {

		return fmt.Errorf(
			"hadolint installation failed: %s",
			string(out),
		)
	}

	log.OK("Hadolint installed successfully")

	return nil
}
