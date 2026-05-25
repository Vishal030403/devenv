package cmd

import (
	"fmt"

	"devenv-gamma/internal/jenkins"
	"devenv-gamma/internal/registry"
	"devenv-gamma/pkg/state"

	"github.com/spf13/cobra"
)

var (
	verboseStatus bool
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check Jenkins and Docker Registry status with runtime configuration",
	Long: `Check the health and configuration of Team Gamma services including
dynamic port allocations and runtime state.`,
	Run: func(cmd *cobra.Command, args []string) {
		subProcessPrintln("[INFO] Checking Team Gamma service status")
		if subProcessVerbose() {
			fmt.Println("=== Team Gamma Service Status ===\n")
		}

		fmt.Println("--- Docker Registry ---")
		if err := registry.CheckRegistryStatus(); err != nil {
			fmt.Println("[ERROR] Registry:", err)
		}
		fmt.Println()

		fmt.Println("--- Jenkins ---")
		if err := jenkins.CheckJenkinsStatus(); err != nil {
			fmt.Println("[ERROR] Jenkins:", err)
		}
		fmt.Println()

		if verboseStatus {
			fmt.Println("--- Runtime State ---")
			if err := state.PrintRuntimeState(); err != nil {
				fmt.Println("[ERROR] Failed to load runtime state:", err)
			}
			fmt.Println()
		}

		displayQuickSummary()
	},
}

func displayQuickSummary() {
	runtimeState, err := state.GetRuntimeState()
	if err != nil {
		fmt.Println("[WARN] Could not load runtime state:", err)
		return
	}

	fmt.Println("=== Quick Summary ===")

	registryStatus := "❌ Not Running"
	if runtimeState.Registry.Enabled && runtimeState.Registry.Healthy {
		registryStatus = fmt.Sprintf("✅ Running on localhost:%d", runtimeState.Registry.HostPort)
	} else if runtimeState.Registry.Enabled {
		registryStatus = fmt.Sprintf("⚠️  Running but health unknown (localhost:%d)", runtimeState.Registry.HostPort)
	}

	jenkinsStatus := "❌ Not Running"
	if runtimeState.Jenkins.Enabled && runtimeState.Jenkins.Healthy {
		jenkinsStatus = fmt.Sprintf("✅ Running on localhost:%d", runtimeState.Jenkins.UIPort)
	} else if runtimeState.Jenkins.Enabled {
		jenkinsStatus = fmt.Sprintf("⚠️  Running but health unknown (localhost:%d)", runtimeState.Jenkins.UIPort)
	}

	fmt.Printf("Registry: %s\n", registryStatus)
	fmt.Printf("Jenkins:  %s\n", jenkinsStatus)

	if runtimeState.Deployments != nil {
		for key, d := range runtimeState.Deployments {
			if d.CurrentImage == "" {
				continue
			}
			fmt.Printf("Deploy %s: %s", key, d.CurrentImage)
			if d.RollbackTarget != "" {
				fmt.Printf(" (rollback: %s)", d.RollbackTarget)
			}
			fmt.Println()
		}
	}

	if !verboseStatus {
		fmt.Println("\n💡 Use --verbose flag to see detailed runtime state")
	}
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().BoolVarP(&verboseStatus, "verbose", "v", false, "Show detailed runtime state and configuration")
}
