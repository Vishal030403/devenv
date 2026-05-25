package orchestrator

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"devenv/teamalpha/internal/artifact"
	"devenv/teamalpha/internal/cleanup"
	"devenv/teamalpha/internal/cluster"
	appconfig "devenv/teamalpha/internal/config"
	"devenv/teamalpha/internal/debug"
	"devenv/teamalpha/internal/appforward"
	"devenv/teamalpha/internal/buildtag"
	"devenv/teamalpha/internal/deploy"
	"devenv/teamalpha/internal/gammastate"
	"devenv/teamalpha/internal/installer"
	"devenv/teamalpha/internal/jenkinsclient"
	"devenv/teamalpha/internal/jenkinsforward"
	"devenv/teamalpha/internal/kubeport"
	"devenv/teamalpha/internal/portalloc"
	"devenv/teamalpha/internal/lock"
	"devenv/teamalpha/internal/log"
	"devenv/teamalpha/internal/pipeline"
	"devenv/teamalpha/internal/preflight"
	"devenv/teamalpha/internal/report"
	"devenv/teamalpha/internal/rollout"
	runtimeengine "devenv/teamalpha/internal/runtime"
	"devenv/teamalpha/internal/sourcecopy"
	"devenv/teamalpha/internal/validation"
	pkgconfig "devenv/teamalpha/pkg/config"

	gammaint "devenv-gamma/pkg/integration"
	"devenv-gamma/pkg/state"
)

const (
	teamBetaFolder  = "team beta"
	teamDeltaFolder = "team delta"
	teamGammaFolder = "team gamma"
)

// pipelineDeployContext carries immutable image refs and registry coordinates for CD + rollback.
type pipelineDeployContext struct {
	registryPort  int
	registryHost  string
	imageTag      string
	imageRef      string
	containerName string
}

// logLocalNetworkingAfterGammaSetup prints a concise, demo-friendly summary of localhost endpoints
// (Jenkins on :8080 via port-forward, registry host port from Team Gamma state).
func logLocalNetworkingAfterGammaSetup() {
	log.Info("localhost service exposure summary")
	jEp := gammastate.EffectiveJenkinsLocalPort()
	log.Info(fmt.Sprintf("Jenkins mapped to 127.0.0.1:%d", jEp))
	log.Info("Jenkins URL: " + gammastate.EffectiveJenkinsLocalURL())
	if rp, ok := gammastate.RegistryHostPort(); ok && rp > 0 {
		log.Info(fmt.Sprintf("Registry available at localhost:%d", rp))
	}
	log.Success("Local networking initialized")
}

func summarizeJenkinsURLAtPipelineEnd() {
	log.Success("Jenkins available at " + gammastate.EffectiveJenkinsLocalURL())
}

// ─────────────────────────────────────────────────────────────────────────────
// Public commands
// ─────────────────────────────────────────────────────────────────────────────

func Setup() error {
	log.Info("Starting local environment setup")

	r := report.New()
	namespace := appconfig.TargetNamespace()

	// Step 1: Preflight + cluster
	stage := time.Now()
	res, err := preflight.RunPreflightChecks()
	if err != nil {
		r.Record("Preflight", report.StatusFailed, time.Since(stage))
		r.Print()
		return err
	}
	r.Record("Preflight", report.StatusOK, time.Since(stage))

	stage = time.Now()
	if err := cluster.CreateCluster(); err != nil {
		r.Record("Cluster Creation", report.StatusFailed, time.Since(stage))
		r.Print()
		return err
	}
	r.Record("Cluster Creation", report.StatusOK, time.Since(stage))

	stage = time.Now()
	if err := cluster.EnsureNamespace(namespace); err != nil {
		r.Record("Namespace", report.StatusFailed, time.Since(stage))
		r.Print()
		return err
	}
	r.Record("Namespace", report.StatusOK, time.Since(stage))

	// Step 2: Jenkins + Registry
	log.Step(2, 3, "Jenkins and Registry")
	stage = time.Now()
	if err := RunTeamGammaSetup(); err != nil {
		r.Record("Jenkins & Registry", report.StatusFailed, time.Since(stage))
		r.Print()
		return err
	}
	r.Record("Jenkins & Registry", report.StatusOK, time.Since(stage))
	logLocalNetworkingAfterGammaSetup()
	if err := syncIntegratedPlatformConfig(targetProjectPath()); err != nil {
		log.Warn("Platform config sync: " + err.Error())
	}

	// Step 3: Project Detection
	log.Step(3, 3, "Project Detection")
	stage = time.Now()
	if err := RunTeamDeltaSetup(); err != nil {
		r.Record("Project Detection", report.StatusFailed, time.Since(stage))
		r.Print()
		return err
	}
	r.Record("Project Detection", report.StatusOK, time.Since(stage))

	r.Print()
	reportPort := res.RegistryPort
	if p, ok := gammastate.RegistryHostPort(); ok && p > 0 {
		reportPort = p
	}
	projectPath := targetProjectPath()
	jenkinsfilePath := filepath.Join(projectPath, "Jenkinsfile")
	if err := jenkinsclient.EnsureJenkinsJob("http://127.0.0.1:8080", jenkinsfilePath); err != nil {
		log.Warn("Jenkins pipeline provisioning failed: " + err.Error())
	} else {
		log.OK("Jenkins pipeline ready")
	}
	log.Done(fmt.Sprintf("Setup complete (registry port: %d)", reportPort))
	log.Info("Integrated CI/CD: devenv run (local changes) | Jenkins: http://127.0.0.1:8080/job/devenv/job/local-ci-cd/")
	return nil
}

