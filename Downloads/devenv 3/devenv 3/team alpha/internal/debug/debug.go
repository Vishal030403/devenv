package debug

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"devenv/teamalpha/internal/log"
)

// Known Kubernetes pod failure conditions that signal a broken deployment.
var knownFailures = []string{
	"CrashLoopBackOff",
	"ImagePullBackOff",
	"ErrImagePull",
	"FailedScheduling",
}

// DetectPodFailures checks pod status for known failure patterns.
// Returns a descriptive error if any are found, nil otherwise.
func DetectPodFailures(appName, namespace string) error {
	cmd := exec.Command("kubectl", "get", "pods",
		"-n", namespace, "-l", "app="+appName, "--no-headers")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil // can't query — don't add noise on top of the real error
	}
	for _, failure := range knownFailures {
		if strings.Contains(string(out), failure) {
			return fmt.Errorf("pod failure detected: %s", failure)
		}
	}
	return nil
}

// CollectPodLogs prints the last 50 log lines from pods matching the app label.
func CollectPodLogs(appName, namespace string) {
	log.Info(fmt.Sprintf("Collecting pod logs for %s in namespace %s", appName, namespace))
	cmd := exec.Command("kubectl", "logs",
		"-l", "app="+appName, "-n", namespace, "--tail=50", "--prefix")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Warn(fmt.Sprintf("Could not collect pod logs: %v", err))
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fmt.Println("  " + line)
	}
}

// DescribePods runs kubectl describe on pods for the given app label.
func DescribePods(appName, namespace string) {
	log.Info(fmt.Sprintf("Describing pods for %s in namespace %s", appName, namespace))
	cmd := exec.Command("kubectl", "describe", "pods", "-l", "app="+appName, "-n", namespace)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Warn(fmt.Sprintf("Could not describe pods: %v", err))
		return
	}
	fmt.Println(strings.TrimSpace(string(out)))
}

// ShowEvents prints recent Kubernetes events for the given namespace.
func ShowEvents(namespace string) {
	log.Info(fmt.Sprintf("Fetching Kubernetes events for namespace %s", namespace))
	cmd := exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Warn(fmt.Sprintf("Could not fetch events: %v", err))
		return
	}
	fmt.Println(strings.TrimSpace(string(out)))
}

// CollectDiagnostics gathers full diagnostics for a namespace.
// Pass verbose=true to also print pod descriptions and events.
func CollectDiagnostics(namespace string, verbose bool) {
	log.Info("Collecting diagnostics")

	podOutput, err := exec.Command(
		"kubectl", "get", "pods", "-n", namespace, "--no-headers",
	).Output()
	if err != nil {
		log.Failed("Failed to collect pod diagnostics")
		return
	}

	log.Info("Diagnostics Summary")
	for _, line := range strings.Split(strings.TrimSpace(string(podOutput)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		podName := fields[0]
		status := fields[2]
		restarts := fields[3]

		log.Info(fmt.Sprintf("Pod: %s  Status: %s", podName, status))

		switch status {
		case "CrashLoopBackOff":
			log.Failed(fmt.Sprintf("Pod %s in CrashLoopBackOff", podName))
		case "ImagePullBackOff":
			log.Failed(fmt.Sprintf("Pod %s image pull failed", podName))
		case "Pending":
			log.Warning(fmt.Sprintf("Pod %s pending scheduling", podName))
		case "Error":
			log.Failed(fmt.Sprintf("Pod %s entered error state", podName))
		}

		if restarts != "0" {
			log.Warning(fmt.Sprintf("Restart count: %s", restarts))
		}
	}

	if !verbose {
		log.Info("Pass --verbose for detailed Kubernetes inspection")
		return
	}

	log.Info("Verbose diagnostics enabled")
	commands := [][]string{
		{"kubectl", "get", "pods", "-n", namespace},
		{"kubectl", "get", "events", "-n", namespace},
		{"kubectl", "describe", "pods", "-n", namespace},
	}
	for _, cmdArgs := range commands {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	}
}
