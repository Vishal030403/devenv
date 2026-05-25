package installer

import (
	"fmt"
	"os/exec"
	"runtime"

	"devenv/teamalpha/internal/log"
)

func InstallTrivy() error {

	if _, err := exec.LookPath("trivy"); err == nil {
		log.OK("Trivy already installed")
		return nil
	}

	log.Info("Installing Trivy")

	var cmd *exec.Cmd

	switch runtime.GOOS {

	case "darwin":

		cmd = exec.Command("brew", "install", "trivy")

	case "linux":

		cmd = exec.Command(
			"sh",
			"-c",
			"sudo apt-get update && sudo apt-get install -y wget apt-transport-https gnupg lsb-release && wget -qO - https://aquasecurity.github.io/trivy-repo/deb/public.key | sudo apt-key add - && echo deb https://aquasecurity.github.io/trivy-repo/deb $(lsb_release -sc) main | sudo tee /etc/apt/sources.list.d/trivy.list && sudo apt-get update && sudo apt-get install -y trivy",
		)

	case "windows":

		cmd = exec.Command(
			"powershell",
			"-Command",
			"choco install trivy -y",
		)

	default:

		return fmt.Errorf("unsupported OS")
	}

	out, err := cmd.CombinedOutput()

	if err != nil {

		return fmt.Errorf(
			"trivy installation failed: %s",
			string(out),
		)
	}

	log.OK("Trivy installed successfully")

	return nil
}
