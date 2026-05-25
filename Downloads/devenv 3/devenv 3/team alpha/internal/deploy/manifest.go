package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UpdateKustomizeImageTag sets images[].newTag in the local overlay kustomization.
func UpdateKustomizeImageTag(projectPath, appName, tag string) error {
	kustomizationPath := filepath.Join(projectPath, "k8s", "overlays", "local", "kustomization.yml")
	data, err := os.ReadFile(kustomizationPath)
	if err != nil {
		return fmt.Errorf("read kustomization: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	targetBlock := false
	updated := false
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "- name:") && strings.Contains(trim, appName) {
			targetBlock = true
			continue
		}
		if targetBlock && strings.HasPrefix(trim, "newTag:") {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + "newTag: " + tag
			updated = true
			targetBlock = false
			continue
		}
		if targetBlock && strings.HasPrefix(trim, "- name:") {
			targetBlock = false
		}
	}

	if !updated {
		return fmt.Errorf("could not find images.newTag for %s in %s", appName, kustomizationPath)
	}
	return os.WriteFile(kustomizationPath, []byte(strings.Join(lines, "\n")), 0644)
}
