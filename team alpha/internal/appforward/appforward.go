package appforward

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Record describes a background kubectl port-forward started by Team Alpha for the user application.
type Record struct {
	PID       int    `json:"pid"`
	LocalPort int    `json:"local_port"`
	Service   string `json:"service"`
	Namespace string `json:"namespace"`
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
	return filepath.Join(dir, "alpha-app-portforward.json"), nil
}

// Save persists metadata so StopPrevious can terminate a prior port-forward on the next run or on teardown.
func Save(serviceName, namespace string, localPort, pid int) error {
	p, err := statePath()
	if err != nil {
		return err
	}
	r := Record{PID: pid, LocalPort: localPort, Service: serviceName, Namespace: namespace}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

// StopPrevious terminates the last recorded app port-forward process, if any, and removes the state file.
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
	var r Record
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
