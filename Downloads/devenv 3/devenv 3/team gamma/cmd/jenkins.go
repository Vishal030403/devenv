package cmd

import (
	"fmt"

	"devenv-gamma/internal/jenkins"

	"github.com/spf13/cobra"
)

var jenkinsFullStop bool

var jenkinsCmd = &cobra.Command{
	Use:   "jenkins",
	Short: "Manage Jenkins lifecycle (start, stop, status)",
}

var jenkinsStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start registry (if needed), deploy Jenkins, expose UI on localhost:8080",
	Run: func(cmd *cobra.Command, args []string) {
		if err := jenkins.StartIntegrated(); err != nil {
			fmt.Println("[ERROR]", err)
			return
		}
		fmt.Println("[OK] Jenkins started")
	},
}

var jenkinsStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop Jenkins UI port-forward, or remove Jenkins from cluster with --full",
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		if jenkinsFullStop {
			err = jenkins.StopIntegrated()
		} else {
			err = jenkins.StopExposure()
		}
		if err != nil {
			fmt.Println("[ERROR]", err)
			return
		}
		fmt.Println("[OK] Jenkins stopped")
	},
}

var jenkinsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check Jenkins Helm release, pod health, and localhost:8080 UI",
	Run: func(cmd *cobra.Command, args []string) {
		if err := jenkins.CheckJenkinsStatus(); err != nil {
			fmt.Println("[ERROR]", err)
			return
		}
	},
}

func init() {
	rootCmd.AddCommand(jenkinsCmd)
	jenkinsCmd.AddCommand(jenkinsStartCmd, jenkinsStopCmd, jenkinsStatusCmd)
	jenkinsStopCmd.Flags().BoolVar(&jenkinsFullStop, "full", false, "Uninstall Jenkins Helm release from the cluster")
}
