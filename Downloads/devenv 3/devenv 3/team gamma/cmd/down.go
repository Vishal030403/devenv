package cmd

import (
	"devenv-gamma/internal/jenkins"
	"devenv-gamma/internal/registry"
	"devenv-gamma/pkg/k8sutil"
	"devenv-gamma/pkg/state"
	"fmt"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop and remove Jenkins, Docker Registry, and active port-forwards",
	Long: `Stop and remove all Team Gamma services including:
- Jenkins deployment
- Docker Registry container
- Active kubectl port-forward processes`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("=== Stopping Team Gamma Services ===\n")

		// Step 1: Stop all persistent port-forwards (kubectl)
		fmt.Println("[INFO] Cleaning stale port-forward processes...")
		fmt.Println("--- Stopping Port-Forwards ---")
		manager := k8sutil.GetPortForwardManager()
		if err := manager.StopAll(); err != nil {
			fmt.Printf("[WARN] Port-forward cleanup had errors: %v\n", err)
		}
		fmt.Println()
		
		// Step 2: Cleanup Jenkins
		fmt.Println("--- Stopping Jenkins ---")
		if err := jenkins.CleanupJenkins(); err != nil {
			fmt.Println("[ERROR] Jenkins cleanup:", err)
		}
		fmt.Println()

		// Step 3: Cleanup Registry
		fmt.Println("--- Stopping Registry ---")
		if err := registry.StopRegistry(); err != nil {
			fmt.Println("[ERROR] Registry cleanup:", err)
		}
		fmt.Println()
		
		// Step 4: Clear runtime state
		fmt.Println("--- Clearing Runtime State ---")
		if err := state.ClearRuntimeState(); err != nil {
			fmt.Printf("[WARN] Failed to clear runtime state: %v\n", err)
		} else {
			fmt.Println("[OK] Runtime state cleared")
		}

		fmt.Println("\n=== Team Gamma Services Stopped ===")
	},
}

func init() {
	rootCmd.AddCommand(downCmd)
}
