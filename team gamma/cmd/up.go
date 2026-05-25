package cmd

import (
	"fmt"

	"devenv-gamma/internal/jenkins"
	"devenv-gamma/internal/registry"
	"devenv-gamma/pkg/state"

	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start Jenkins and Docker Registry with dynamic port allocation",
	Long: `Start Team Gamma services (Jenkins and Docker Registry) with intelligent
dynamic port allocation. If default ports are occupied, alternate ports
will be automatically allocated and tracked.`,
	Run: func(cmd *cobra.Command, args []string) {
		subProcessPrintln("[INFO] Starting Team Gamma services")
		if subProcessVerbose() {
			fmt.Println("=== Starting Team Gamma Services ===")
			fmt.Println("[INFO] Using dynamic port allocation for Docker containers")
			fmt.Println()
		}

		fmt.Println("--- Docker Registry ---")
		if err := registry.StartRegistry(); err != nil {
			fmt.Println("[ERROR] Failed to start Registry:", err)
			fmt.Println("[INFO] Troubleshooting: Check if Docker is running")
			return
		}
		fmt.Println()

		fmt.Println("--- Jenkins (Kubernetes) ---")
		if err := jenkins.DeployJenkins(); err != nil {
			fmt.Println("[ERROR] Failed to deploy Jenkins:", err)
			fmt.Println("[INFO] Troubleshooting: Check if Kubernetes cluster is running")
			return
		}
		fmt.Println()

		subProcessPrintln("[OK] Team Gamma services are up")
		if subProcessVerbose() {
			fmt.Println("=== Team Gamma Services Ready ===")
		}
		displayServicesSummary()
		fmt.Println("\n--- Integrated CI/CD (devenv + Jenkins) ---")
		fmt.Println("[INFO] Jenkins job folder: devenv/local-ci-cd")
		fmt.Println("[INFO] Shared config: devenv-system/devenv-platform-config")
		fmt.Println("[INFO] Fast local loop (saved files, no git): devenv run")
		fmt.Println("[INFO] Full pipeline in Jenkins UI: Build devenv/local-ci-cd with GIT_URL")
	},
}

func displayServicesSummary() {
	runtimeState, err := state.GetRuntimeState()
	if err != nil {
		fmt.Println("[WARN] Could not load runtime state:", err)
		return
	}

	fmt.Println("\n📋 Service Configuration:")
	fmt.Println("┌─────────────────────────────────────────────┐")

	if runtimeState.Registry.Enabled {
		fmt.Printf("│ Docker Registry:                            │\n")
		fmt.Printf("│   URL:  %-35s │\n", runtimeState.Registry.URL)
		fmt.Printf("│   Port: %-35d │\n", runtimeState.Registry.HostPort)
		fmt.Printf("│   Status: %-33s │\n", getHealthStatus(runtimeState.Registry.Healthy))
	}

	fmt.Println("├─────────────────────────────────────────────┤")

	if runtimeState.Jenkins.Enabled {
		fmt.Printf("│ Jenkins:                                    │\n")
		fmt.Printf("│   URL:  %-35s │\n", runtimeState.Jenkins.URL)
		fmt.Printf("│   Port: %-35d │\n", runtimeState.Jenkins.UIPort)
		fmt.Printf("│   Login: admin / admin123                   │\n")
		fmt.Printf("│   Status: %-33s │\n", getHealthStatus(runtimeState.Jenkins.Healthy))
	}

	fmt.Println("└─────────────────────────────────────────────┘")

	fmt.Println("\n💡 Tips:")
	fmt.Println("  • View runtime state: devenv-gamma status --verbose")
	fmt.Println("  • Access Jenkins: open", runtimeState.Jenkins.URL)
	fmt.Println("  • Registry URL for pipelines:", runtimeState.Registry.URL)
	fmt.Println("  • Runtime config location:", getRuntimeStateLocation())
}

func getHealthStatus(healthy bool) string {
	if healthy {
		return "✓ Healthy"
	}
	return "⚠ Unknown"
}

func getRuntimeStateLocation() string {
	path, err := state.GetStateFilePath()
	if err != nil {
		return "(unknown)"
	}
	return path
}

func init() {
	rootCmd.AddCommand(upCmd)
}