// RunPipeline executes the integrated CI/CD flow via Jenkins (default).
func RunPipeline() error {
	return RunPipelineOptions(PipelineOptions{UseLocal: false})
}

// PipelineOptions controls integrated execution mode.
type PipelineOptions struct {
	UseLocal bool // Run pipeline directly on host instead of via Jenkins
}

// RunPipelineOptions runs CI/CD via Jenkins (default) or directly on the host (--local).
func RunPipelineOptions(opts PipelineOptions) error {
	// ── Execution protection: prevent concurrent runs ─────────────────────
	if err := lock.Acquire(); err != nil {
		return err
	}
	defer lock.Release()

	projectPath := targetProjectPath()
	appName := appNameFromPath(projectPath)
	namespace := appName + "-ns"

	if opts.UseLocal {
		return runLocalPipeline(projectPath, appName, namespace)
	}

	// ── Jenkins CI/CD flow ───────────────────────────────────────────────
	return runJenkinsPipeline(projectPath, appName, namespace)
}

// runJenkinsPipeline validates the environment, uploads source to the cluster,
// triggers the Jenkins job, streams console output, and port-forwards the app.
func runJenkinsPipeline(projectPath, appName, namespace string) error {
	p := pipeline.New()
	defer p.PrintSummary()

	log.Info("Running CI/CD pipeline via Jenkins")

	// fail-fast helper
	failFast := func(name string, fn func() error) error {
		return p.Run(name, fn)
	}

	// ── Tier 1: Environment validation ───────────────────────────────────

	var preflightResult *preflight.Result

	if err := failFast("Preflight Checks", func() error {
		old := os.Stdout
		devNull, _ := os.Open(os.DevNull)
		os.Stdout = devNull
		var err error
		preflightResult, err = preflight.RunPreflightChecks()
		os.Stdout = old
		devNull.Close()
		if err != nil {
			return err
		}
		log.OK("Environment checks passed")
		return nil
	}); err != nil {
		return err
	}

	if err := failFast("Kubernetes Validation", func() error {
		old := os.Stdout
		devNull, _ := os.Open(os.DevNull)
		os.Stdout = devNull
		err1 := preflight.ValidateContext()
		err2 := cluster.ValidateClusterHealth()
		os.Stdout = old
		devNull.Close()
		if err1 != nil {
			return err1
		}
		if err2 != nil {
			return err2
		}
		log.OK("Cluster healthy")
		return nil
	}); err != nil {
		return err
	}

	if err := failFast("Jenkins Validation", func() error {
		if preflightResult != nil {
			rp := registryPortForPipeline(preflightResult)
			_ = validation.ValidateRegistry(rp)
		}
		old := os.Stdout
		devNull, _ := os.Open(os.DevNull)
		os.Stdout = devNull
		err := CheckTeamGammaStatus()
		os.Stdout = old
		devNull.Close()
		if err != nil {
			log.Error("Jenkins inaccessible")
			return err
		}
		ui := gammastate.EffectiveJenkinsLocalPort()
		if err := validation.ValidateJenkins(ui); err != nil {
			log.Error("Jenkins unreachable at 127.0.0.1 — " + err.Error())
			return err
		}
		log.OK("Jenkins reachable at " + gammastate.EffectiveJenkinsLocalURL())
		return nil
	}); err != nil {
		return err
	}

	// Stabilize Jenkins UI port-forward
	if err := jenkinsforward.EnsureStandaloneAfterGamma(); err != nil {
		log.Warn("Jenkins UI kubectl port-forward: " + err.Error())
	}

	// ── Tier 2: Project scaffolding + source upload ──────────────────────

	if err := failFast("Project Detection", func() error {
		old := os.Stdout
		devNull, _ := os.Open(os.DevNull)
		os.Stdout = devNull
		err := RunTeamDeltaSetup()
		os.Stdout = old
		devNull.Close()
		if err != nil {
			return err
		}
		log.OK("Project scaffolding up to date")
		return nil
	}); err != nil {
		return err
	}

	if err := failFast("Jenkinsfile Validation", func() error {
		return validation.ValidateProjectJenkinsfile(projectPath)
	}); err != nil {
		return err
	}

	if err := syncIntegratedPlatformConfig(projectPath); err != nil {
		log.Warn("Platform config sync: " + err.Error())
	}

	if err := failFast("Source Upload", func() error {
		log.Info("Uploading project source to Jenkins workspace...")
		return sourcecopy.CopySourceToCluster(projectPath)
	}); err != nil {
		return err
	}

	// ── Tier 3: Sync Jenkins job + trigger build ─────────────────────────

	jenkinsURL := gammastate.EffectiveJenkinsLocalURL()
	jenkinsfilePath := filepath.Join(projectPath, "Jenkinsfile")

	if err := failFast("Jenkins Job Sync", func() error {
		log.Info("Syncing Jenkins job with project Jenkinsfile...")
		return jenkinsclient.SyncJobFromJenkinsfile(jenkinsURL, jenkinsfilePath, appName)
	}); err != nil {
		return err
	}

	if err := failFast("Jenkins CI/CD Build", func() error {
		// Try git remote URL (optional)
		gitURL, _ := jenkinsclient.GitRemoteOrigin(projectPath)
		// If no git remote, gitURL is empty — Jenkins will use PVC source
		return jenkinsclient.TriggerAndFollowBuild(jenkinsURL, gitURL, appName)
	}); err != nil {
		return err
	}

	// ── Post-build: port-forward and summary ─────────────────────────────

	// Re-affirm Jenkins UI forwarding
	if err := jenkinsforward.EnsureStandaloneAfterGamma(); err != nil {
		log.Warn("Jenkins UI port-forward (post-build): " + err.Error())
	}

	summarizeJenkinsURLAtPipelineEnd()
	startPortForward(appName, namespace)
	log.Done("Pipeline completed via Jenkins CI/CD")
	return nil
}

