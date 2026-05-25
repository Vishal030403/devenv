package cluster

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"devenv/teambeta/pkg/config"
)

func CreateCluster() error {

	fmt.Println("[INFO] Initializing cluster")

	out, err := exec.Command("kind", "get", "clusters").Output()
	if err != nil {
		return fmt.Errorf("failed to get clusters: %w", err)
	}

	clusters := strings.Split(string(out), "\n")
	for _, c := range clusters {
		if strings.TrimSpace(c) == config.ClusterName {
			fmt.Println("[INFO] Cluster already exists")

			contextName := "kind-" + config.ClusterName
			exec.Command("kubectl", "config", "use-context", contextName).Run()

			return nil
		}
	}

	cmd := exec.Command("kind", "create", "cluster", "--name", config.ClusterName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return errors.New("cluster creation failed")
	}

	contextName := "kind-" + config.ClusterName
	exec.Command("kubectl", "config", "use-context", contextName).Run()

	if err := exec.Command("kubectl", "cluster-info").Run(); err != nil {
		return errors.New("cluster not reachable")
	}

	fmt.Println("[OK] Cluster ready")

	return nil
}

func DeleteCluster() error {

	fmt.Println("[INFO] Cleaning cluster")

	out, err := exec.Command("kind", "get", "clusters").Output()
	if err != nil {
		return fmt.Errorf("failed to get clusters: %w", err)
	}

	clusters := strings.Split(string(out), "\n")

	found := false
	for _, c := range clusters {
		if strings.TrimSpace(c) == config.ClusterName {
			found = true
			break
		}
	}

	if !found {
		fmt.Println("[INFO] No cluster found")
		return nil
	}

	cmd := exec.Command("kind", "delete", "cluster", "--name", config.ClusterName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return errors.New("failed to delete cluster")
	}

	contextName := "kind-" + config.ClusterName
	exec.Command("kubectl", "config", "delete-context", contextName).Run()

	fmt.Println("[OK] Cluster deleted")

	return nil
}
