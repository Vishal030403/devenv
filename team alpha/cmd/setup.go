package cmd

import (
	"devenv/teamalpha/orchestrator"

	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup environment prerequisites",
	RunE: func(cmd *cobra.Command, args []string) error {
		return orchestrator.Setup()
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