// runLocalPipeline runs the full CI/CD pipeline directly on the host (legacy/--local mode).
func runLocalPipeline(projectPath, appName, namespace string) error {
	p := pipeline.New()
	defer p.PrintSummary()

	log.Info("Running CI/CD pipeline locally (--local mode)")

	if err := syncIntegratedPlatformConfig(projectPath); err != nil {
		log.Warn("Platform config sync: " + err.Error())
	}

	var preflightResult *preflight.Result
	var imageTag string
	var deployCtx pipelineDeployContext
	deployed := false

	// fail-fast: runs a fatal stage; caller must return on error.
	failFast := func(name string, fn func() error) error {
		return p.Run(name, fn)
	}

	// failFastCD: fatal stage that also triggers rollback+debug if something is deployed.
	failFastCD := func(name string, fn func() error) error {
		err := p.Run(name, fn)
		if err != nil && deployed {
			debug.CollectDiagnostics(namespace, false)
			debug.CollectPodLogs(appName, namespace)
			debug.DescribePods(appName, namespace)
			debug.ShowEvents(namespace)
			p.Run("Rollback Handling", func() error { //nolint
				_ = state.MarkDeploymentFailed(namespace, appName)
				return rollbackToStableImage(appName, namespace, deployCtx)
			})
			p.MarkRolledBack()
		}
		return err
	}

	// ── Tier 1: Environment validation ───────────────────────────────────

	if err := failFast("Preflight Checks", func() error {
		old := os.Stdout
		devNull, _ := os.Open(os.DevNull)
		os.Stdout = devNull
		var err error
		preflightResult, err = preflight.RunPreflightChecks()
		os.Stdout = old
		devNull.Close()
		if err != nil {
			return err
		}
		log.OK("Environment checks passed")
		return nil
	}); err != nil {
		return err
	}

	if err := failFast("Kubernetes Validation", func() error {
		old := os.Stdout
		devNull, _ := os.Open(os.DevNull)
		os.Stdout = devNull
		err1 := preflight.ValidateContext()
		err2 := cluster.ValidateClusterHealth()
		os.Stdout = old
		devNull.Close()
		if err1 != nil {
			return err1
		}
		if err2 != nil {
			return err2
		}
		log.OK("Cluster healthy")
		return nil
	}); err != nil {
		return err
	}

	if err := failFast("Jenkins Validation", func() error {
		if preflightResult != nil {
			rp := registryPortForPipeline(preflightResult)
			_ = validation.ValidateRegistry(rp)
			log.Info(fmt.Sprintf("Registry has no HTML UI — use REST API http://127.0.0.1:%d/v2/ (empty body or HTTP 401 is normal)", rp))
		}
		old := os.Stdout
		devNull, _ := os.Open(os.DevNull)
		os.Stdout = devNull
		err := CheckTeamGammaStatus()
		os.Stdout = old
		devNull.Close()
		if err != nil {
			log.Error("Jenkins inaccessible")
			return err
		}
		ui := gammastate.EffectiveJenkinsLocalPort()
		if err := validation.ValidateJenkins(ui); err != nil {
			log.Error("Jenkins unreachable at 127.0.0.1 — " + err.Error())
			return err
		}
		log.OK("Jenkins reachable at " + gammastate.EffectiveJenkinsLocalURL())
		return nil
	}); err != nil {
		return err
	}

	if err := jenkinsforward.EnsureStandaloneAfterGamma(); err != nil {
		log.Warn("Jenkins UI kubectl port-forward: " + err.Error())
		log.Info("Open Jenkins at " + gammastate.EffectiveJenkinsLocalURL())
	}

	if err := failFast("Project Detection", func() error {
		old := os.Stdout
		devNull, _ := os.Open(os.DevNull)
		os.Stdout = devNull
		err := RunTeamDeltaSetup()
		os.Stdout = old
		devNull.Close()
		if err != nil {
			return err
		}
		log.OK("Project scaffolding up to date")
		return nil
	}); err != nil {
		return err
	}

	if err := failFast("Jenkinsfile Validation", func() error {
		return validation.ValidateProjectJenkinsfile(projectPath)
	}); err != nil {
		return err
	}

	// Config loading is optional — missing devenv.yaml is non-fatal.
	if _, err := os.Stat(filepath.Join(projectPath, "devenv.yaml")); err == nil {
		p.RunOptional("Config Loading", func() error {
			return loadProjectConfig(projectPath)
		})
	} else {
		p.Skip("Config Loading", "devenv.yaml not found")
	}

	// ── Tier 2: CI validation ─────────────────────────────────────────────

	if err := failFast("Linting", func() error {
		return lintProjectStructure(projectPath)
	}); err != nil {
		return err
	}

	p.RunOptional("Unit Testing", func() error {
		return runUnitTests(projectPath)
	})

	p.RunOptional("Security Scanning", func() error {
		old := os.Stdout
		devNull, _ := os.Open(os.DevNull)
		os.Stdout = devNull
		err := runSecurityScan(projectPath)
		os.Stdout = old
		devNull.Close()
		return err
	})

	if err := failFast("Docker Build", func() error {
		log.Info("Building Docker image...")
		cmd := exec.Command("docker", "build", "-t", appName+":latest", ".")
		cmd.Dir = projectPath
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Print(string(out))
			return fmt.Errorf("docker build failed: %w", err)
		}
		log.OK("Image built: " + appName + ":latest")
		return nil
	}); err != nil {
		return err
	}

	if err := failFast("Image Tagging", func() error {
		var err error
		imageTag, err = tagDockerImage(appName, projectPath)
		return err
	}); err != nil {
		return err
	}

	if err := failFast("Manifest Validation", func() error {
		return validateK8sManifests(projectPath)
	}); err != nil {
		return err
	}

	if err := failFast("Registry Push", func() error {
		rp := registryPortForPipeline(preflightResult)
		if err := pushToRegistryWithRetry(appName, imageTag, rp); err != nil {
			return err
		}
		artifact.SyncRegistryCatalog(rp, appName, namespace, appName)
		return nil
	}); err != nil {
		return err
	}

	// ── Tier 3: CD deployment ─────────────────────────────────────────────

	deployCtx.registryPort = registryPortForPipeline(preflightResult)
	deployCtx.registryHost = registryHostForK8sPull(deployCtx.registryPort)
	deployCtx.imageTag = imageTag
	deployCtx.imageRef = deploy.ImageRef(deployCtx.registryHost, appName, imageTag)
	log.Info(fmt.Sprintf("Deploying immutable artifact %s (not :latest-only)", deployCtx.imageRef))

	deployed = true

	if err := failFastCD("Kubernetes Deployment", func() error {
		return deployToCluster(projectPath, deployCtx, appName, namespace)
	}); err != nil {
		return err
	}

	if err := failFastCD("Rollout Monitoring", func() error {
		deployments, err := getProjectDeployments(appName, namespace)
		if err != nil || len(deployments) == 0 {
			return rollout.MonitorRollout(appName, namespace)
		}
		for _, d := range deployments {
			if err := rollout.MonitorRollout(d, namespace); err != nil {
				return err
			}
		}
		return debug.DetectPodFailures(appName, namespace)
	}); err != nil {
		return err
	}

	if err := failFastCD("Health Verification", func() error {
		if failErr := debug.DetectPodFailures(appName, namespace); failErr != nil {
			return failErr
		}
		return healthCheck(appName, namespace)
	}); err != nil {
		return err
	}

	p.RunOptional("Smoke Testing", func() error {
		svcPort := kubeport.KubernetesServicePort(namespace, appName)
		log.Info("Allocating dynamic application port...")
		localPort, err := portalloc.AllocateApplicationLocalPort()
		if err != nil {
			return err
		}
		if localPort == portalloc.JenkinsReservedLocalPort {
			return fmt.Errorf("cannot run smoke forward on Jenkins localhost port %d", portalloc.JenkinsReservedLocalPort)
		}
		mapping := fmt.Sprintf("%d:%d", localPort, svcPort)
		pfCmd := exec.Command("kubectl", "port-forward",
			"--address", "127.0.0.1",
			"service/"+appName, mapping, "-n", namespace)
		if err := pfCmd.Start(); err != nil {
			log.Warn("Smoke test skipped: could not start port-forward (" + err.Error() + ")")
			return nil
		}
		defer func() { _ = pfCmd.Process.Kill() }()
		time.Sleep(3 * time.Second)
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d", localPort))
		if err != nil {
			log.Warn("Smoke test: could not reach app via port-forward (" + err.Error() + ")")
			return nil
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 500 {
			return fmt.Errorf("smoke test returned HTTP %d", resp.StatusCode)
		}
		log.OK(fmt.Sprintf("Smoke test passed: HTTP %d (http://127.0.0.1:%d)", resp.StatusCode, localPort))
		return nil
	})

	_ = validation.ValidateNetworkPolicies(namespace)

	if !p.RolledBack() {
		p.Skip("Rollback Handling", "no deployment failure")
	}

	if err := jenkinsforward.EnsureStandaloneAfterGamma(); err != nil {
		log.Warn("Jenkins UI port-forward (after CI stages): " + err.Error())
	}

	if err := state.MarkDeploymentSuccess(namespace, appName, deployCtx.imageTag); err != nil {
		log.Warn("Could not record successful deployment: " + err.Error())
	}
	artifact.SyncRegistryCatalog(deployCtx.registryPort, appName, namespace, appName)
	artifact.LogDeploymentSummary(namespace, appName)

	summarizeJenkinsURLAtPipelineEnd()
	startPortForward(appName, namespace)
	log.Done("Pipeline completed successfully (local mode)")
	return nil
}

