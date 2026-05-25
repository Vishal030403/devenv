package jenkinsforward

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	pkgconfig "devenv/teamalpha/pkg/config"
)

const (
	namespace   = "jenkins"
	serviceName = "jenkins"
	remotePort  = 8080
)

type record struct {
	PID       int `json:"pid"`
	LocalPort int `json:"local_port"`
}

func statePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".devenv")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "team-alpha-jenkins-portforward.json"), nil
}

// StopPrevious kills the kubectl port-forward Alpha started previously (does not touch subprocesses Gamma started).
func StopPrevious() error {
	p, err := statePath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var r record
	if err := json.Unmarshal(data, &r); err != nil {
		_ = os.Remove(p)
		return nil
	}
	_ = os.Remove(p)
	if r.PID <= 0 {
		return nil
	}
	proc, err := os.FindProcess(r.PID)
	if err != nil {
		return nil
	}
	_ = proc.Kill()
	_, _ = proc.Wait()
	return nil
}

func localPortForJenkinsUI() int { return pkgconfig.JenkinsUIPort }

// jenkinsLandingLooksOK checks Jenkins /login without logging (used before starting duplicate forwards).
func jenkinsLandingLooksOK(port int) bool {
	url := fmt.Sprintf("http://127.0.0.1:%d/login", port)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<10))
	if resp.StatusCode >= 500 {
		return false
	}
	if strings.TrimSpace(resp.Header.Get("X-Jenkins")) != "" {
		return true
	}
	return strings.Contains(strings.ToLower(string(body)), "jenkins")
}

// EnsureStandaloneAfterGamma starts kubectl port-forward from the Alpha process tree so Jenkins at 127.0.0.1:8080 survives
// after the ephemeral `go run` Team Gamma subprocess exits (observed mainly on Windows).
func EnsureStandaloneAfterGamma() error {
	lp := localPortForJenkinsUI()
	if jenkinsLandingLooksOK(lp) {
		return nil
	}

	if err := StopPrevious(); err != nil {
		return fmt.Errorf("clear prior Alpha Jenkins forward: %w", err)
	}

	// Tiny settle after StopPrevious freeing the port on some OS kernels.
	time.Sleep(200 * time.Millisecond)

	mapping := fmt.Sprintf("%d:%d", lp, remotePort)
	cmd := exec.Command("kubectl", "port-forward",
		"--address", "127.0.0.1",
		"-n", namespace,
		fmt.Sprintf("svc/%s", serviceName),
		mapping,
	)
	applyDetach(cmd)
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start kubectl Jenkins port-forward: %w", err)
	}
	pid := cmd.Process.Pid

	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		if jenkinsLandingLooksOK(lp) {
			if err := SavePID(pid, lp); err != nil {
				_ = cmd.Process.Kill()
				return fmt.Errorf("persist Jenkins port-forward PID: %w", err)
			}
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	_ = cmd.Process.Kill()
	return fmt.Errorf("Jenkins login page did not become reachable on 127.0.0.1:%d within timeout", lp)
}

// SavePID records the kubectl pid so Down() / next run can stop our forward only.
func SavePID(pid, localPort int) error {
	p, err := statePath()
	if err != nil {
		return err
	}
	r := record{PID: pid, LocalPort: localPort}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}
