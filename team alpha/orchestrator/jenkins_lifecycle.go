package orchestrator

import (
	"fmt"
	"os"

	"devenv/teamalpha/internal/cluster"
	"devenv/teamalpha/internal/gammastate"
	"devenv/teamalpha/internal/jenkinsforward"
	"devenv/teamalpha/internal/log"
	"devenv/teamalpha/internal/validation"
	pkgconfig "devenv/teamalpha/pkg/config"
)

func setJenkinsLifecycleEnv() {
	projectPath := targetProjectPath()
	_ = os.Setenv("DEVENV_PROJECT_PATH", projectPath)
	_ = os.Setenv("DEVENV_CLUSTER_NAME", pkgconfig.ClusterName)
	_ = os.Setenv("DEVENV_APP_NAME", appNameFromPath(projectPath))
}

func runTeamGammaJenkins(subcommand string, extraArgs ...string) error {
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
	return runTeamCommandEnv(gammaDir, "jenkins", env, append([]string{subcommand}, extraArgs...)...)
}

// JenkinsStart brings up registry + in-cluster Jenkins, syncs platform config, and stabilizes :8080.
func JenkinsStart() error {
	log.Info("Starting Jenkins (integrated with devenv registry and Kind cluster)")
	setJenkinsLifecycleEnv()
	projectPath := targetProjectPath()

	if err := validation.ValidateProjectJenkinsfile(projectPath); err != nil {
		return err
	}
	if err := ValidateCommittedIntegratedJenkinsfile(); err != nil {
		return err
	}

	if err := runTeamGammaJenkins("start"); err != nil {
		return err
	}

	if err := jenkinsforward.EnsureStandaloneAfterGamma(); err != nil {
		log.Warn("Jenkins UI port-forward (Alpha): " + err.Error())
	}

	if err := syncIntegratedPlatformConfig(projectPath); err != nil {
		log.Warn("Platform config sync: " + err.Error())
	}

	if port, ok := gammastate.RegistryHostPort(); ok && port > 0 {
		if err := cluster.EnsureKindRegistryHTTPPull(port); err != nil {
			log.Warn("Kind registry pull config: " + err.Error())
		}
	}

	log.Success("Jenkins available at " + gammastate.EffectiveJenkinsLocalURL())
	return nil
}

// JenkinsStop stops UI exposure by default; full=true uninstalls the Helm release.
func JenkinsStop(full bool) error {
	setJenkinsLifecycleEnv()
	_ = jenkinsforward.StopPrevious()

	if full {
		log.Info("Removing Jenkins from cluster (Helm uninstall)")
		if err := runTeamGammaJenkins("stop", "--full"); err != nil {
			return err
		}
		log.Info("Registry still running — use devenv down for full cleanup")
		return nil
	}

	log.Info("Stopping Jenkins UI port-forward (cluster deployment unchanged)")
	if err := runTeamGammaJenkins("stop"); err != nil {
		return err
	}
	log.Info("Registry still running — use Team Gamma down or devenv down to remove it")
	return nil
}

// JenkinsStatus validates Helm release, pod health, and http://127.0.0.1:8080.
func JenkinsStatus() error {
	setJenkinsLifecycleEnv()

	if err := runTeamGammaJenkins("status"); err != nil {
		return err
	}

	if err := jenkinsforward.EnsureStandaloneAfterGamma(); err != nil {
		log.Warn("Could not ensure Alpha Jenkins port-forward: " + err.Error())
	} else {
		log.OK("Jenkins UI port-forward active at " + gammastate.EffectiveJenkinsLocalURL())
	}

	if rp, ok := gammastate.RegistryHostPort(); ok && rp > 0 {
		log.Info(fmt.Sprintf("Registry host port for pipelines: %d", rp))
	}
	return nil
}
