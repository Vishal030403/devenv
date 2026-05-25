package jenkins

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// validateJenkinsUILoginURL checks that GET url returns a plausible Jenkins login page,
// so we don't treat an unrelated listener on localhost:8080 as "Jenkins up".
func validateJenkinsUILoginURL(raw string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(raw)
	if err != nil {
		return fmt.Errorf("Jenkins UI not reachable: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))

	if resp.StatusCode >= 500 {
		return fmt.Errorf("Jenkins HTTP error: HTTP %d", resp.StatusCode)
	}
	if !responseLooksLikeJenkins(resp, body) {
		return fmt.Errorf("%s responds but does not look like Jenkins (wrong process on UI port or broken port-forward)", raw)
	}
	return nil
}

func responseLooksLikeJenkins(resp *http.Response, body []byte) bool {
	if strings.TrimSpace(resp.Header.Get("X-Jenkins")) != "" {
		return true
	}
	s := strings.ToLower(string(body))
	return strings.Contains(s, "jenkins")
}
