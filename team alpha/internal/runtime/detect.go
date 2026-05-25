package runtime

import (
	"os"
	"path/filepath"
)

func DetectFramework(projectPath string) string {

	if fileExists(filepath.Join(projectPath, "package.json")) {
		return "react"
	}

	if fileExists(filepath.Join(projectPath, "requirements.txt")) {
		return "python"
	}

	if fileExists(filepath.Join(projectPath, "pom.xml")) {
		return "java_springboot"
	}

	if fileExists(filepath.Join(projectPath, "go.mod")) {
		return "go"
	}

	return "unknown"
}

func fileExists(path string) bool {

	_, err := os.Stat(path)

	return err == nil
}
