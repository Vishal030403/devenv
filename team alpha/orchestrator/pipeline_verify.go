package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"

	"devenv/teamalpha/internal/log"
	"devenv/teamalpha/internal/validation"
	"pipeline-cli/scaffolding_engine/core/generator"
	"pipeline-cli/scaffolding_engine/core/validator"
)

// PipelineTestSecurity runs security-oriented pipeline checks (Jenkinsfile + k8s manifests).
func PipelineTestSecurity() error {
	log.Info("Running pipeline test security")
	projectPath := targetProjectPath()

	if err := validation.ValidateProjectJenkinsfile(projectPath); err != nil {
		return err
	}

	if err := validateK8sManifests(projectPath); err != nil {
		return fmt.Errorf("kubernetes manifest validation: %w", err)
	}
	log.OK("Kubernetes manifest dry-run passed")

	if err := lintProjectStructure(projectPath); err != nil {
		return fmt.Errorf("project structure: %w", err)
	}
	log.OK("Project structure validation passed")

	log.Success("Pipeline test security completed")
	return nil
}

// PipelineTestAll runs full offline pipeline verification (security + templates + integrated job).
func PipelineTestAll() error {
	log.Info("Running pipeline test all")

	if err := PipelineTestSecurity(); err != nil {
		return err
	}

	if err := ValidateCommittedIntegratedJenkinsfile(); err != nil {
		return err
	}

	if err := validateFrameworkPipelineTemplates(); err != nil {
		return err
	}

	log.Success("Pipeline test all completed")
	return nil
}

func validateFrameworkPipelineTemplates() error {
	log.Info("Validating rendered Jenkinsfile templates per framework")
	frameworks := []string{
		"react", "expressjs", "django", "fastapi", "flask",
		"java_springboot", "golang", "ruby_on_rails",
	}
	for _, fw := range frameworks {
		spec := generator.ResolveFrameworkSpec(fw, ".", "")
		vars := spec.ToPipelineVars("sample-app")
		rendered, err := generator.RenderPipeline(vars)
		if err != nil {
			return fmt.Errorf("render %s: %w", fw, err)
		}
		label := fmt.Sprintf("framework:%s", fw)
		if err := validator.ValidateContent(rendered, label, true); err != nil {
			log.Failed(fmt.Sprintf("Template validation failed for %s", fw))
			return err
		}
		log.OK(fmt.Sprintf("Framework template valid: %s", fw))
	}
	return nil
}

// resolveIntegratedJenkinsfilePath finds Jenkinsfile.integrated for validation outside cluster.
func resolveIntegratedJenkinsfilePath() (string, error) {
	candidates := []string{
		filepath.Join("..", "team gamma", "pipelines", "Jenkinsfile.integrated"),
		filepath.Join("team gamma", "pipelines", "Jenkinsfile.integrated"),
	}
	wd, _ := os.Getwd()
	candidates = append(candidates, filepath.Join(wd, "..", "team gamma", "pipelines", "Jenkinsfile.integrated"))
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("Jenkinsfile.integrated not found")
}

// ValidateCommittedIntegratedJenkinsfile validates the checked-in integrated pipeline file.
func ValidateCommittedIntegratedJenkinsfile() error {
	path, err := resolveIntegratedJenkinsfilePath()
	if err != nil {
		return err
	}
	return validation.ValidateJenkinsPipelineSyntax(path, true)
}
