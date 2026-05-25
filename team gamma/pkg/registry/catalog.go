// Package registry provides public helpers for the local Docker Registry v2 API
// (importable from Team Alpha and other modules — not internal/).
package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ListRepositoryTags returns tag names for a repository in the local registry (v2 API).
func ListRepositoryTags(hostPort int, repository string) ([]string, error) {
	repository = strings.TrimPrefix(strings.TrimSpace(repository), "/")
	if repository == "" {
		return nil, fmt.Errorf("empty repository name")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d/v2/%s/tags/list", hostPort, repository)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("registry tags list HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Tags, nil
}

// LogRepositoryCatalog prints stored tags for a repository (deployment artifact visibility).
func LogRepositoryCatalog(hostPort int, repository string) {
	tags, err := ListRepositoryTags(hostPort, repository)
	if err != nil {
		fmt.Printf("[WARN] Registry catalog for %s: %v\n", repository, err)
		return
	}
	if len(tags) == 0 {
		fmt.Printf("[INFO] Registry catalog for %s: (no tags yet)\n", repository)
		return
	}
	fmt.Printf("[INFO] Registry catalog for %s: %s\n", repository, strings.Join(tags, ", "))
}
