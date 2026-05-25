package deploy

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"devenv/teamalpha/internal/log"
	"devenv/teamalpha/internal/rollout"
)

// ContainerName returns the first container name on a Deployment.
func ContainerName(namespace, deployment string) string {
	out, err := exec.Command("kubectl", "get", "deployment", deployment,
		"-n", namespace,
		"-o", "jsonpath={.spec.template.spec.containers[0].name}",
	).Output()
	if err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			return s
		}
	}
	return deployment
}

// RunningImage returns the image reference currently configured on the Deployment template.
func RunningImage(namespace, deployment, container string) (string, error) {
	out, err := exec.Command("kubectl", "get", "deployment", deployment,
		"-n", namespace,
		"-o", fmt.Sprintf("jsonpath={.spec.template.spec.containers[?(@.name=='%s')].image}", container),
	).Output()
	if err != nil {
		return "", fmt.Errorf("get deployment image: %w", err)
	}
	img := strings.TrimSpace(string(out))
	if img == "" {
		return "", fmt.Errorf("deployment %s/%s has no image for container %s", namespace, deployment, container)
	}
	return img, nil
}

// SetImageOnly updates the deployment image reference without waiting for rollout.
func SetImageOnly(namespace, deployment, container, imageRef string) error {
	log.Info(fmt.Sprintf("Setting deployment image: %s", imageRef))
	out, err := exec.Command("kubectl", "set", "image",
		fmt.Sprintf("deployment/%s", deployment),
		fmt.Sprintf("%s=%s", container, imageRef),
		"-n", namespace,
		"--record",
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl set image: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SetImage switches the deployment to an existing registry image and waits for rollout (rollback path).
func SetImage(namespace, deployment, container, imageRef string) error {
	if err := SetImageOnly(namespace, deployment, container, imageRef); err != nil {
		return err
	}
	return rollout.MonitorRollout(deployment, namespace)
}

// WaitForRollout blocks until the deployment rollout completes or times out.
func WaitForRollout(namespace, deployment string) error {
	return rollout.MonitorRollout(deployment, namespace)
}

// TagFromImageRef extracts the tag portion from host/repo:tag (best effort).
func TagFromImageRef(imageRef string) string {
	imageRef = strings.TrimSpace(imageRef)
	if idx := strings.LastIndex(imageRef, ":"); idx >= 0 && idx < len(imageRef)-1 {
		tag := imageRef[idx+1:]
		if !strings.Contains(tag, "/") {
			return tag
		}
	}
	return ""
}

// ImageRef builds a registry pull reference for kind/docker desktop clusters.
func ImageRef(registryHost, appName, tag string) string {
	return fmt.Sprintf("%s/%s:%s", registryHost, appName, tag)
}

// AnnotateDeployment records build metadata on the deployment object.
func AnnotateDeployment(namespace, deployment, tag string) {
	_ = exec.Command("kubectl", "annotate", "deployment", deployment,
		"-n", namespace,
		"devenv.cloudtechner/deploy-tag="+tag,
		"devenv.cloudtechner/deployed-at="+time.Now().UTC().Format(time.RFC3339),
		"--overwrite",
	).Run()
}
