package validation

import (
	"fmt"
	"os"
	"path/filepath"

	"devenv/teamalpha/internal/log"
	"pipeline-cli/scaffolding_engine/core/generator"
	"pipeline-cli/scaffolding_engine/core/validator"
)

// ValidateJenkinsPipelineSyntax validates a Jenkinsfile before runtime or deployment.
func ValidateJenkinsPipelineSyntax(jenkinsfilePath string, requireDevenvStages bool) error {
	log.Info("Validating Jenkins pipeline syntax...")

	if _, err := os.Stat(jenkinsfilePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("Jenkinsfile not found: %s", jenkinsfilePath)
		}
		return err
	}

	data, err := os.ReadFile(jenkinsfilePath)
	if err != nil {
		return err
	}
	if err := validator.ValidateContent(string(data), jenkinsfilePath, requireDevenvStages); err != nil {
		log.Failed("Jenkinsfile validation failed")
		return err
	}

	log.Success("Jenkinsfile validation passed")
	return nil
}

// ValidateProjectJenkinsfile validates Jenkinsfile in the project root when present.
func ValidateProjectJenkinsfile(projectPath string) error {
	path := filepath.Join(projectPath, "Jenkinsfile")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Info("Validating Jenkins pipeline syntax...")
		log.Warn("No Jenkinsfile in project — skipping pipeline syntax validation")
		return nil
	}
	return ValidateJenkinsPipelineSyntax(path, true)
}

// ValidateIntegratedJenkinsfile validates the canonical in-cluster pipeline script.
func ValidateIntegratedJenkinsfile() error {
	rendered, err := generator.RenderDefaultIntegratedPipeline()
	if err != nil {
		return fmt.Errorf("render integrated pipeline: %w", err)
	}
	tmp, err := os.CreateTemp("", "devenv-integrated-jenkinsfile-*.groovy")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(rendered); err != nil {
		_ = tmp.Close()
		return err
	}
	_ = tmp.Close()
	return ValidateJenkinsPipelineSyntax(tmpPath, true)
}