func Status() error {
	log.Info("Checking local development environment status")

	if err := preflight.ValidateContext(); err != nil {
		log.Warn("Kubernetes context not set correctly")
	}

	if err := cluster.ValidateClusterHealth(); err != nil {
		log.Error("Cluster health check failed: " + err.Error())
	} else {
		log.OK("Cluster healthy")
	}

	if err := CheckTeamGammaStatus(); err != nil {
		log.Warn("Jenkins and registry services not available")
	}

	if err := CheckTeamDeltaStatus(); err != nil {
		log.Warn("Project detection not run")
	}

	log.Done("Status check completed")
	return nil
}

func Down() error {
	log.Info("Cleaning stale application port-forward processes...")
	_ = appforward.StopPrevious()
	_ = jenkinsforward.StopPrevious()
	log.Info("Cleaning local development environment")
	namespace := appconfig.TargetNamespace()

	log.Step(1, 3, "Remove Jenkins and Registry")
	if err := CleanupTeamGammaServices(); err != nil {
		log.Warn("Jenkins/registry cleanup error: " + err.Error())
	}

	log.Step(2, 3, "Delete cluster and verify")
	if err := cluster.DeleteCluster(); err != nil {
		return err
	}
	_ = cleanup.VerifyNamespaceCleanup(namespace)
	_ = cleanup.VerifyIngressCleanup()
	_ = cleanup.VerifyServiceCleanup()
	_ = cleanup.CleanupLeftovers()
	_ = cleanup.VerifyCleanup(pkgconfig.ClusterName)

	log.Step(3, 3, "Clean temporary state")
	_ = CleanupTeamDeltaFiles()
	cleanupTemporaryState()

	log.Done("Environment cleanup completed")
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Team delegation
// ─────────────────────────────────────────────────────────────────────────────

func RunTeamDeltaSetup() error     { return runTeamDeltaCommand("up") }
func CheckTeamDeltaStatus() error  { return runTeamDeltaCommand("status") }
func CleanupTeamDeltaFiles() error { return runTeamDeltaCommand("down") }

func runTeamDeltaCommand(command string) error {
	deltaDir, err := findSiblingTeamDir(teamDeltaFolder)
	if err != nil {
		return err
	}
	return runTeamCommandEnv(deltaDir, command, nil, "--path", targetProjectPath())
}

func RunTeamGammaSetup() error { return runTeamGammaCommand("up") }
func CheckTeamGammaStatus() error     { return runTeamGammaCommand("status") }
func CleanupTeamGammaServices() error { return runTeamGammaCommand("down") }

func runTeamGammaCommand(command string) error {
	gammaDir, err := findSiblingTeamDir(teamGammaFolder)
	if err != nil {
		return err
	}
	projectPath := targetProjectPath()
	env := []string{
		"DEVENV_QUIET_SUBPROCESS=1",
		fmt.Sprintf("DEVENV_PROJECT_PATH=%s", projectPath),
		fmt.Sprintf("DEVENV_CLUSTER_NAME=%s", pkgconfig.ClusterName),
		fmt.Sprintf("DEVENV_APP_NAME=%s", appNameFromPath(projectPath)),
	}
	return runTeamCommandEnv(gammaDir, command, env)
}

// ─────────────────────────────────────────────────────────────────────────────
// CI stage implementations
// ─────────────────────────────────────────────────────────────────────────────

func loadProjectConfig(projectPath string) error {
	configPath := filepath.Join(projectPath, "devenv.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("could not read devenv.yaml: %w", err)
	}
	if len(data) == 0 {
		return fmt.Errorf("devenv.yaml is empty")
	}
	log.OK(fmt.Sprintf("Config loaded from %s", configPath))
	return nil
}

func lintProjectStructure(projectPath string) error {
	required := []string{
		filepath.Join(projectPath, "Dockerfile"),
		filepath.Join(projectPath, "k8s", "overlays", "local"),
	}
	for _, path := range required {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("missing required path: %s", path)
		}
	}
	return nil
}

