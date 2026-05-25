package installer

import (
	"fmt"
	"os"
	"os/exec"
)

func InstallKubectl() error {

	if IsMac() {

		cmd := exec.Command(
			"brew",
			"install",
			"kubectl",
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
			"Kubernetes.kubectl",
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
				"kubectl",
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
