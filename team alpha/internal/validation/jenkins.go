package validation

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"devenv/teamalpha/internal/log"
	"devenv/teamalpha/internal/utils"
)

func ValidateJenkins(port int) error {
	url := fmt.Sprintf("http://127.0.0.1:%d/login", port)
	start := utils.StartTimer()
	log.Running("Validating Jenkins accessibility")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		log.Failed("Jenkins inaccessible")
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))

	if resp.StatusCode >= 500 {
		log.Failed(fmt.Sprintf("Jenkins unhealthy (HTTP %d)", resp.StatusCode))
		return fmt.Errorf("invalid response: HTTP %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Jenkins") == "" {
		s := strings.ToLower(string(body))
		if !strings.Contains(s, "jenkins") {
			log.Failed("Something on the Jenkins UI port responds but does not appear to be Jenkins (try http://127.0.0.1)")
			return fmt.Errorf("unexpected response body for Jenkins UI")
		}
	}

	log.Success(fmt.Sprintf("Jenkins validation completed in %s", utils.EndTimer(start)))
	return nil
}
