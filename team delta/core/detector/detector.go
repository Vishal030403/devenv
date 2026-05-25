package detector

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var ignoreDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"venv":         true,
	".venv":        true,
	"env":          true,
	".env":         true,
	"__pycache__":  true,
	".cache":       true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
}

func findFileDeep(projectPath string, targetFileName string) (string, bool) {
	foundPath := ""
	found := false
	filepath.WalkDir(projectPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil { return nil }
		if d.IsDir() {
			if ignoreDirs[d.Name()] { return filepath.SkipDir }
			return nil
		}
		if d.Name() == targetFileName {
			foundPath = path
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return foundPath, found
}

func fileContains(path string, substr string) bool {
	content, err := os.ReadFile(path)
	if err != nil { return false }
	return strings.Contains(strings.ToLower(string(content)), strings.ToLower(substr))
}

// DetectFramework inspects signal files and returns (frameworkName, entryPath)
func DetectFramework(projectPath string) (string, string) {

	// 1. Java
	if _, ok := findFileDeep(projectPath, "pom.xml"); ok { return "java_springboot", "" }
	if _, ok := findFileDeep(projectPath, "build.gradle"); ok { return "java_springboot", "" }

	// 2. Ruby on Rails — Gemfile containing "rails"
	if gemfilePath, ok := findFileDeep(projectPath, "Gemfile"); ok {
		if fileContains(gemfilePath, "rails") { return "ruby_on_rails", "" }
	}

	// 3. Go — go.mod present
	if _, ok := findFileDeep(projectPath, "go.mod"); ok { return "golang", "" }

	// 4. Python frameworks
	if reqPath, ok := findFileDeep(projectPath, "requirements.txt"); ok {
		if fileContains(reqPath, "django") {
			if pyPath, found := findFileDeep(projectPath, "manage.py"); found {
				relPath, _ := filepath.Rel(projectPath, pyPath)
				return "django", relPath
			}
			return "django", "manage.py"
		}
		if fileContains(reqPath, "fastapi") {
			if pyPath, found := findFileDeep(projectPath, "main.py"); found {
				relPath, _ := filepath.Rel(projectPath, pyPath)
				module := strings.ReplaceAll(strings.TrimSuffix(relPath, ".py"), string(filepath.Separator), ".")
				return "fastapi", module
			}
			if pyPath, found := findFileDeep(projectPath, "app.py"); found {
				relPath, _ := filepath.Rel(projectPath, pyPath)
				module := strings.ReplaceAll(strings.TrimSuffix(relPath, ".py"), string(filepath.Separator), ".")
				return "fastapi", module
			}
			return "fastapi", "main"
		}
		// Flask — after fastapi/django checks
		if fileContains(reqPath, "flask") {
			if pyPath, found := findFileDeep(projectPath, "app.py"); found {
				relPath, _ := filepath.Rel(projectPath, pyPath)
				module := strings.ReplaceAll(strings.TrimSuffix(relPath, ".py"), string(filepath.Separator), ".")
				return "flask", module
			}
			if pyPath, found := findFileDeep(projectPath, "main.py"); found {
				relPath, _ := filepath.Rel(projectPath, pyPath)
				module := strings.ReplaceAll(strings.TrimSuffix(relPath, ".py"), string(filepath.Separator), ".")
				return "flask", module
			}
			return "flask", "app"
		}
	}

	// Django fallback (no requirements.txt)
	if pyPath, ok := findFileDeep(projectPath, "manage.py"); ok {
		relPath, _ := filepath.Rel(projectPath, pyPath)
		return "django", relPath
	}

	// 5. JavaScript
	if packageJsonPath, ok := findFileDeep(projectPath, "package.json"); ok {
		if fileContains(packageJsonPath, `"react"`) || fileContains(packageJsonPath, `"react-scripts"`) {
			return "react", ""
		}
		if fileContains(packageJsonPath, `"express"`) { return "expressjs", "" }
	}

	return "unknown", ""
}
