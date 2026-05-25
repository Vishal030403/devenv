package cmd

import (
	"devenv/teamalpha/orchestrator"

	"github.com/spf13/cobra"
)

var useLocal bool

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the full local CI/CD flow via Jenkins",
	Long: `Run the integrated local CI/CD pipeline through Jenkins.

By default devenv triggers the Jenkins job (devenv/local-ci-cd) which builds,
tests, pushes to the local registry, and deploys to the Kind cluster.

Use --local to run the pipeline directly on your machine instead of Jenkins.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return orchestrator.RunPipelineOptions(orchestrator.PipelineOptions{
			UseLocal: useLocal,
		})
	},
}

func init() {
	runCmd.Flags().BoolVar(&useLocal, "local", false, "Run pipeline directly on host instead of via Jenkins")
	rootCmd.AddCommand(runCmd)
}
