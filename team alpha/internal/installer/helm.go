package installer

import (
	"fmt"
	"os"
	"os/exec"
)

func InstallHelm() error {

	if IsMac() {

		cmd := exec.Command(
			"brew",
			"install",
			"helm",
		)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		return cmd.Run()
	}

	if IsWindows() {

		cmd := exec.Command(
			"winget",
			"install",
			"Helm.Helm",
		)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		return cmd.Run()
	}

	if IsLinux() {

		cmd := exec.Command(
			"bash",
			"-c",
			"curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash",
		)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		return cmd.Run()
	}

	return fmt.Errorf("unsupported operating system")
}
