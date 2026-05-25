package generator

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"pipeline-cli/scaffolding_engine/core/validator"
	"pipeline-cli/scaffolding_engine/templates"
)

// PipelineVars are Go-template variables for Jenkinsfile.tmpl.
type PipelineVars struct {
	Framework       string
	AppName         string
	HealthPath      string
	AppPort         int
	K8sOverlay      string
	K8sNamespace    string
	NamespaceSuffix string
	DockerImageName string
	DeploymentName  string
	LintCommand     string
	UnitTestCommand string
}

func pipelineVarsFromSpec(spec FrameworkSpec, appName string) PipelineVars {
	if appName == "" {
		appName = "app"
	}
	return spec.ToPipelineVars(appName)
}

// RenderPipeline executes pipeline/Jenkinsfile.tmpl with the given variables.
func RenderPipeline(v PipelineVars) (string, error) {
	data, err := templates.Files.ReadFile("pipeline/Jenkinsfile.tmpl")
	if err != nil {
		return "", fmt.Errorf("read pipeline template: %w", err)
	}
	funcMap := template.FuncMap{
		"groovy": groovyEscape,
	}
	tmpl, err := template.New("Jenkinsfile").Funcs(funcMap).Parse(string(data))
	if err != nil {
		return "", fmt.Errorf("parse pipeline template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, v); err != nil {
		return "", fmt.Errorf("execute pipeline template: %w", err)
	}
	return buf.String(), nil
}

func groovyEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// RenderDefaultIntegratedPipeline renders the canonical in-cluster Jenkins script (no project-specific names).
func RenderDefaultIntegratedPipeline() (string, error) {
	return RenderPipeline(PipelineVars{
		Framework:       "generic",
		AppName:         "app",
		HealthPath:      "/",
		AppPort:         80,
		K8sOverlay:      "k8s/overlays/local",
		K8sNamespace:    "app-ns",
		NamespaceSuffix: "-ns",
		DockerImageName: "app",
		DeploymentName:  "app",
		LintCommand:     "",
		UnitTestCommand: "",
	})
}

// GeneratePipelineFile writes Jenkinsfile into the project when missing or when force is true.
func GeneratePipelineFile(framework, projectPath string, spec FrameworkSpec, force bool) error {
	appName := SanitizeAppName(projectPath)
	vars := pipelineVarsFromSpec(spec, appName)
	rendered, err := RenderPipeline(vars)
	if err != nil {
		return err
	}

	outPath := filepath.Join(projectPath, "Jenkinsfile")
	if !force {
		if _, err := os.Stat(outPath); err == nil {
			fmt.Println("[INFO] Jenkinsfile already exists — skipping (delete to regenerate)")
			return nil
		}
	}

	if err := validator.ValidateContent(rendered, outPath, true); err != nil {
		return fmt.Errorf("generated Jenkinsfile failed validation: %w", err)
	}

	if err := os.WriteFile(outPath, []byte(rendered), 0644); err != nil {
		return fmt.Errorf("write Jenkinsfile: %w", err)
	}
	fmt.Printf("Generated %s (framework=%s, tests=%q)\n", outPath, framework, vars.UnitTestCommand)
	return nil
}
