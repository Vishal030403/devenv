package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FrameworkSpec holds scaffold + pipeline defaults for one detected framework.
type FrameworkSpec struct {
	Framework       string
	AppPort         int
	HealthPath      string
	K8sOverlay      string
	NamespaceSuffix string
	LintCommand     string
	UnitTestCommand string
	RunCommand      string
	PythonVersion   string
	NodeVersion     string
	JavaVersion     string
	GoVersion       string
	RubyVersion     string
}

// SanitizeAppName derives a DNS-safe application name from a project directory.
func SanitizeAppName(projectPath string) string {
	rawName := filepath.Base(projectPath)
	re := regexp.MustCompile(`[^a-z0-9-]`)
	appName := strings.ToLower(rawName)
	appName = strings.ReplaceAll(appName, "_", "-")
	appName = strings.ReplaceAll(appName, " ", "-")
	appName = re.ReplaceAllString(appName, "")
	return strings.Trim(appName, "-")
}

// ResolveFrameworkSpec returns merged k8s + pipeline defaults, with project-aware lint/test commands.
func ResolveFrameworkSpec(framework, projectPath, entryPath string) FrameworkSpec {
	spec := baseFrameworkSpec(framework, entryPath)
	spec.applyProjectProbes(projectPath, framework)
	return spec
}

// ToTemplateVars converts a spec into the map used by k8s/Dockerfile templates.
func (s FrameworkSpec) ToTemplateVars(appName string) map[string]interface{} {
	vars := map[string]interface{}{
		"app_name":         appName,
		"health_path":      s.HealthPath,
		"app_port":         s.AppPort,
		"k8s_overlay":      s.K8sOverlay,
		"k8s_namespace":    appName + s.NamespaceSuffix,
		"framework":        s.Framework,
		"lint_command":     s.LintCommand,
		"unit_test_command": s.UnitTestCommand,
	}
	if s.RunCommand != "" {
		vars["run_command"] = s.RunCommand
	}
	if s.PythonVersion != "" {
		vars["python_version"] = s.PythonVersion
	}
	if s.NodeVersion != "" {
		vars["node_version"] = s.NodeVersion
	}
	if s.JavaVersion != "" {
		vars["java_version"] = s.JavaVersion
	}
	if s.GoVersion != "" {
		vars["go_version"] = s.GoVersion
	}
	if s.RubyVersion != "" {
		vars["ruby_version"] = s.RubyVersion
	}
	return vars
}

// ToPipelineVars converts a spec into Jenkins pipeline template variables.
func (s FrameworkSpec) ToPipelineVars(appName string) PipelineVars {
	return PipelineVars{
		Framework:       s.Framework,
		AppName:           appName,
		HealthPath:        s.HealthPath,
		AppPort:           s.AppPort,
		K8sOverlay:        s.K8sOverlay,
		K8sNamespace:      appName + s.NamespaceSuffix,
		NamespaceSuffix:   s.NamespaceSuffix,
		DockerImageName:   appName,
		DeploymentName:    appName,
		LintCommand:       s.LintCommand,
		UnitTestCommand:   s.UnitTestCommand,
	}
}

