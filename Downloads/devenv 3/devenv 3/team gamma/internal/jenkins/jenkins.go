package jenkins

import (
	"devenv-gamma/pkg/k8sutil"
	"devenv-gamma/pkg/netutil"
	"devenv-gamma/pkg/state"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	jenkinsNamespace       = "jenkins"
	jenkinsReleaseName     = "jenkins"
	jenkinsServiceName     = "jenkins"
	jenkinsLoopbackHost    = "127.0.0.1" // match kubectl port-forward --address (see k8sutil)
	jenkinsLocalUIPort     = 8080        // fixed host port for kubectl port-forward (user-facing)
	jenkinsServicePort     = 8080        // container/service port inside the cluster
	validationLocalPort    = 18080       // temporary port for HTTP checks without binding the UI port
	maxRetries             = 3
	retryDelay             = 5 * time.Second
)

func jenkinsUIURL(port int, pathSuffix string) string {
	if pathSuffix != "" && !strings.HasPrefix(pathSuffix, "/") {
		pathSuffix = "/" + pathSuffix
	}
	return fmt.Sprintf("http://%s:%d%s", jenkinsLoopbackHost, port, pathSuffix)
}

// EnsureJenkinsPortForward starts (or reuses) a persistent port-forward so Jenkins is always at http://127.0.0.1:8080.
func EnsureJenkinsPortForward() error {
	deployed, err := isJenkinsHelmReleasePresent()
	if err != nil {
		return err
	}
	if !deployed {
		return errors.New("Jenkins is not deployed")
	}

	mgr := k8sutil.GetPortForwardManager()
	if existing := mgr.Get(jenkinsNamespace, jenkinsServiceName, jenkinsLocalUIPort); existing != nil && existing.IsRunning() {
		fmt.Printf("[INFO] Jenkins port-forward already active on %s:%d\n", jenkinsLoopbackHost, jenkinsLocalUIPort)
		fmt.Printf("[INFO] Jenkins mapped to %s:%d\n", jenkinsLoopbackHost, jenkinsLocalUIPort)
		_ = syncJenkinsURLInState()
		return nil
	}

	if !netutil.IsPortAvailable(jenkinsLocalUIPort) {
		checkURL := jenkinsUIURL(jenkinsLocalUIPort, "/login")
		if err := validateJenkinsUILoginURL(checkURL, 8*time.Second); err == nil {
			fmt.Printf("[INFO] Jenkins UI already reachable on %s:%d\n", jenkinsLoopbackHost, jenkinsLocalUIPort)
			fmt.Printf("[INFO] Jenkins mapped to %s:%d\n", jenkinsLoopbackHost, jenkinsLocalUIPort)
			_ = syncJenkinsURLInState()
			return nil
		}
		return fmt.Errorf("%s:%d is required for the Jenkins UI but another process occupies it (and it does not respond as Jenkins)", jenkinsLoopbackHost, jenkinsLocalUIPort)
	}

	fmt.Println("[INFO] Starting Jenkins localhost exposure...")
	cfg := k8sutil.PortForwardConfig{
		Namespace:   jenkinsNamespace,
		ServiceName: jenkinsServiceName,
		LocalPort:   jenkinsLocalUIPort,
		RemotePort:  jenkinsServicePort,
		Timeout:     90 * time.Second,
		AppName:     "jenkins",
	}
	pf, err := k8sutil.StartPersistentPortForward(cfg)
	if err != nil {
		return fmt.Errorf("failed to start Jenkins port-forward: %w", err)
	}
	if err := state.RegisterPortForward(jenkinsNamespace, jenkinsServiceName, "jenkins", jenkinsLocalUIPort, jenkinsServicePort, pf.GetPID()); err != nil {
		fmt.Printf("[WARN] Could not record port-forward in runtime state: %v\n", err)
	}
	if err := syncJenkinsURLInState(); err != nil {
		fmt.Printf("[WARN] Could not sync Jenkins URL in runtime state: %v\n", err)
	}
	fmt.Printf("[INFO] Jenkins mapped to %s:%d\n", jenkinsLoopbackHost, jenkinsLocalUIPort)
	fmt.Printf("[SUCCESS] Jenkins available at %s\n", jenkinsUIURL(jenkinsLocalUIPort, ""))
	return nil
}

