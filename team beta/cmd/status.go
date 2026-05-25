package cmd

import (
	"devenv/teambeta/internal/cluster"
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check cluster and pods status",
	Run: func(cmd *cobra.Command, args []string) {
		if err := cluster.CheckClusterStatus(); err != nil {
			fmt.Println("[ERROR]", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
