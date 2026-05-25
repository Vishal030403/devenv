package preflight

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"devenv/teamalpha/internal/installer"
	"devenv/teamalpha/internal/log"
)

const dockerVersionTimeout = 30 * time.Second

func ValidateDocker() error {
	log.Running("Validating Docker daemon")

	if _, err := exec.LookPath("docker"); err != nil {

		log.Warn("Docker not installed — attempting installation")

		if err := installer.InstallDocker(); err != nil {

			log.Failed("Docker installation failed")

			showInstallHint("docker")

			return errors.New("docker installation failed")
		}

		log.OK("Docker installed successfully")
	}

	version, err := runDockerVersion()
	if err != nil {
		log.Warn("Docker daemon unreachable — attempting recovery")

		if runtime.GOOS == "darwin" {

			exec.Command("open", "-a", "Docker").Start()

		} else if runtime.GOOS == "windows" {

			exec.Command(
				"powershell",
				"-Command",
				"Start-Process Docker Desktop",
			).Start()

		} else if runtime.GOOS == "linux" {

			exec.Command(
				"sudo",
				"systemctl",
				"start",
				"docker",
			).Start()
		}

		timeout := time.After(60 * time.Second)

		ticker := time.NewTicker(5 * time.Second)

		defer ticker.Stop()

		for {

			select {

			case <-timeout:

				log.Failed("Docker daemon unreachable")
				return errors.New("docker daemon unreachable")

			case <-ticker.C:

				cmd := exec.Command("docker", "info")

				if err := cmd.Run(); err == nil {

					version, _ := runDockerVersion()

					log.Info("Docker " + version)
					log.OK("Docker daemon recovered")

					return nil
				}

				log.Info("Waiting for Docker daemon...")
			}
		}
	}
	log.Info("Docker " + version)
	// Server version from `docker version` proves the engine API is reachable;
	// `docker info` is slower and can time out on Desktop without meaning the engine is down.
	log.OK("Docker daemon healthy")
	return nil
}

func runDockerVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dockerVersionTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}").Output()
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(string(out))
	if version == "" {
		return "unknown", nil
	}
	return version, nil
}