func syncJenkinsURLInState() error {
	return state.UpdateJenkinsState(func(js *state.JenkinsState) {
		js.Enabled = true
		js.UIPort = jenkinsLocalUIPort
		js.URL = jenkinsUIURL(jenkinsLocalUIPort, "")
		js.NodePort = 0
	})
}

// DeployJenkins deploys Jenkins (ClusterIP in-cluster) and exposes the UI only via kubectl port-forward on 127.0.0.1:8080.
func DeployJenkins() error {
	fmt.Println("[INFO] Deploying Jenkins into Kubernetes (in-cluster ClusterIP service; Jenkins UI via kubectl port-forward on 127.0.0.1:8080)")
	
	// Validate prerequisites
	if err := ensureHelmInstalled(); err != nil {
		return err
	}

	if err := ensureKubectlWorking(); err != nil {
		return fmt.Errorf("kubectl not working: %w", err)
	}

	// Ensure namespace
	if err := ensureNamespace(); err != nil {
		return err
	}

	// Add Helm repo with retry
	if err := addJenkinsHelmRepoWithRetry(); err != nil {
		return err
	}

	// Check if already deployed
	existing, err := isJenkinsHelmReleasePresent()
	if err != nil {
		return fmt.Errorf("failed to check Jenkins status: %w", err)
	}

	if existing {
		fmt.Println("[INFO] Jenkins Helm release already present, verifying health...")

		if err := waitForJenkinsPod(); err != nil {
			fmt.Println("[WARN] Jenkins pod not healthy, reinstalling...")
			if err := uninstallJenkins(); err != nil {
				return fmt.Errorf("failed to remove unhealthy Jenkins: %w", err)
			}
		} else if err := verifyJenkinsService(); err != nil {
			fmt.Printf("[WARN] Jenkins service validation failed: %v\n", err)
			fmt.Println("[INFO] Reinstalling Jenkins...")
			if err := uninstallJenkins(); err != nil {
				return fmt.Errorf("failed to remove Jenkins with invalid service: %w", err)
			}
		} else {
			fmt.Println("[INFO] Validating Jenkins HTTP accessibility (diagnostic port-forward)...")
			if err := validateJenkinsViaPortForward(); err != nil {
				fmt.Printf("[WARN] Jenkins HTTP validation failed: %v\n", err)
				fmt.Println("[WARN] Continuing with existing deployment (pod healthy)")
			}
			if err := EnsureJenkinsPortForward(); err != nil {
				return err
			}
			if err := updateJenkinsRuntimeState(true); err != nil {
				fmt.Printf("[WARN] Failed to update runtime state: %v\n", err)
			}
			fmt.Println("[OK] Login: admin / admin123")
			
			return nil
		}
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			fmt.Printf("[INFO] Retry %d/%d: Installing Jenkins (ClusterIP)...\n", attempt, maxRetries)
			time.Sleep(retryDelay)
			_ = uninstallJenkins()
		}

		if err := installJenkinsClusterIP(); err != nil {
			lastErr = err
			fmt.Printf("[WARN] Attempt %d failed: %v\n", attempt, err)
			continue
		}

		if err := waitForJenkinsPod(); err != nil {
			lastErr = err
			fmt.Printf("[WARN] Jenkins installed but pod not ready: %v\n", err)
			continue
		}

		if err := verifyJenkinsService(); err != nil {
			lastErr = err
			fmt.Printf("[WARN] Jenkins service validation failed: %v\n", err)
			continue
		}

		fmt.Println("[INFO] Validating Jenkins HTTP accessibility (diagnostic port-forward)...")
		if err := validateJenkinsViaPortForward(); err != nil {
			fmt.Printf("[WARN] Jenkins HTTP validation failed: %v\n", err)
			fmt.Println("[WARN] Continuing — Jenkins may still be initializing")
		} else {
			fmt.Println("[OK] Jenkins HTTP endpoint validated successfully")
		}

		if err := EnsureJenkinsPortForward(); err != nil {
			lastErr = err
			continue
		}

		if err := updateJenkinsRuntimeState(true); err != nil {
			fmt.Printf("[WARN] Failed to update runtime state: %v\n", err)
		}

		fmt.Println("[OK] Login: admin / admin123")
		fmt.Println("[OK] Jenkins persistent storage: ephemeral (for local dev)")
		
		return nil
	}

	return fmt.Errorf("failed to deploy Jenkins after %d attempts: %w", maxRetries, lastErr)
}