func runUnitTests(projectPath string) error {

	framework := runtimeengine.DetectFramework(projectPath)

	log.Info("Detected framework for testing: " + framework)

	return runtimeengine.RunTests(
		framework,
		projectPath,
	)
}

func runSecurityScan(projectPath string) error {
	dockerfilePath := filepath.Join(projectPath, "Dockerfile")

	trivyAvailable := true
	hadolintAvailable := true

	if _, err := exec.LookPath("trivy"); err != nil {
		if err := installer.InstallTrivy(); err != nil {
			trivyAvailable = false
		}
	}

	if _, err := exec.LookPath("hadolint"); err != nil {
		if err := installer.InstallHadolint(); err != nil {
			hadolintAvailable = false
		}
	}

	if !trivyAvailable && !hadolintAvailable {
		if _, err := os.Stat(dockerfilePath); err != nil {
			return fmt.Errorf("Dockerfile not found at %s", dockerfilePath)
		}
		log.OK("Dockerfile present (install trivy or hadolint for full scan)")
		return nil
	}

	if hadolintAvailable {
		cmd := exec.Command("hadolint", dockerfilePath)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("hadolint: %s", strings.TrimSpace(string(out)))
		}
		log.OK("Dockerfile passed hadolint scan")
	}

	if trivyAvailable {
		log.OK("Trivy available — image scan ready")
	}

	return nil
}

func runDockerBuild(projectPath, appName string) error {
	cmd := exec.Command("docker", "build", "-t", appName+":latest", ".")
	cmd.Dir = projectPath
	stream := newPacedLineWriter(80 * time.Millisecond)
	cmd.Stdout = stream
	cmd.Stderr = stream
	err := cmd.Run()
	stream.Flush()
	if err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}
	return nil
}

func tagDockerImage(appName, projectPath string) (string, error) {
	// Unique per run — does not require git commit (uncommitted UI edits still get a new tag).
	tag := buildtag.UniqueImageTag(projectPath)

	versioned := appName + ":" + tag
	cmd := exec.Command("docker", "tag", appName+":latest", versioned)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("docker tag failed: %s", strings.TrimSpace(string(out)))
	}
	log.OK(fmt.Sprintf("Tagged immutable artifact %s (also :latest)", versioned))
	log.Info("Each run uses a new tag so Kubernetes pulls your latest build without git commit")
	return tag, nil
}

