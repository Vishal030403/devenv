package cmd

import (
	"devenv/teamalpha/orchestrator"

	"github.com/spf13/cobra"
)

var pipelineJenkinsFullStop bool

var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Pipeline platform commands (Jenkins lifecycle, shared with devenv setup/run)",
}

var pipelineTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Offline pipeline verification (Jenkinsfile syntax, manifests, templates)",
}

var pipelineTestSecurityCmd = &cobra.Command{
	Use:   "security",
	Short: "Validate Jenkinsfile syntax, k8s manifests, and project structure",
	RunE: func(cmd *cobra.Command, args []string) error {
		return orchestrator.PipelineTestSecurity()
	},
}

var pipelineTestAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Run security checks plus integrated and per-framework Jenkinsfile validation",
	RunE: func(cmd *cobra.Command, args []string) error {
		return orchestrator.PipelineTestAll()
	},
}

var pipelineJenkinsCmd = &cobra.Command{
	Use:   "jenkins",
	Short: "Start, stop, or check Jenkins (in-cluster Helm + localhost:8080)",
}

var pipelineJenkinsStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start registry, deploy Jenkins, expose UI on http://127.0.0.1:8080",
	RunE: func(cmd *cobra.Command, args []string) error {
		return orchestrator.JenkinsStart()
	},
}

var pipelineJenkinsStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop Jenkins UI port-forward (default) or remove Helm release with --full",
	RunE: func(cmd *cobra.Command, args []string) error {
		return orchestrator.JenkinsStop(pipelineJenkinsFullStop)
	},
}

var pipelineJenkinsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check Jenkins Helm release, pod health, and UI on localhost:8080",
	RunE: func(cmd *cobra.Command, args []string) error {
		return orchestrator.JenkinsStatus()
	},
}

func init() {
	rootCmd.AddCommand(pipelineCmd)
	pipelineCmd.AddCommand(pipelineTestCmd, pipelineJenkinsCmd)
	pipelineTestCmd.AddCommand(pipelineTestSecurityCmd, pipelineTestAllCmd)
	pipelineJenkinsCmd.AddCommand(pipelineJenkinsStartCmd, pipelineJenkinsStopCmd, pipelineJenkinsStatusCmd)
	pipelineJenkinsStopCmd.Flags().BoolVar(&pipelineJenkinsFullStop, "full", false, "Uninstall Jenkins Helm release from the cluster")
}
