package installer

import (
	"fmt"
	"os"
	"os/exec"
)

func InstallKind() error {

	if IsMac() {

		cmd := exec.Command(
			"brew",
			"install",
			"kind",
		)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		return cmd.Run()
	}

	if IsWindows() {

		cmd := exec.Command(
			"winget",
			"install",
			"Kubernetes.kind",
		)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		return cmd.Run()
	}

	if IsLinux() {

		if _, err := exec.LookPath("apt-get"); err == nil {

			cmd := exec.Command(
				"bash",
				"-c",
				"curl -Lo ./kind https://kind.sigs.k8s.io/dl/latest/kind-linux-amd64 && chmod +x ./kind && sudo mv ./kind /usr/local/bin/kind",
			)

			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			return cmd.Run()
		}
	}

	return fmt.Errorf("unsupported operating system")
}
