package preflight

import (
	"net/http"
	"time"

	"devenv/teamalpha/internal/log"
)

func ValidateInternet() error {
	log.Running("Checking internet connectivity")

	client := http.Client{Timeout: 10 * time.Second}
	if _, err := client.Get("https://github.com"); err != nil {
		log.Failed("Internet unavailable")
		return err
	}

	log.OK("Internet connectivity available")
	return nil
}
