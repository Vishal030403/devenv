package jenkins

import (
	"fmt"
	"os"
	"strings"
	"time"

	"devenv-gamma/internal/registry"
	"devenv-gamma/pkg/integration"
	"devenv-gamma/pkg/k8sutil"
	"devenv-gamma/pkg/state"
)

// StartIntegrated ensures the local registry, in-cluster Jenkins (Helm), UI on 127.0.0.1:8080,
// platform ConfigMap, and devenv/local-ci-cd job. Idempotent — safe to run repeatedly.
func StartIntegrated() error {
	fmt.Println("[INFO] Jenkins lifecycle: start (registry + in-cluster Jenkins + localhost:8080)")

	if err := registry.StartRegistry(); err != nil {
		return fmt.Errorf("registry required for Jenkins pipelines: %w", err)
	}

	if err := DeployJenkins(); err != nil {
		return err
	}

	projectPath := strings.TrimSpace(os.Getenv("DEVENV_PROJECT_PATH"))
	clusterName := strings.TrimSpace(os.Getenv("DEVENV_CLUSTER_NAME"))
	if clusterName == "" {
		clusterName = "dev-cluster"
	}
	appName := strings.TrimSpace(os.Getenv("DEVENV_APP_NAME"))
	if appName == "" {
		appName = "react-demo"
	}

	cfg, err := integration.LoadPlatformConfigFromState(projectPath, clusterName, appName)
	if err != nil {
		return fmt.Errorf("load platform config: %w", err)
	}
	if err := integration.SyncPlatformConfigToCluster(cfg); err != nil {
		return fmt.Errorf("sync platform config for Jenkins agents: %w", err)
	}

	fmt.Printf("[SUCCESS] Jenkins integrated stack ready at %s\n", cfg.JenkinsURL)
	return nil
}

// StopExposure stops localhost:8080 port-forwards only. The Helm release stays in the cluster.
func StopExposure() error {
	fmt.Println("[INFO] Jenkins lifecycle: stop UI exposure (cluster deployment unchanged)")

	mgr := k8sutil.GetPortForwardManager()
	if pf := mgr.Get(jenkinsNamespace, jenkinsServiceName, jenkinsLocalUIPort); pf != nil {
		if err := pf.Stop(); err != nil {
			fmt.Printf("[WARN] Stop Gamma port-forward: %v\n", err)
		}
	}
	_ = state.UnregisterPortForward(jenkinsNamespace, jenkinsServiceName, jenkinsLocalUIPort)

	_ = state.UpdateJenkinsState(func(js *state.JenkinsState) {
		if js.Enabled {
			js.Healthy = false
		}
		js.LastCheck = time.Now()
	})

	fmt.Println("[OK] Jenkins UI port-forward stopped — run 'devenv pipeline jenkins start' to restore http://127.0.0.1:8080")
	return nil
}

// StopIntegrated removes Jenkins from the cluster (Helm uninstall). Registry is left running.
func StopIntegrated() error {
	fmt.Println("[INFO] Jenkins lifecycle: full stop (remove Helm release)")
	if err := StopExposure(); err != nil {
		fmt.Printf("[WARN] %v\n", err)
	}
	return CleanupJenkins()
}
