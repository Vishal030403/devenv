package lock

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func lockPath() string {
	return filepath.Join(os.TempDir(), ".devenv.lock")
}

// Acquire creates a lock file with the current PID.
// Returns an error if another devenv run process is already active.
func Acquire() error {
	path := lockPath()
	if data, err := os.ReadFile(path); err == nil {
		pidStr := strings.TrimSpace(string(data))
		pid, parseErr := strconv.Atoi(pidStr)
		if parseErr == nil && processIsRunning(pid) {
			return fmt.Errorf(
				"devenv run is already executing (PID %d) — wait for it to finish, or run `%s status` with this same binary (not plain `devenv` unless it is on your PATH). If nothing is running, remove stale lock: %s",
				pid, os.Args[0], path,
			)
		}
		// Stale lock from a dead process — remove it.
		_ = os.Remove(path)
	}
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0644)
}

// Release removes the lock file.
func Release() {
	_ = os.Remove(lockPath())
}
