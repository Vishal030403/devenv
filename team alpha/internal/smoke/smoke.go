package smoke

import (
	"fmt"
	"net/http"
	"time"

	"devenv/teamalpha/internal/log"
	"devenv/teamalpha/internal/utils"
)

func ValidateURL(name, url string) error {
	start := utils.StartTimer()
	log.Running(fmt.Sprintf("Validating %s accessibility", name))

	client := http.Client{Timeout: 10 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		log.Failed(fmt.Sprintf("%s inaccessible", name))
		return err
	}

	if response.StatusCode >= 400 {
		log.Failed(fmt.Sprintf("%s unhealthy (HTTP %d)", name, response.StatusCode))
		return fmt.Errorf("invalid response: HTTP %d", response.StatusCode)
	}

	log.Success(fmt.Sprintf("%s validation completed in %s", name, utils.EndTimer(start)))
	return nil
}
