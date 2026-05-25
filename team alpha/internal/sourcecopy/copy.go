package sourcecopy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"devenv/teamalpha/internal/log"
)

const (
	pvcName    = "devenv-workspace-pvc"
	namespace  = "jenkins"
	loaderPod  = "devenv-source-loader"
	mountPath  = "/devenv-source"
	pvcStorage = "500Mi"
)

// CopySourceToCluster copies the project source into a PVC accessible by Jenkins agents.
// Flow: tar project → create PVC → start loader pod → kubectl cp → extract → cleanup pod.
func CopySourceToCluster(projectPath string) error {
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("resolve project path: %w", err)
	}
	log.Info("Uploading project source to Jenkins workspace PVC...")

	// 1. Create tarball
	tarPath, err := createTarball(abs)
	if err != nil {
		return fmt.Errorf("create tarball: %w", err)
	}
	defer os.Remove(tarPath)
	log.OK("Source tarball created")

	// 2. Ensure PVC exists
	if err := ensurePVC(); err != nil {
		return fmt.Errorf("ensure PVC: %w", err)
	}
	log.OK("Workspace PVC ready")

	// 3. Start loader pod
	if err := startLoaderPod(); err != nil {
		return fmt.Errorf("start loader pod: %w", err)
	}
	defer cleanupLoaderPod()
	log.OK("Source loader pod running")

	// 4. Copy tarball and extract
	if err := copyAndExtract(tarPath); err != nil {
		return fmt.Errorf("copy source to cluster: %w", err)
	}
	log.OK("Source uploaded to workspace PVC")
	return nil
}

// CleanupPVC removes the workspace PVC (called during devenv down).
func CleanupPVC() {
	_ = exec.Command("kubectl", "delete", "pvc", pvcName, "-n", namespace, "--ignore-not-found").Run()
	_ = exec.Command("kubectl", "delete", "pod", loaderPod, "-n", namespace, "--ignore-not-found").Run()
}

func createTarball(projectPath string) (string, error) {
	tmp, err := os.CreateTemp("", "devenv-source-*.tar.gz")
	if err != nil {
		return "", err
	}
	tarPath := tmp.Name()
	_ = tmp.Close()

	cmd := exec.Command("tar", "czf", tarPath,
		"--exclude=.git",
		"--exclude=node_modules",
		"--exclude=dist",
		"--exclude=build",
		"--exclude=__pycache__",
		"--exclude=.devenv.lock",
		"-C", projectPath, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(tarPath)
		return "", fmt.Errorf("tar: %s: %w", strings.TrimSpace(string(out)), err)
	}

	info, err := os.Stat(tarPath)
	if err != nil {
		return "", err
	}
	log.Info(fmt.Sprintf("Source tarball: %d KB", info.Size()/1024))
	return tarPath, nil
}

func ensurePVC() error {
	// Check if PVC exists
	check := exec.Command("kubectl", "get", "pvc", pvcName, "-n", namespace, "--no-headers")
	if out, err := check.Output(); err == nil && strings.TrimSpace(string(out)) != "" {
		return nil // Already exists
	}

	manifest := fmt.Sprintf(`apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: %s
  namespace: %s
  labels:
    app.kubernetes.io/managed-by: devenv
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: %s`, pvcName, namespace, pvcStorage)

	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}

	// Wait for PVC to be bound
	wait := exec.Command("kubectl", "wait", "--for=jsonpath={.status.phase}=Bound",
		"pvc/"+pvcName, "-n", namespace, "--timeout=30s")
	if out, err := wait.CombinedOutput(); err != nil {
		log.Warn(fmt.Sprintf("PVC may not be bound yet: %s", strings.TrimSpace(string(out))))
		// Don't fail — some provisioners bind lazily when first mounted
	}
	return nil
}

func startLoaderPod() error {
	// Delete previous if exists
	_ = exec.Command("kubectl", "delete", "pod", loaderPod, "-n", namespace,
		"--ignore-not-found", "--grace-period=0", "--force").Run()
	time.Sleep(1 * time.Second)

	manifest := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
  labels:
    app.kubernetes.io/managed-by: devenv
spec:
  restartPolicy: Never
  containers:
  - name: loader
    image: busybox:latest
    command: ["sh", "-c", "sleep 300"]
    volumeMounts:
    - name: source
      mountPath: %s
  volumes:
  - name: source
    persistentVolumeClaim:
      claimName: %s`, loaderPod, namespace, mountPath, pvcName)

	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}

	// Wait for pod to be ready
	wait := exec.Command("kubectl", "wait", "--for=condition=ready",
		"pod/"+loaderPod, "-n", namespace, "--timeout=90s")
	if out, err := wait.CombinedOutput(); err != nil {
		return fmt.Errorf("loader pod not ready: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func copyAndExtract(tarPath string) error {
	// Clear previous source in PVC
	clearCmd := exec.Command("kubectl", "exec", loaderPod, "-n", namespace, "--",
		"sh", "-c", fmt.Sprintf("rm -rf %s/* %s/.[!.]* 2>/dev/null; mkdir -p %s", mountPath, mountPath, mountPath))
	_ = clearCmd.Run()

	// Copy tarball to pod
	dest := fmt.Sprintf("%s/%s:%s/source.tar.gz", namespace, loaderPod, mountPath)
	cpCmd := exec.Command("kubectl", "cp", tarPath, dest)
	if out, err := cpCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("kubectl cp: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// Extract and remove tarball
	extract := exec.Command("kubectl", "exec", loaderPod, "-n", namespace, "--",
		"sh", "-c", fmt.Sprintf("cd %s && tar xzf source.tar.gz && rm -f source.tar.gz", mountPath))
	if out, err := extract.CombinedOutput(); err != nil {
		return fmt.Errorf("extract: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// Verify extraction
	verify := exec.Command("kubectl", "exec", loaderPod, "-n", namespace, "--",
		"sh", "-c", fmt.Sprintf("ls -la %s/ | head -20", mountPath))
	if out, err := verify.CombinedOutput(); err == nil {
		log.Info("PVC contents:\n" + strings.TrimSpace(string(out)))
	}

	return nil
}

func cleanupLoaderPod() {
	_ = exec.Command("kubectl", "delete", "pod", loaderPod, "-n", namespace,
		"--ignore-not-found", "--grace-period=0", "--force").Run()
}
