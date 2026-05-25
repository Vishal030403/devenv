package cmd

import (
	"fmt"

	"devenv/teambeta/internal/cluster"
	"devenv/teambeta/internal/preflight"

	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start development environment",
	Run: func(cmd *cobra.Command, args []string) {

		fmt.Println("[INFO] Starting environment")

		res, err := preflight.RunPreflightChecks()
		if err != nil {
			fmt.Println("[ERROR]", err)
			return
		}

		fmt.Println("[INFO] Using registry port:", res.RegistryPort)

		if err := cluster.CreateCluster(); err != nil {
			fmt.Println("[ERROR]", err)
			return
		}

		if err := cluster.InstallNginxIngress(); err != nil {
			fmt.Println("[ERROR]", err)
			return
		}

		fmt.Println("[OK] Environment ready")
	},
}

func init() {
	rootCmd.AddCommand(upCmd)
}
