package installer

import (
	"fmt"
	"os"
	"os/exec"
)

func InstallDocker() error {

	if IsMac() {

		if _, err := exec.LookPath("brew"); err != nil {
			return fmt.Errorf("homebrew not installed")
		}

		cmd := exec.Command(
			"brew",
			"install",
			"--cask",
			"docker",
		)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		return cmd.Run()
	}

	if IsWindows() {

		cmd := exec.Command(
			"winget",
			"install",
			"-e",
			"--id",
			"Docker.DockerDesktop",
		)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		return cmd.Run()
	}

	if IsLinux() {

		if _, err := exec.LookPath("apt-get"); err == nil {

			cmd := exec.Command(
				"sudo",
				"apt-get",
				"install",
				"docker.io",
				"-y",
			)

			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			return cmd.Run()
		}

		return fmt.Errorf("unsupported linux package manager")
	}

	return fmt.Errorf("unsupported operating system")
}
