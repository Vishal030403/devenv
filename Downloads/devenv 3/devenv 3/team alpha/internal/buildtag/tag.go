package buildtag

import (
	"os/exec"
	"strings"
	"time"
)

// UniqueImageTag returns a tag that changes on every pipeline run so Kubernetes rolls out
// new pods even when you have not run git commit (local Jenkins / devenv workflow).
//
// Format examples:
//   - 7aea1ed-20260519-153045   (in a git repo, clean or dirty)
//   - build-20260519-153045       (no git repo)
func UniqueImageTag(projectPath string) string {
	stamp := time.Now().Format("20060102-150405")
	base := gitShortBase(projectPath)
	if base == "" {
		base = "build"
	}
	return base + "-" + stamp
}

func gitShortBase(projectPath string) string {
	if projectPath == "" {
		return ""
	}
	out, err := exec.Command("git", "-C", projectPath, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	sha := strings.TrimSpace(string(out))
	if sha == "" {
		return ""
	}
	if dirty, _ := gitWorkingTreeDirty(projectPath); dirty {
		return sha + "-dirty"
	}
	return sha
}

func gitWorkingTreeDirty(projectPath string) (bool, error) {
	out, err := exec.Command("git", "-C", projectPath, "status", "--porcelain").Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}
