package cmd

import (
	"fmt"

	"devenv/teambeta/internal/cluster"

	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Destroy development environment",
	Run: func(cmd *cobra.Command, args []string) {

		fmt.Println("[INFO] Stopping environment")

		if err := cluster.DeleteCluster(); err != nil {
			fmt.Println("[ERROR]", err)
			return
		}

		fmt.Println("[OK] Environment cleaned")
	},
}

func init() {
	rootCmd.AddCommand(downCmd)
}
