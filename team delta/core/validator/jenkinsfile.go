package validator

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// DevenvRequiredStages are declarative stages expected in generated LocalCiCd Jenkinsfiles.
var DevenvRequiredStages = []string{
	"Load platform config",
	"Checkout",
	"Detect app",
	"Linting",
	"Unit Tests",
	"Security Scan",
	"Docker Build",
	"Registry Push",
	"Kubernetes Deployment",
	"Rollout Verification",
	"Smoke Testing",
	"Rollback Handling",
}

var (
	stageNameRe     = regexp.MustCompile(`(?m)stage\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	goTemplateLeft  = regexp.MustCompile(`\{\{\s*\.`)
	pipelineBlockRe = regexp.MustCompile(`(?m)^\s*pipeline\s*\{`)
	stagesBlockRe   = regexp.MustCompile(`(?m)stages\s*\{`)
)

// ValidateFile runs syntax and structure checks on a Jenkinsfile path.
func ValidateFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read Jenkinsfile: %w", err)
	}
	return ValidateContent(string(data), path, true)
}

// ValidateContent validates Jenkins pipeline Groovy/declarative syntax and stage structure.
// When requireDevenvStages is true, missing template stages fail validation.
func ValidateContent(content, label string, requireDevenvStages bool) error {
	var errs []string

	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("%s: Jenkinsfile is empty", label)
	}

	if goTemplateLeft.MatchString(content) {
		errs = append(errs, "unrendered Go template syntax ({{ . }}) — re-run scaffolding")
	}

	if !pipelineBlockRe.MatchString(content) {
		errs = append(errs, "missing declarative pipeline { } block")
	}
	if !stagesBlockRe.MatchString(content) {
		errs = append(errs, "missing stages { } block")
	}
	if !strings.Contains(content, "agent") {
		errs = append(errs, "missing agent { } definition")
	}

	if err := checkDelimiterBalance(content, '{', '}'); err != nil {
		errs = append(errs, "brace balance: "+err.Error())
	}
	if err := checkDelimiterBalance(content, '(', ')'); err != nil {
		errs = append(errs, "parenthesis balance: "+err.Error())
	}
	if err := checkStringQuotes(content); err != nil {
		errs = append(errs, "string literal: "+err.Error())
	}

	stages := stageNameRe.FindAllStringSubmatch(content, -1)
	if len(stages) == 0 {
		errs = append(errs, "no stage('...') declarations found")
	} else {
		seen := make(map[string]int)
		for _, m := range stages {
			name := strings.TrimSpace(m[1])
			if name == "" {
				errs = append(errs, "empty stage name")
				continue
			}
			seen[name]++
		}
		if requireDevenvStages {
			for _, want := range DevenvRequiredStages {
				if seen[want] == 0 {
					errs = append(errs, fmt.Sprintf("missing required stage %q", want))
				}
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%s:\n  - %s", label, strings.Join(errs, "\n  - "))
}

func checkDelimiterBalance(content string, open, close rune) error {
	level := 0
	inStr := rune(0)
	escape := false
	lineComment := false

	for i, r := range content {
		if lineComment {
			if r == '\n' {
				lineComment = false
			}
			continue
		}
		if inStr != 0 {
			if escape {
				escape = false
				continue
			}
			if r == '\\' {
				escape = true
				continue
			}
			if r == inStr {
				inStr = 0
			}
			continue
		}
		if r == '/' && i+1 < len(content) {
			next := rune(content[i+1])
			if next == '/' {
				lineComment = true
				continue
			}
		}
		if r == '\'' || r == '"' {
			inStr = r
			continue
		}
		switch r {
		case open:
			level++
		case close:
			level--
			if level < 0 {
				return fmt.Errorf("unexpected %q at position %d", string(close), i)
			}
		}
	}
	if level != 0 {
		return fmt.Errorf("unclosed %q (depth %d)", string(open), level)
	}
	return nil
}

func checkStringQuotes(content string) error {
	// Groovy multiline ''' in agent yaml is allowed; basic check for odd unescaped " outside '''
	inTriple := false
	for i := 0; i < len(content); i++ {
		if i+2 < len(content) && content[i:i+3] == "'''" {
			inTriple = !inTriple
			i += 2
			continue
		}
		if inTriple {
			continue
		}
		if content[i] == '"' {
			// find closing quote on same line (simplified)
			rest := content[i+1:]
			if !strings.Contains(rest, "\"") && !strings.Contains(rest, "\n") {
				return fmt.Errorf("unclosed double-quoted string near offset %d", i)
			}
		}
	}
	return nil
}
