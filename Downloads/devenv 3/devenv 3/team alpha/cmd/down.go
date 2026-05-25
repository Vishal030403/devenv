package cmd

import (
	"devenv/teamalpha/orchestrator"

	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Clean up the local environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		return orchestrator.Down()
	},
}

func init() {
	rootCmd.AddCommand(downCmd)
}
