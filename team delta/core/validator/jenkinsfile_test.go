package validator

import (
	"strings"
	"testing"
)

func TestValidateContent_validPipeline(t *testing.T) {
	content := `pipeline {
  agent any
  stages {
    stage('Load platform config') { steps { echo 'ok' } }
    stage('Docker Build') { steps { sh 'true' } }
  }
}`
	if err := ValidateContent(content, "test", false); err != nil {
		t.Fatalf("expected valid pipeline: %v", err)
	}
}

func TestValidateContent_unrenderedTemplate(t *testing.T) {
	content := `pipeline { stages { stage('X') { steps { echo '{{ .AppName }}' } } } }`
	if err := ValidateContent(content, "test", false); err == nil {
		t.Fatal("expected error for Go template leftovers")
	}
}

func TestValidateContent_unbalancedBraces(t *testing.T) {
	content := `pipeline { stages { stage('A') { steps { echo 'hi' } }`
	if err := ValidateContent(content, "test", false); err == nil {
		t.Fatal("expected brace balance error")
	}
}

func TestValidateContent_missingStages(t *testing.T) {
	content := `pipeline { agent any stages { stage('Only') { steps { } } } }`
	err := ValidateContent(content, "test", true)
	if err == nil {
		t.Fatal("expected missing stage errors")
	}
	if !strings.Contains(err.Error(), "Docker Build") {
		t.Fatalf("expected missing stage name in error: %v", err)
	}
}
