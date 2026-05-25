package generator

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"pipeline-cli/scaffolding_engine/templates"
)

// GenerateFiles generates the scaffolding files based on the framework
func GenerateFiles(framework string, projectPath string, entryPath string) error {
	// Derive app name from directory, then sanitize it to be DNS-compatible.
	// Kubernetes resource names must be lowercase alphanumeric + hyphens only.
	rawName := filepath.Base(projectPath)
	re := regexp.MustCompile(`[^a-z0-9-]`)
	appName := strings.ToLower(rawName)
	appName = strings.ReplaceAll(appName, "_", "-")
	appName = strings.ReplaceAll(appName, " ", "-")
	appName = re.ReplaceAllString(appName, "")
	appName = strings.Trim(appName, "-")

	// tmplVars holds all values that templates can reference with {{ .key }}.
	tmplVars := map[string]interface{}{
		"app_name": appName,
	}

	// Dynamic Logic Injection!
	defaults := map[string]map[string]interface{}{
		"django": {
			"app_port":       8000,
			"python_version": "3.12",
			"run_command":    fmt.Sprintf(`["python", "%s", "runserver", "0.0.0.0:8000"]`, entryPath),
			"health_path":    "/", 
			"test_command":   fmt.Sprintf(`python %s test`, entryPath),
		},
		"fastapi": {
			"app_port":       8000,
			"python_version": "3.12",
			"run_command":    fmt.Sprintf(`["uvicorn", "%s:app", "--host", "0.0.0.0", "--port", "8000"]`, entryPath),
			"health_path":    "/docs", 
			"test_command":   `pytest`,
		},
		"expressjs": {
			"app_port":     3000,
			"node_version": "22",
			"run_command":  `["npm", "start"]`,
			"health_path":  "/",
			"test_command": `npm run test`,
		},
		"react": {
			"app_port":     8080,
			"node_version": "22",
			"run_command":  `["nginx", "-g", "daemon off;"]`,
			"health_path":  "/",
			"test_command": `npm run test`,
		},
		"java_springboot": {
			"app_port":     8080,
			"java_version": "17",
			"run_command":  `["sh", "-c", "java -jar target/*.jar"]`,
			"health_path":  "/actuator/health", 
			"test_command": `./mvnw test`,
		},
	}

	if defs, ok := defaults[framework]; ok {
		for k, v := range defs {
			tmplVars[k] = v
		}
	}

	// 1. Define the directories to walk (Shared templates FIRST, then framework-specific)
	dirsToWalk := []string{"shared", framework}

	// 2. Loop through both directories
	for _, dir := range dirsToWalk {
		err := fs.WalkDir(templates.Files, dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// If the directory doesn't exist in embed, return the error
				return fmt.Errorf("templates directory not found for: %s", dir)
			}

			if d.IsDir() {
				return nil
			}

			if strings.HasSuffix(d.Name(), ".tmpl") {
				// 3. Dynamically trim the prefix based on which directory we are currently walking
				relPath := strings.TrimPrefix(path, dir+"/")
				
				outputRelPath := strings.TrimSuffix(relPath, ".tmpl")
				outputPath := filepath.Join(projectPath, outputRelPath)

				err = os.MkdirAll(filepath.Dir(outputPath), os.ModePerm)
				if err != nil {
					return err
				}

				// Read file content from the embedded filesystem
				fileData, err := templates.Files.ReadFile(path)
				if err != nil {
					return err
				}

				if _, err := os.Stat(outputPath); err == nil {
					fmt.Printf("⚠️  Skipping existing file (already customized): %s\n", outputRelPath)
					return nil
				}

				tmpl, err := template.New(filepath.Base(path)).Parse(string(fileData))
				if err != nil {
					return err
				}

				var buf bytes.Buffer
				err = tmpl.Execute(&buf, tmplVars)
				if err != nil {
					return fmt.Errorf("error executing template %s: %w", path, err)
				}

				err = os.WriteFile(outputPath, buf.Bytes(), 0644)
				if err != nil {
					return err
				}

				fmt.Println("Generated", outputPath)
			}

			return nil
		})

		// Catch any errors from walking the specific directory before continuing
		if err != nil {
			return err
		}
	}

	return nil
}