func validateK8sManifests(projectPath string) error {
	overlayPath := filepath.Join(projectPath, "k8s", "overlays", "local")
	if _, err := os.Stat(overlayPath); os.IsNotExist(err) {
		log.Warn("No k8s/overlays/local found — skipping manifest validation")
		return nil
	}
	cmd := exec.Command("kubectl", "apply", "--dry-run=client", "-k", overlayPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("manifest validation failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Registry push (primary deployment path)
// ─────────────────────────────────────────────────────────────────────────────

// registryHostForK8sPull is the registry hostname used in Kubernetes image
// references. kind nodes must reach the host registry; "localhost" inside the
// node is not the Mac/Windows host, so Docker Desktop uses host.docker.internal.
// Override with DEVENV_REGISTRY_PULL_HOST (host:port) or DEVENV_REGISTRY.
func registryHostForK8sPull(port int) string {
	if h := strings.TrimSpace(os.Getenv("DEVENV_REGISTRY_PULL_HOST")); h != "" {
		return h
	}
	if h := strings.TrimSpace(os.Getenv("DEVENV_REGISTRY")); h != "" {
		return h
	}
	switch runtime.GOOS {
	case "darwin", "windows":
		return fmt.Sprintf("host.docker.internal:%d", port)
	default:
		return fmt.Sprintf("localhost:%d", port)
	}
}

// registryPortForPipeline prefers Team Gamma's persisted registry host_port
// so push/deploy match an existing devenv-registry even when preflight picked
// a different "next free" port.
func registryPortForPipeline(res *preflight.Result) int {
	if p, ok := gammastate.RegistryHostPort(); ok && p > 0 {
		return p
	}
	if res != nil && res.RegistryPort > 0 {
		return res.RegistryPort
	}
	return pkgconfig.RegistryStartPort
}

func pushToRegistryWithRetry(appName, tag string, registryPort int) error {
	host := fmt.Sprintf("localhost:%d", registryPort)

	log.Info(fmt.Sprintf("Pushing images to registry at %s", host))

	// Validate registry is accessible before attempting push
	if err := validateRegistryAccessible(registryPort); err != nil {
		return fmt.Errorf("registry not accessible: %w", err)
	}

	tags := []string{"latest", tag}
	maxRetries := 3
	retryDelay := 2 * time.Second

	for _, srcTag := range tags {
		src := appName + ":" + srcTag
		dst := host + "/" + appName + ":" + srcTag

		log.Info(fmt.Sprintf("Tagging %s → %s", src, dst))
		if out, err := exec.Command("docker", "tag", src, dst).CombinedOutput(); err != nil {
			return fmt.Errorf("docker tag failed: %s", strings.TrimSpace(string(out)))
		}

		// Push with retry
		var lastErr error
		for attempt := 1; attempt <= maxRetries; attempt++ {
			if attempt > 1 {
				log.Info(fmt.Sprintf("Retry %d/%d: Pushing %s", attempt, maxRetries, dst))
				time.Sleep(retryDelay)
			}

			cmd := exec.Command("docker", "push", dst)
			// Use a filtered writer that suppresses noisy "Waiting" layer lines
			stream := newFilteredLineWriter(80*time.Millisecond, func(line string) bool {
				trimmed := strings.TrimSpace(line)
				// Suppress lines that are only layer hash + "Waiting/Preparing"
				if strings.HasSuffix(trimmed, ": Waiting") || strings.HasSuffix(trimmed, ": Preparing") {
					return false
				}
				return true
			})
			cmd.Stdout = stream
			cmd.Stderr = stream

			if err := cmd.Run(); err != nil {
				stream.Flush()
				lastErr = fmt.Errorf("push attempt %d failed: %w", attempt, err)
				log.Warn(lastErr.Error())
				continue
			}

			stream.Flush()
			if srcTag != "latest" {
				log.OK(fmt.Sprintf("Pushed immutable artifact %s", dst))
			} else {
				log.OK(fmt.Sprintf("Pushed %s", dst))
			}
			break // Success
		}

		if lastErr != nil {
			return fmt.Errorf("failed to push %s after %d attempts: %w", dst, maxRetries, lastErr)
		}
	}

	log.OK(fmt.Sprintf("All images pushed — deploy will use immutable tag :%s", tag))
	return nil
}

func validateRegistryAccessible(port int) error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/v2/", port))
	if err != nil {
		return fmt.Errorf("registry HTTP check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registry returned HTTP %d", resp.StatusCode)
	}

	return nil
}

func pushToRegistry(appName, tag string) error {
	return pushToRegistryWithRetry(appName, tag, registryPortForPipeline(nil))
}

// ─────────────────────────────────────────────────────────────────────────────
// CD stage implementations
// ─────────────────────────────────────────────────────────────────────────────

func loadImageToCluster(appName string) error {
	cmd := exec.Command("kind", "load", "docker-image", appName+":latest", "--name", pkgconfig.ClusterName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kind load failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func deployToCluster(projectPath string, dctx pipelineDeployContext, appName, namespace string) error {
	log.Info("Deploying to Kubernetes cluster")

	if err := cluster.EnsureKindRegistryHTTPPull(dctx.registryPort); err != nil {
		log.Warn(fmt.Sprintf("kind registry pull config (optional): %v", err))
	}

	container := deploy.ContainerName(namespace, appName)
	priorImage, _ := deploy.RunningImage(namespace, appName, container)
	if err := state.RecordDeploymentStart(namespace, appName, appName, container, dctx.registryHost, dctx.imageTag, dctx.imageRef, priorImage); err != nil {
		log.Warn("Could not record deployment metadata: " + err.Error())
	}
	artifact.LogDeploymentSummary(namespace, appName)

	if err := updateManifestsForRegistry(projectPath, dctx.registryPort); err != nil {
		log.Warn(fmt.Sprintf("Could not update manifests for registry: %v", err))
	}

	if err := deploy.UpdateKustomizeImageTag(projectPath, appName, dctx.imageTag); err != nil {
		return fmt.Errorf("update kustomize image tag: %w", err)
	}
	log.OK(fmt.Sprintf("Manifest image tag set to %s (immutable)", dctx.imageTag))

	overlayPath := filepath.Join(projectPath, "k8s", "overlays", "local")
	cmd := exec.Command("kubectl", "apply", "-k", overlayPath)
	stream := newPacedLineWriter(150 * time.Millisecond)
	cmd.Stdout = stream
	cmd.Stderr = stream
	err := cmd.Run()
	stream.Flush()
	if err != nil {
		return fmt.Errorf("kubectl apply failed: %w", err)
	}

	log.OK("Kubernetes resources applied")

	if err := deploy.SetImageOnly(namespace, appName, container, dctx.imageRef); err != nil {
		return err
	}
	deploy.AnnotateDeployment(namespace, appName, dctx.imageTag)
	log.OK(fmt.Sprintf("Deployment image set to %s", dctx.imageRef))
	return nil
}

func updateManifestsForRegistry(projectPath string, registryPort int) error {
	// Update the kustomization.yaml to use registry URL for images
	kustomizationPath := filepath.Join(projectPath, "k8s", "overlays", "local", "kustomization.yml")

	data, err := os.ReadFile(kustomizationPath)
	if err != nil {
		return fmt.Errorf("failed to read kustomization: %w", err)
	}

	content := string(data)
	registryURL := registryHostForK8sPull(registryPort)
	wantPrefix := registryURL + "/"

	// Check if already has the intended registry prefix
	if strings.Contains(content, wantPrefix) {
		return nil // Already configured
	}

	// Update image reference to include registry (replace any prior newName value)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(trim, "newName:") {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(trim, "newName:"))
		appName := path.Base(val)
		if appName == "" || appName == "." {
			continue
		}
		lines[i] = fmt.Sprintf("    newName: %s/%s", registryURL, appName)
	}

	updated := strings.Join(lines, "\n")
	if err := os.WriteFile(kustomizationPath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("failed to write updated kustomization: %w", err)
	}

	log.OK("Updated manifests to pull from registry")
	return nil
}

// getProjectDeployments lists deployment names in the target namespace for the app.
func getProjectDeployments(appName, namespace string) ([]string, error) {
	out, err := exec.Command(
		"kubectl", "get", "deployments",
		"-n", namespace,
		"-o", "custom-columns=NAME:.metadata.name",
		"--no-headers",
	).Output()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if n := strings.TrimSpace(line); n != "" && n != appName {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return []string{appName}, nil
	}
	return names, nil
}

func healthCheck(appName, namespace string) error {
	time.Sleep(3 * time.Second)
	cmd := exec.Command("kubectl", "get", "pods",
		"-n", namespace,
		"-l", "app="+appName,
		"--field-selector=status.phase=Running",
		"--no-headers",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not query pods: %w", err)
	}
	running := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) != "" {
			running++
		}
	}
	if running == 0 {
		return fmt.Errorf("no running pods for %s in namespace %s", appName, namespace)
	}
	return nil
}

// rollbackToStableImage redeploys a prior registry image without rebuilding (production-style rollback).
func rollbackToStableImage(appName, namespace string, dctx pipelineDeployContext) error {
	log.Info("Rolling back to previous stable image from local registry (no Docker/npm rebuild)")

	imageRef, tag, ok, err := state.RollbackImageRef(namespace, appName)
	if err != nil {
		return err
	}
	if !ok || strings.TrimSpace(imageRef) == "" {
		log.Warn("No registry rollback target in deployment history — falling back to kubectl rollout undo")
		return rollbackDeploymentUndo(appName, namespace)
	}

	container := dctx.containerName
	if container == "" {
		container = deploy.ContainerName(namespace, appName)
	}
	log.Info("Rollback target: " + imageRef)
	if err := deploy.SetImage(namespace, appName, container, imageRef); err != nil {
		return fmt.Errorf("registry rollback failed: %w", err)
	}
	if tag == "" {
		tag = deploy.TagFromImageRef(imageRef)
	}
	if err := state.MarkDeploymentRolledBack(namespace, appName, imageRef, tag); err != nil {
		log.Warn("Could not update rollback state: " + err.Error())
	}
	artifact.SyncRegistryCatalog(dctx.registryPort, appName, namespace, appName)
	artifact.LogDeploymentSummary(namespace, appName)
	log.Success("Rollback completed — previous stable image running from registry")
	return nil
}

func rollbackDeploymentUndo(appName, namespace string) error {
	cmd := exec.Command("kubectl", "rollout", "undo", "deployment/"+appName, "-n", namespace)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl rollout undo failed: %s", strings.TrimSpace(string(out)))
	}
	log.OK(fmt.Sprintf("Rolled back deployment %s via ReplicaSet history", appName))
	return nil
}

