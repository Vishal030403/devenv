package generator

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"pipeline-cli/scaffolding_engine/templates"
)

// GenerateFiles generates the scaffolding files based on the framework.
func GenerateFiles(framework string, projectPath string, entryPath string) error {
	appName := SanitizeAppName(projectPath)
	spec := ResolveFrameworkSpec(framework, projectPath, entryPath)
	tmplVars := spec.ToTemplateVars(appName)

	if err := GeneratePipelineFile(framework, projectPath, spec, false); err != nil {
		return err
	}

	if err := generateDevenvYAML(projectPath, spec, appName); err != nil {
		return err
	}

	return fs.WalkDir(templates.Files, framework, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("templates directory not found for framework: %s", framework)
		}
		if d.IsDir() {
			return nil
		}

		if strings.HasSuffix(d.Name(), ".tmpl") {
			relPath := strings.TrimPrefix(path, framework+"/")
			outputRelPath := strings.TrimSuffix(relPath, ".tmpl")
			outputPath := filepath.Join(projectPath, outputRelPath)

			if err := os.MkdirAll(filepath.Dir(outputPath), os.ModePerm); err != nil {
				return err
			}

			fileData, err := templates.Files.ReadFile(path)
			if err != nil {
				return err
			}

			tmpl, err := template.New(filepath.Base(path)).Parse(string(fileData))
			if err != nil {
				return err
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, tmplVars); err != nil {
				return fmt.Errorf("error executing template %s: %w", path, err)
			}

			if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
				return err
			}

			fmt.Println("Generated", outputPath)
		}
		return nil
	})
}

// generateDevenvYAML writes a minimal runtime config when missing (consumed by devenv run).
func generateDevenvYAML(projectPath string, spec FrameworkSpec, appName string) error {
	outPath := filepath.Join(projectPath, "devenv.yaml")
	if _, err := os.Stat(outPath); err == nil {
		return nil
	}
	content := fmt.Sprintf(`app:
  name: %s
  framework: %s

registry:
  url: localhost:5000

namespace: %s%s

environment: local

deployment:
  replicas: 1
  port: %d
  health_path: %q
`, appName, spec.Framework, appName, spec.NamespaceSuffix, spec.AppPort, spec.HealthPath)
	if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
		return err
	}
	fmt.Println("Generated", outPath)
	return nil
}
