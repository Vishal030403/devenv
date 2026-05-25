package preflight

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"devenv/teamalpha/internal/log"
)

var wingetIDs = map[string]string{
	"kind":    "Kubernetes.kind",
	"kubectl": "Kubernetes.kubectl",
	"helm":    "Helm.Helm",
}

var brewFormulas = map[string]string{
	"kind":    "kind",
	"kubectl": "kubectl",
	"helm":    "helm",
}

var installHints = map[string]string{
	"kind":    "https://kind.sigs.k8s.io/docs/user/quick-start/#installation",
	"kubectl": "https://kubernetes.io/docs/tasks/tools/",
	"helm":    "https://helm.sh/docs/intro/install/",
	"docker":  "https://docs.docker.com/desktop/install/windows-install/",
}

// tryAutoInstall attempts to install a missing CLI tool automatically.
// Returns true if the tool is available after the attempt.
func tryAutoInstall(tool string) bool {
	log.Info(fmt.Sprintf("Attempting to auto-install %s ...", tool))

	switch runtime.GOOS {
	case "windows":
		return installWindows(tool)
	case "darwin":
		return installMac(tool)
	default:
		// Linux: try go install for kind as last resort, otherwise hint
		if tool == "kind" && tryGoInstall("sigs.k8s.io/kind@latest") {
			return toolAvailable(tool)
		}
		showInstallHint(tool)
		return false
	}
}

func installWindows(tool string) bool {
	// 1. Try winget (available on Windows 10/11)
	if _, err := exec.LookPath("winget"); err == nil {
		if pkg, ok := wingetIDs[tool]; ok {
			log.Running(fmt.Sprintf("Installing %s via winget ...", tool))
			cmd := exec.Command(
				"winget", "install", "-e", "--id", pkg,
				"--accept-source-agreements", "--accept-package-agreements",
			)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err == nil {
				log.OK(fmt.Sprintf("%s installed via winget", tool))
				return toolAvailable(tool)
			}
			log.Warn(fmt.Sprintf("winget install failed for %s", tool))
		}
	}

	// 2. Fallback: go install (works for kind since Go is present)
	if tool == "kind" && tryGoInstall("sigs.k8s.io/kind@latest") {
		return toolAvailable(tool)
	}

	showInstallHint(tool)
	return false
}

func installMac(tool string) bool {
	if _, err := exec.LookPath("brew"); err == nil {
		if formula, ok := brewFormulas[tool]; ok {
			log.Running(fmt.Sprintf("Installing %s via Homebrew ...", tool))
			cmd := exec.Command("brew", "install", formula)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err == nil {
				log.OK(fmt.Sprintf("%s installed via Homebrew", tool))
				return toolAvailable(tool)
			}
			log.Warn(fmt.Sprintf("Homebrew install failed for %s", tool))
		}
	}

	if tool == "kind" && tryGoInstall("sigs.k8s.io/kind@latest") {
		return toolAvailable(tool)
	}

	showInstallHint(tool)
	return false
}

func tryGoInstall(pkg string) bool {
	log.Running(fmt.Sprintf("Installing via: go install %s", pkg))
	cmd := exec.Command("go", "install", pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err == nil {
		log.OK("Installed via go install")
		return true
	}
	log.Warn("go install failed")
	return false
}

func showInstallHint(tool string) {
	if url, ok := installHints[tool]; ok {
		log.Hint(fmt.Sprintf("Install %s manually: %s", tool, url))
	}
}

func toolAvailable(tool string) bool {
	_, err := exec.LookPath(tool)
	return err == nil
}