func baseFrameworkSpec(framework, entryPath string) FrameworkSpec {
	s := FrameworkSpec{
		Framework:       framework,
		HealthPath:      "/",
		AppPort:         80,
		K8sOverlay:      "k8s/overlays/local",
		NamespaceSuffix: "-ns",
	}
	switch framework {
	case "django":
		s.AppPort = 8000
		s.PythonVersion = "3.12"
		s.RunCommand = `["python", "` + entryPath + `", "runserver", "0.0.0.0:8000"]`
		s.LintCommand = "python -m compileall -q ."
		s.UnitTestCommand = "python manage.py test --parallel 1"
	case "fastapi":
		s.AppPort = 8000
		s.HealthPath = "/docs"
		s.PythonVersion = "3.12"
		s.RunCommand = `["uvicorn", "` + entryPath + `:app", "--host", "0.0.0.0", "--port", "8000"]`
		s.LintCommand = "python -m compileall -q ."
		s.UnitTestCommand = "pytest -q"
	case "flask":
		s.AppPort = 5000
		s.PythonVersion = "3.12"
		s.RunCommand = `["python", "-m", "flask", "--app", "` + entryPath + `", "run", "--host=0.0.0.0", "--port=5000"]`
		s.LintCommand = "python -m compileall -q ."
		s.UnitTestCommand = "pytest -q"
	case "expressjs":
		s.AppPort = 3000
		s.NodeVersion = "18"
		s.HealthPath = "/"
		s.RunCommand = `["npm", "start"]`
		s.LintCommand = "npm run lint --if-present"
		s.UnitTestCommand = "npm test"
	case "react":
		s.AppPort = 80
		s.NodeVersion = "20"
		s.RunCommand = `["nginx", "-g", "daemon off;"]`
		s.LintCommand = "npm run lint --if-present"
		s.UnitTestCommand = "npm test"
	case "java_springboot":
		s.AppPort = 8080
		s.HealthPath = "/actuator/health"
		s.JavaVersion = "17"
		s.RunCommand = `["sh", "-c", "java -jar target/*.jar"]`
		s.LintCommand = "mvn -q -DskipTests validate"
		s.UnitTestCommand = "mvn -q test"
	case "golang":
		s.AppPort = 8080
		s.HealthPath = "/health"
		s.GoVersion = "1.22"
		s.RunCommand = `["./server"]`
		s.LintCommand = "go vet ./..."
		s.UnitTestCommand = "go test ./..."
	case "ruby_on_rails":
		s.AppPort = 3000
		s.HealthPath = "/up"
		s.RubyVersion = "3.2"
		s.RunCommand = `["bundle", "exec", "rails", "server", "-b", "0.0.0.0"]`
		s.LintCommand = "bundle exec rubocop"
		s.UnitTestCommand = "bundle exec rails test"
	default:
		s.Framework = "generic"
	}
	return s
}

func (s *FrameworkSpec) applyProjectProbes(projectPath, framework string) {
	switch framework {
	case "react", "expressjs":
		s.applyNodeScripts(projectPath)
	case "java_springboot":
		s.applyJavaMavenWrapper(projectPath)
	case "django", "fastapi", "flask":
		s.applyPythonTestRunner(projectPath, framework)
	case "golang":
		// go test ./... is already correct
	case "ruby_on_rails":
		if !fileExists(projectPath, "Gemfile") {
			return
		}
		if !fileExists(projectPath, "bin/rails") {
			s.UnitTestCommand = "bundle exec rake test"
		}
	}
}

func (s *FrameworkSpec) applyNodeScripts(projectPath string) {
	scripts := readPackageScripts(projectPath)
	if scripts == nil {
		return
	}
	if _, ok := scripts["lint"]; ok {
		s.LintCommand = "npm run lint"
	}
	if _, ok := scripts["test"]; ok {
		s.UnitTestCommand = "CI=true npm test -- --passWithNoTests"
	} else {
		s.UnitTestCommand = ""
	}
}

func (s *FrameworkSpec) applyJavaMavenWrapper(projectPath string) {
	if fileExists(projectPath, "mvnw") {
		s.LintCommand = "./mvnw -q -DskipTests validate"
		s.UnitTestCommand = "./mvnw -q test"
		return
	}
	// Windows checkout may only have mvnw.cmd; Jenkins agents run Linux.
	if fileExists(projectPath, "mvnw.cmd") && !fileExists(projectPath, "mvnw") {
		s.LintCommand = "mvn -q -DskipTests validate"
		s.UnitTestCommand = "mvn -q test"
	}
}

func (s *FrameworkSpec) applyPythonTestRunner(projectPath, framework string) {
	if hasPytestLayout(projectPath) {
		s.UnitTestCommand = "pytest -q"
		return
	}
	if framework == "django" && fileExists(projectPath, "manage.py") {
		s.UnitTestCommand = "python manage.py test --parallel 1"
	}
}

func fileExists(projectPath, name string) bool {
	_, err := os.Stat(filepath.Join(projectPath, name))
	return err == nil
}

func hasPytestLayout(projectPath string) bool {
	for _, f := range []string{"pytest.ini", "conftest.py", "pyproject.toml", "setup.cfg"} {
		if !fileExists(projectPath, f) {
			continue
		}
		if f == "pyproject.toml" || f == "setup.cfg" {
			content, err := os.ReadFile(filepath.Join(projectPath, f))
			if err == nil && strings.Contains(string(content), "pytest") {
				return true
			}
			continue
		}
		return true
	}
	return false
}

type packageJSON struct {
	Scripts map[string]string `json:"scripts"`
}

func readPackageScripts(projectPath string) map[string]string {
	data, err := os.ReadFile(filepath.Join(projectPath, "package.json"))
	if err != nil {
		return nil
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}
	return pkg.Scripts
}