func finishIntegratedBootstrap() error {
	projectPath := strings.TrimSpace(os.Getenv("DEVENV_PROJECT_PATH"))
	clusterName := strings.TrimSpace(os.Getenv("DEVENV_CLUSTER_NAME"))
	if clusterName == "" {
		clusterName = "devenv-local"
	}
	defaultApp := strings.TrimSpace(os.Getenv("DEVENV_APP_NAME"))
	if defaultApp == "" {
		defaultApp = "react-demo"
	}
	return BootstrapIntegrated(projectPath, clusterName, defaultApp)
}

func ensureKubectlWorking() error {
	cmd := exec.Command("kubectl", "cluster-info")
	if err := cmd.Run(); err != nil {
		return errors.New("kubectl cannot connect to cluster")
	}
	return nil
}

// isJenkinsHelmReleasePresent reports whether the Jenkins Helm release exists in the namespace.
func isJenkinsHelmReleasePresent() (bool, error) {
	out, err := exec.Command("helm", "list", "-n", jenkinsNamespace, "--filter", "^"+jenkinsReleaseName+"$", "--short").Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == jenkinsReleaseName, nil
}

// updateJenkinsRuntimeState updates the runtime state with Jenkins configuration (UI always http://127.0.0.1:8080 via port-forward).
func updateJenkinsRuntimeState(healthy bool) error {
	podName, _ := exec.Command("kubectl", "get", "pods", "-n", jenkinsNamespace,
		"-l", "app.kubernetes.io/component=jenkins-controller",
		"-o", "jsonpath={.items[0].metadata.name}").Output()

	return state.UpdateJenkinsState(func(js *state.JenkinsState) {
		js.Enabled = true
		js.UIPort = jenkinsLocalUIPort
		js.NodePort = 0
		js.URL = jenkinsUIURL(jenkinsLocalUIPort, "")
		js.PodName = strings.TrimSpace(string(podName))
		js.Healthy = healthy
		js.LastCheck = time.Now()
	})
}

// Legacy helper for older call sites — NodePort is no longer used for user-facing URLs.
func isJenkinsDeployedWithPort() (deployed bool, nodePort int, err error) {
	deployed, err = isJenkinsHelmReleasePresent()
	if err != nil || !deployed {
		return deployed, 0, err
	}
	portOut, err := exec.Command("kubectl", "get", "svc", "-n", jenkinsNamespace,
		"-l", "app.kubernetes.io/component=jenkins-controller",
		"-o", "jsonpath={.items[0].spec.ports[?(@.name=='http')].nodePort}").Output()
	if err == nil && len(portOut) > 0 {
		s := strings.TrimSpace(string(portOut))
		if s != "" {
			if port, err := strconv.Atoi(s); err == nil {
				return true, port, nil
			}
		}
	}
	return true, 0, nil
}

// Legacy function for compatibility
func isJenkinsDeployed() (bool, error) {
	deployed, _, err := isJenkinsDeployedWithPort()
	return deployed, err
}

// installJenkinsClusterIP installs Jenkins with a ClusterIP service (access via kubectl port-forward only).
func installJenkinsClusterIP() error {
	fmt.Println("[INFO] Installing Jenkins via Helm with ClusterIP service (this may take 2-3 minutes)...")

	cmd := exec.Command("helm", "install", jenkinsReleaseName, "jenkins/jenkins",
		"--namespace", jenkinsNamespace,
		"--set", "controller.serviceType=ClusterIP",
		"--set", "controller.admin.username=admin",
		"--set", "controller.admin.password=admin123",
		"--set", "persistence.enabled=false",
		"--set", "controller.installPlugins[0]=kubernetes:latest",
		"--set", "controller.installPlugins[1]=workflow-aggregator:latest",
		"--set", "controller.installPlugins[2]=git:latest",
		"--set", "controller.installPlugins[3]=configuration-as-code:latest",
		"--set", "controller.installPlugins[4]=kubernetes:latest",
		"--timeout", "10m",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("helm install failed: %w", err)
	}

	return nil
}

// Legacy function for backward compatibility
func installJenkins() error {
	return installJenkinsClusterIP()
}

func uninstallJenkins() error {
	out, err := exec.Command("helm", "list", "-n", jenkinsNamespace, "--filter", "^"+jenkinsReleaseName+"$", "--short").Output()
	if err != nil || strings.TrimSpace(string(out)) != jenkinsReleaseName {
		return nil // Not installed
	}

	cmd := exec.Command("helm", "uninstall", jenkinsReleaseName, "--namespace", jenkinsNamespace)
	return cmd.Run()
}

