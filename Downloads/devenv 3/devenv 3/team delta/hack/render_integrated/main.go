//go:build ignore

// Sync Team Gamma Jenkinsfile.integrated from templates/pipeline/Jenkinsfile.tmpl
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"pipeline-cli/scaffolding_engine/core/generator"
)

func main() {
	script, err := generator.RenderDefaultIntegratedPipeline()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	dest := filepath.Join("..", "..", "..", "team gamma", "pipelines", "Jenkinsfile.integrated")
	if err := os.WriteFile(dest, []byte(script), 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Wrote", dest)
}
