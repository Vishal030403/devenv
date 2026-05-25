package cmd

import (
	"devenv/teamalpha/orchestrator"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check local environment status",
	RunE: func(cmd *cobra.Command, args []string) error {
		return orchestrator.Status()
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