// verifyJenkinsService validates that the Jenkins Kubernetes service exists and is properly configured
func verifyJenkinsService() error {
	fmt.Println("[INFO] Verifying Jenkins service exists...")
	
	// Check if service exists
	out, err := exec.Command("kubectl", "get", "svc", "-n", jenkinsNamespace,
		"-l", "app.kubernetes.io/component=jenkins-controller",
		"-o", "name").Output()
	
	if err != nil {
		return fmt.Errorf("failed to query Jenkins service: %w", err)
	}
	
	if len(out) == 0 {
		return errors.New("Jenkins service not found")
	}
	
	fmt.Println("[OK] Jenkins service exists")
	
	// Verify service has endpoints
	endpointsOut, err := exec.Command("kubectl", "get", "endpoints", "-n", jenkinsNamespace,
		"-l", "app.kubernetes.io/component=jenkins-controller",
		"-o", "jsonpath={.items[*].subsets[*].addresses}").Output()
	
	if err != nil {
		fmt.Printf("[WARN] Could not verify endpoints: %v\n", err)
		return nil // Don't fail on endpoint check
	}
	
	if len(endpointsOut) == 0 || strings.TrimSpace(string(endpointsOut)) == "" {
		fmt.Println("[WARN] Jenkins service has no ready endpoints yet")
		return nil // Don't fail - service might need more time
	}
	
	fmt.Println("[OK] Jenkins service has ready endpoints")
	return nil
}

// validateJenkinsViaPortForward validates Jenkins HTTP accessibility using kubectl port-forward
// This works reliably on macOS + Kind where NodePort access may not work
func validateJenkinsViaPortForward() error {
	fmt.Println("[INFO] Starting temporary port-forward for validation...")
	
	// Use a dedicated local port for validation so we do not bind Jenkins UI port 8080 during checks
	localPort := validationLocalPort
	
	config := k8sutil.PortForwardConfig{
		Namespace:   jenkinsNamespace,
	ServiceName: jenkinsServiceName,
		LocalPort:   localPort,
		RemotePort:  jenkinsServicePort,
		Timeout:     15 * time.Second,
	}
	
	// Start port-forward with retries
	pf, err := k8sutil.RetryPortForward(config, 2, 2*time.Second)
	if err != nil {
		return fmt.Errorf("failed to establish port-forward: %w", err)
	}
	defer pf.Stop()
	
	// Validate HTTP endpoint
	url := fmt.Sprintf("%s/login", pf.GetLocalURL())
	fmt.Printf("[INFO] Validating Jenkins at %s\n", url)
	
	if err := validateJenkinsUILoginURL(url, 10*time.Second); err != nil {
		return fmt.Errorf("Jenkins HTTP validation failed: %w", err)
	}
	
	fmt.Println("[OK] Jenkins HTTP endpoint accessible via port-forward")
	return nil
}

// validateJenkinsAccessOnPort validates Jenkins HTTP access on specific NodePort
// DEPRECATED: This function only works on Linux with direct NodePort access
// For macOS/Kind, use validateJenkinsViaPortForward() instead
func validateJenkinsAccessOnPort(nodePort int) error {
	fmt.Printf("[WARN] Direct NodePort validation may not work on macOS/Kind (port %d)\n", nodePort)
	return validateJenkinsViaPortForward()
}