func startPortForward(appName, namespace string) {
	_ = appforward.StopPrevious()
	svcPort := kubeport.KubernetesServicePort(namespace, appName)
	log.Info("Allocating dynamic application port...")
	localPort, err := portalloc.AllocateApplicationLocalPort()
	if err != nil {
		log.Warn(fmt.Sprintf("Could not allocate a localhost port for the workload (Jenkins owns :%d): %v", pkgconfig.JenkinsUIPort, err))
		return
	}
	if localPort == portalloc.JenkinsReservedLocalPort {
		log.Error("Workload port-forward refused: would bind Jenkins-reserved localhost port")
		return
	}

	mapping := fmt.Sprintf("%d:%d", localPort, svcPort)
	cmd := exec.Command("kubectl", "port-forward",
		"--address", "127.0.0.1",
		"service/"+appName, mapping, "-n", namespace)
	if err := cmd.Start(); err != nil {
		log.Warn("Could not start port-forward: " + err.Error())
		return
	}
	if err := appforward.Save(appName, namespace, localPort, cmd.Process.Pid); err != nil {
		log.Warn("Could not persist port-forward metadata: " + err.Error())
	}
	time.Sleep(2 * time.Second)
	log.Success(fmt.Sprintf("App available at http://127.0.0.1:%d", localPort))
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func cleanupTemporaryState() {
	appName := appNameFromPath(targetProjectPath())
	log.Info(fmt.Sprintf("Removing Docker image %s:latest", appName))
	if err := exec.Command("docker", "rmi", appName+":latest", "--force").Run(); err != nil {
		log.Warn(fmt.Sprintf("Could not remove image %s:latest (may not exist)", appName))
		return
	}
	log.OK(fmt.Sprintf("Removed image %s:latest", appName))
}

func syncIntegratedPlatformConfig(projectPath string) error {
	cfg, err := gammaint.LoadPlatformConfigFromState(projectPath, pkgconfig.ClusterName, appNameFromPath(projectPath))
	if err != nil {
		return err
	}
	return gammaint.SyncPlatformConfigToCluster(cfg)
}

// runViaJenkins is kept for backward compatibility — now called from runJenkinsPipeline.
func runViaJenkins(projectPath, appName string) error {
	return fmt.Errorf("use RunPipelineOptions with UseLocal=false for Jenkins CI/CD")
}

func targetProjectPath() string {
	if path := os.Getenv("DEVENV_PROJECT_PATH"); path != "" {
		return path
	}
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Join("..", "sample apps", "react-demo")
	}
	for _, candidate := range []string{
		filepath.Join(wd, "..", "sample apps", "react-demo"),
		filepath.Join(wd, "sample apps", "react-demo"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Join(wd, "..", "sample apps", "react-demo")
}

func appNameFromPath(projectPath string) string {
	base := strings.ToLower(filepath.Base(projectPath))
	return strings.NewReplacer(" ", "-", "_", "-").Replace(base)
}

func findSiblingTeamDir(teamFolder string) (string, error) {
	wd, _ := os.Getwd()
	executable, _ := os.Executable()
	executableDir := filepath.Dir(executable)

	candidates := []string{
		filepath.Join(wd, "..", teamFolder),
		filepath.Join(wd, teamFolder),
		filepath.Join(executableDir, "..", teamFolder),
		filepath.Join(executableDir, teamFolder),
	}

	for _, candidate := range candidates {
		abs, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(abs, "go.mod")); err == nil {
			return abs, nil
		}
	}
	return "", fmt.Errorf("required component %q not found (expected sibling folder)", teamFolder)
}

func runTeamCommand(teamDir, command string, args ...string) error {
	return runTeamCommandEnv(teamDir, command, nil, args...)
}

func runTeamCommandEnv(teamDir, command string, extraEnv []string, args ...string) error {
	commandArgs := append([]string{"run", ".", command}, args...)
	cmd := exec.Command("go", commandArgs...)
	cmd.Dir = teamDir
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}

	stream := newPacedLineWriter(220 * time.Millisecond)
	cmd.Stdout = stream
	cmd.Stderr = stream

	err := cmd.Run()
	stream.Flush()

	if err != nil {
		return fmt.Errorf("%q step failed: %w", command, err)
	}

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// pacedLineWriter — throttles subprocess output for readable progress display
// ─────────────────────────────────────────────────────────────────────────────

type pacedLineWriter struct {
	buffer bytes.Buffer
	output bytes.Buffer
	delay  time.Duration
}

func newPacedLineWriter(delay time.Duration) *pacedLineWriter {
	return &pacedLineWriter{delay: delay}
}

func (w *pacedLineWriter) Write(p []byte) (int, error) {
	w.output.Write(p)
	for _, b := range p {
		if b == '\n' {
			w.printBufferedLine()
			continue
		}
		w.buffer.WriteByte(b)
	}
	return len(p), nil
}

func (w *pacedLineWriter) Flush() {
	if w.buffer.Len() > 0 {
		w.printBufferedLine()
	}
}

func (w *pacedLineWriter) String() string { return w.output.String() }

func (w *pacedLineWriter) printBufferedLine() {
	line := strings.TrimRight(w.buffer.String(), "\r")
	w.buffer.Reset()
	io.WriteString(os.Stdout, line+"\n")
	if strings.TrimSpace(line) != "" {
		time.Sleep(w.delay)
	}
}

// filteredLineWriter wraps pacedLineWriter and drops lines rejected by filter.
type filteredLineWriter struct {
	buffer bytes.Buffer
	delay  time.Duration
	filter func(string) bool
}

func newFilteredLineWriter(delay time.Duration, filter func(string) bool) *filteredLineWriter {
	return &filteredLineWriter{delay: delay, filter: filter}
}

func (w *filteredLineWriter) Write(p []byte) (int, error) {
	for _, b := range p {
		if b == '\n' {
			line := strings.TrimRight(w.buffer.String(), "\r")
			w.buffer.Reset()
			if w.filter(line) {
				io.WriteString(os.Stdout, line+"\n")
				if strings.TrimSpace(line) != "" {
					time.Sleep(w.delay)
				}
			}
			continue
		}
		w.buffer.WriteByte(b)
	}
	return len(p), nil
}

func (w *filteredLineWriter) Flush() {
	if w.buffer.Len() > 0 {
		line := strings.TrimRight(w.buffer.String(), "\r")
		w.buffer.Reset()
		if w.filter(line) {
			io.WriteString(os.Stdout, line+"\n")
		}
	}
}
