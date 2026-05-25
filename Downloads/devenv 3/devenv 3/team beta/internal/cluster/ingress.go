package cluster

import (
	"fmt"
	"os/exec"
	"time"
)

func InstallNginxIngress() error {
	fmt.Println("Starting NGINX Ingress Installation...")

	// 1. NGINX manifest apply karo
	fmt.Println("Downloading and applying NGINX manifests...")
	applyCmd := exec.Command("kubectl", "apply", "-f", "https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml")

	// execute aur error check
	applyOut, err := applyCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ERROR applying Ingress: %v\nOutput: %s", err, string(applyOut))
	}
	fmt.Println("✅ NGINX manifests applied successfully.")

	// 2. Ingress pods ready hone ka wait
	fmt.Println("Waiting for NGINX to be fully ready (this usually takes 1-2 minutes)...")

	// 30 retries * 5s = max 2.5 mins wait
	maxRetries := 30
	for i := 1; i <= maxRetries; i++ {
		// pod 'ready' status check
		waitCmd := exec.Command("kubectl", "wait", "--namespace", "ingress-nginx",
			"--for=condition=ready", "pod",
			"--selector=app.kubernetes.io/component=controller",
			"--timeout=5s")

		err := waitCmd.Run()
		if err == nil {
			// sab sahi hai, pods are ready
			fmt.Println("✅ NGINX Ingress Controller is fully up and running!")
			return nil
		}

		fmt.Printf("Still waiting... (Attempt %d of %d)\n", i, maxRetries)
		time.Sleep(5 * time.Second) // 5s delay next check se pehle
	}

	// loop khatam matlab timeout
	return fmt.Errorf("ERROR: NGINX Ingress setup timed out. The cluster might have a problem")
}