// Legacy function for backward compatibility
func validateJenkinsAccess() error {
	return validateJenkinsViaPortForward()
}
// CheckJenkinsStatus performs comprehensive Jenkins health validation
func CheckJenkinsStatus() error {
	fmt.Println("[INFO] Checking Jenkins status")

	deployed, _, err := isJenkinsDeployedWithPort()
	if err != nil {
		return fmt.Errorf("failed to check Helm release: %w", err)
	}

	if !deployed {
		return errors.New("Jenkins Helm release not found - run 'devenv setup' first")
	}

	fmt.Println("[OK] Jenkins Helm release found")

	out, err := exec.Command("kubectl", "get", "pods", "-n", jenkinsNamespace,
		"-l", "app.kubernetes.io/component=jenkins-controller",
		"-o", "custom-columns=NAME:.metadata.name,STATUS:.status.phase,READY:.status.conditions[?(@.type=='Ready')].status,RESTARTS:.status.containerStatuses[0].restartCount,AGE:.metadata.creationTimestamp").Output()
	if err != nil {
		return fmt.Errorf("failed to get Jenkins pods: %w", err)
	}

	podOutput := strings.TrimSpace(string(out))
	if podOutput == "" || strings.Contains(podOutput, "No resources found") {
		return errors.New("no Jenkins pods found")
	}

	fmt.Println(podOutput)

	if !strings.Contains(podOutput, "Running") || !strings.Contains(podOutput, "True") {
		fmt.Println("[WARN] Jenkins pod may not be ready yet")
	} else {
		fmt.Println("[OK] Jenkins pod is running and ready")
	}

	svcOut, err := exec.Command("kubectl", "get", "svc", "-n", jenkinsNamespace,
		"-l", "app.kubernetes.io/component=jenkins-controller",
		"-o", "custom-columns=NAME:.metadata.name,TYPE:.spec.type").Output()
	if err != nil {
		fmt.Println("[WARN] Could not get Jenkins service:", err)
	} else {
		fmt.Println(string(svcOut))
	}

	if err := verifyJenkinsService(); err != nil {
		fmt.Printf("[WARN] Service validation warning: %v\n", err)
	}

	if err := EnsureJenkinsPortForward(); err != nil {
		return err
	}

	loginURL := jenkinsUIURL(jenkinsLocalUIPort, "/login")
	if err := validateJenkinsUILoginURL(loginURL, 15*time.Second); err != nil {
		return fmt.Errorf("Jenkins UI not reachable at %s: %w", loginURL, err)
	}

	if err := updateJenkinsRuntimeState(true); err != nil {
		fmt.Printf("[WARN] Failed to update runtime state: %v\n", err)
	}

	fmt.Println("[OK] Login credentials: admin / admin123")

	logCmd := exec.Command("kubectl", "logs", "-n", jenkinsNamespace,
		"-l", "app.kubernetes.io/component=jenkins-controller",
		"--tail=20")
	logOut, err := logCmd.Output()
	if err == nil && (strings.Contains(string(logOut), "ERROR") || strings.Contains(string(logOut), "SEVERE")) {
		fmt.Println("[WARN] Recent errors detected in Jenkins logs")
	}

	return nil
}
// CleanupJenkins removes Jenkins with graceful shutdown
func CleanupJenkins() error {
	fmt.Println("[INFO] Removing Jenkins from cluster")
	
	deployed, err := isJenkinsDeployed()
	if err != nil {
		return fmt.Errorf("failed to check Jenkins status: %w", err)
	}

	if !deployed {
		fmt.Println("[INFO] Jenkins not deployed, nothing to remove")
		// Still try to clean up namespace
		_ = cleanupNamespace()
		return nil
	}

	// Uninstall Helm release
	fmt.Println("[INFO] Uninstalling Jenkins Helm release...")
	cmd := exec.Command("helm", "uninstall", jenkinsReleaseName, "--namespace", jenkinsNamespace, "--wait")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("[WARN] Helm uninstall had errors:", err)
		// Continue with cleanup
	}

	// Wait for pods to terminate
	fmt.Println("[INFO] Waiting for Jenkins pods to terminate...")
	for i := 0; i < 30; i++ {
		out, err := exec.Command("kubectl", "get", "pods", "-n", jenkinsNamespace, "-l", "app.kubernetes.io/component=jenkins-controller", "--no-headers").Output()
		if err != nil || strings.TrimSpace(string(out)) == "" {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Clean up namespace
	if err := cleanupNamespace(); err != nil {
		fmt.Println("[WARN] Namespace cleanup had errors:", err)
	}

	// Update runtime state
	_ = state.UpdateJenkinsState(func(js *state.JenkinsState) {
		js.Enabled = false
		js.Healthy = false
	})

	fmt.Println("[OK] Jenkins removed")
	return nil
}

func cleanupNamespace() error {
	// Delete namespace
	cmd := exec.Command("kubectl", "delete", "namespace", jenkinsNamespace, "--ignore-not-found", "--timeout=60s")
	return cmd.Run()
}
func ensureHelmInstalled() error {
	if _, err := exec.LookPath("helm"); err != nil {
		return errors.New("helm not installed - install from https://helm.sh/docs/intro/install/")
	}
	
	// Check Helm version
	out, err := exec.Command("helm", "version", "--short").Output()
	if err != nil {
		return fmt.Errorf("helm installed but not working: %w", err)
	}
	
	fmt.Println("[OK] Helm available:", strings.TrimSpace(string(out)))
	return nil
}

func ensureNamespace() error {
	// Check if namespace exists
	out, err := exec.Command("kubectl", "get", "namespace", jenkinsNamespace, "-o", "name").Output()
	if err == nil && strings.Contains(string(out), jenkinsNamespace) {
		fmt.Println("[OK] Namespace", jenkinsNamespace, "exists")
		return nil
	}
	
	// Create namespace
	fmt.Println("[INFO] Creating namespace", jenkinsNamespace)
	cmd := exec.Command("kubectl", "create", "namespace", jenkinsNamespace)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", jenkinsNamespace, err)
	}
	
	fmt.Println("[OK] Namespace", jenkinsNamespace, "created")
	return nil
}

