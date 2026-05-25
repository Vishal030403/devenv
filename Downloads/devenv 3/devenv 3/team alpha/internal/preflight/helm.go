package preflight

import (
	"errors"
	"os/exec"

	"devenv/teamalpha/internal/installer"
	"devenv/teamalpha/internal/log"
)

func ValidateHelm() error {

	log.Running("Checking Helm installation")

	if _, err := exec.LookPath("helm"); err != nil {

		log.Warn("Helm not installed — attempting installation")

		if err := installer.InstallHelm(); err != nil {

			log.Failed("Helm installation failed")

			return errors.New("helm installation failed")
		}

		log.OK("Helm installed successfully")
	}

	log.OK("Helm available")

	return nil
}
