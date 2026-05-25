package cluster

import (
	"fmt"
	"os/exec"
	"strings"
)

func CheckClusterStatus() error {
	fmt.Println("\n--- [CLUSTER STATUS] ---")

	// 1. Active Kind clusters check karo
	fmt.Println("Checking for active Kind clusters...")
	kindCmd := exec.Command("kind", "get", "clusters")
	kindOut, err := kindCmd.Output()
	if err != nil {
		return fmt.Errorf("ERROR running kind command: %v", err)
	}

	// output clean karke check karo
	clusters := strings.TrimSpace(string(kindOut))
	if clusters == "" {
		return fmt.Errorf("❌ No active Kind clusters found. Please run 'devenv setup' first")
	}
	fmt.Printf("✅ Active Clusters found:\n%s\n\n", clusters)

	// 2. K8s nodes ka status dekho
	fmt.Println("Checking Kubernetes Nodes Status...")
	nodeCmd := exec.Command("kubectl", "get", "nodes")
	nodeOut, err := nodeCmd.Output()
	if err != nil {
		fmt.Printf("❌ Could not get nodes: %v\n", err)
	} else {
		fmt.Println(string(nodeOut))
	}

	// 3. System pods check karo (NGINX etc.)
	fmt.Println("\nChecking System Pods (including NGINX)...")
	podCmd := exec.Command("kubectl", "get", "pods", "-A") // -A sab namespaces ke liye
	podOut, err := podCmd.Output()
	if err != nil {
		fmt.Printf("❌ Could not get pods: %v\n", err)
	} else {
		fmt.Println(string(podOut))
	}

	fmt.Println("------------------------")
	return nil
}