func addJenkinsHelmRepoWithRetry() error {
	// Check if repo already exists
	repoOut, _ := exec.Command("helm", "repo", "list", "-o", "json").Output()
	if strings.Contains(string(repoOut), "https://charts.jenkins.io") {
		fmt.Println("[INFO] Jenkins Helm repo already configured")
		// Update repos
		if err := updateHelmRepos(); err != nil {
			fmt.Println("[WARN] Failed to update Helm repos:", err)
		}
		return nil
	}

	// Add repo with retry
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if attempt > 1 {
			fmt.Printf("[INFO] Retry %d/3: Adding Helm repo...\n", attempt)
			time.Sleep(2 * time.Second)
		}

		cmd := exec.Command("helm", "repo", "add", "jenkins", "https://charts.jenkins.io")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			lastErr = err
			continue
		}

		// Update repos
		if err := updateHelmRepos(); err != nil {
			lastErr = err
			continue
		}

		fmt.Println("[OK] Jenkins Helm repo added")
		return nil
	}

	return fmt.Errorf("failed to add Jenkins Helm repo after 3 attempts: %w", lastErr)
}

func updateHelmRepos() error {
	cmd := exec.Command("helm", "repo", "update")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func waitForJenkinsPod() error {
	fmt.Println("[INFO] Waiting for Jenkins pod to be ready (max 5 minutes)...")
	
	timeout := 5 * time.Minute
	deadline := time.Now().Add(timeout)
	checkInterval := 5 * time.Second

	for time.Now().Before(deadline) {
		// Check pod status
		waitCmd := exec.Command("kubectl", "wait", "--namespace", jenkinsNamespace,
			"--for=condition=ready", "pod",
			"--selector=app.kubernetes.io/component=jenkins-controller",
			"--timeout=5s",
		)
		
		if err := waitCmd.Run(); err == nil {
			// Pod is ready, do additional verification
			time.Sleep(2 * time.Second) // Give it a moment to fully initialize
			
			// Check if containers are actually running
			out, err := exec.Command("kubectl", "get", "pods", "-n", jenkinsNamespace,
				"-l", "app.kubernetes.io/component=jenkins-controller",
				"-o", "jsonpath={.items[*].status.containerStatuses[*].ready}").Output()
			
			if err == nil && strings.Contains(string(out), "true") {
				fmt.Println("[OK] Jenkins pod is ready")
				return nil
			}
		}

		// Show progress
		remaining := time.Until(deadline).Round(time.Second)
		fmt.Printf("[INFO] Still waiting for Jenkins pod... (%v remaining)\n", remaining)
		time.Sleep(checkInterval)
	}

	// Timeout - provide diagnostics
	fmt.Println("[ERROR] Timeout waiting for Jenkins pod")
	fmt.Println("[INFO] Checking pod status...")
	
	diagCmd := exec.Command("kubectl", "get", "pods", "-n", jenkinsNamespace)
	diagCmd.Stdout = os.Stdout
	diagCmd.Stderr = os.Stderr
	diagCmd.Run()

	return errors.New("Jenkins pod did not become ready within 5 minutes - check: kubectl describe pod -n jenkins")
}

// GetJenkinsURL returns the Jenkins access URL (loopback IPv4 + UI port via port-forward).
func GetJenkinsURL() string {
	url, err := state.GetJenkinsURL()
	if err == nil && url != "" {
		return url
	}
	return jenkinsUIURL(jenkinsLocalUIPort, "")
}

// GetJenkinsPort returns the host port for the Jenkins UI (kubectl port-forward).
func GetJenkinsPort() int {
	port, err := state.GetJenkinsPort()
	if err == nil && port > 0 {
		return port
	}
	return jenkinsLocalUIPort
}
