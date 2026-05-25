package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"pipeline-cli/scaffolding_engine/core/detector"
	"pipeline-cli/scaffolding_engine/core/generator"
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	var err error
	switch os.Args[1] {
	case "up":
		err = runUp(os.Args[2:])
	case "status":
		err = runStatus(os.Args[2:])
	case "down":
		err = runDown(os.Args[2:])
	case "--help", "-h", "help":
		printHelp()
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}

	if err != nil {
		fmt.Println("[ERROR]", err)
		os.Exit(1)
	}
}

func runUp(args []string) error {
	projectPath, err := parseProjectPath("up", args)
	if err != nil {
		return err
	}

	framework, entryPath := detector.DetectFramework(projectPath)
	if framework == "unknown" {
		fmt.Printf("[WARN] No supported framework detected in %s\n", projectPath)
		fmt.Println("[WARN] Skipping template generation")
		return nil
	}

	fmt.Printf("[OK] Detected framework: %s\n", framework)
	if entryPath != "" {
		fmt.Printf("[OK] Detected entry path: %s\n", entryPath)
	}

	if err := generator.GenerateFiles(framework, projectPath, entryPath); err != nil {
		return err
	}

	fmt.Println("[OK] Scaffolding generation completed")
	return nil
}

func runStatus(args []string) error {
	projectPath, err := parseProjectPath("status", args)
	if err != nil {
		return err
	}

	framework, entryPath := detector.DetectFramework(projectPath)
	if framework == "unknown" {
		fmt.Printf("[WARN] No supported framework detected in %s\n", projectPath)
		return nil
	}

	fmt.Printf("[OK] Detected framework: %s\n", framework)
	if entryPath != "" {
		fmt.Printf("[OK] Detected entry path: %s\n", entryPath)
	}

	checkFile(projectPath, "Dockerfile")
	checkFile(projectPath, filepath.Join("k8s", "base", "deployment.yml"))
	checkFile(projectPath, filepath.Join("k8s", "base", "service.yml"))
	checkFile(projectPath, filepath.Join("k8s", "base", "ingress.yml"))
	checkFile(projectPath, filepath.Join("k8s", "base", "kustomization.yml"))
	checkFile(projectPath, "Jenkinsfile")
	checkFile(projectPath, "devenv.yaml")
	return nil
}

func runDown(args []string) error {
	projectPath, err := parseProjectPath("down", args)
	if err != nil {
		return err
	}

	fmt.Printf("[INFO] Team Delta cleanup requested for %s\n", projectPath)
	fmt.Println("[WARN] Delta does not delete generated Dockerfile or k8s files automatically")
	return nil
}

func parseProjectPath(command string, args []string) (string, error) {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	pathFlag := fs.String("path", ".", "project path to inspect or scaffold")
	if err := fs.Parse(args); err != nil {
		return "", err
	}

	absPath, err := filepath.Abs(*pathFlag)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("project path is not accessible: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project path is not a directory: %s", absPath)
	}

	return absPath, nil
}

func checkFile(projectPath, relPath string) {
	fullPath := filepath.Join(projectPath, relPath)
	if _, err := os.Stat(fullPath); err == nil {
		fmt.Printf("[OK] %s exists\n", relPath)
		return
	}
	fmt.Printf("[WARN] %s missing\n", relPath)
}

func printHelp() {
	fmt.Println("Team Delta scaffolding CLI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go run . up --path <project-path>")
	fmt.Println("  go run . status --path <project-path>")
	fmt.Println("  go run . down --path <project-path>")
}
