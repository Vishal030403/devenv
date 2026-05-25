package jenkinsclient

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type JenkinsCrumb struct {
	Crumb             string `json:"crumb"`
	CrumbRequestField string `json:"crumbRequestField"`
}

// TriggerIntegratedBuild starts devenv/local-ci-cd with git and app parameters.
// gitURL may be empty when source is uploaded via PVC (kubectl cp).
func TriggerIntegratedBuild(
	jenkinsBaseURL,
	gitURL,
	branch,
	appName string,
) (queueID int, err error) {

	gitURL = strings.TrimSpace(gitURL)

	if branch == "" {
		branch = "main"
	}

	base := strings.TrimSuffix(
		strings.TrimSpace(jenkinsBaseURL),
		"/",
	)

	jobURL := fmt.Sprintf(
		"%s/job/devenv/job/local-ci-cd/buildWithParameters",
		base,
	)

	form := url.Values{}
	if gitURL != "" {
		form.Set("GIT_URL", gitURL)
	}
	form.Set("GIT_BRANCH", branch)

	if strings.TrimSpace(appName) != "" {
		form.Set("APP_NAME", strings.TrimSpace(appName))
	}

	jar, _ := cookiejar.New(nil)

	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	crumb, err := fetchCrumb(base, client)
	if err != nil {
		return 0, fmt.Errorf("fetch crumb: %w", err)
	}

	req, err := http.NewRequest(
		http.MethodPost,
		jobURL,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return 0, err
	}

	req.SetBasicAuth("admin", "admin123")
	req.Header.Set(crumb.CrumbRequestField, crumb.Crumb)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 201 || resp.StatusCode == 200 || resp.StatusCode == 302 {
		// Extract queue item ID from Location header
		loc := resp.Header.Get("Location")
		qID := parseQueueID(loc)
		return qID, nil
	}

	body, _ := io.ReadAll(resp.Body)
	return 0, fmt.Errorf("jenkins trigger HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func fetchCrumb(base string, client *http.Client) (JenkinsCrumb, error) {
	crumbReq, err := http.NewRequest(http.MethodGet, base+"/crumbIssuer/api/json", nil)
	if err != nil {
		return JenkinsCrumb{}, err
	}
	crumbReq.SetBasicAuth("admin", "admin123")
	crumbResp, err := client.Do(crumbReq)
	if err != nil {
		return JenkinsCrumb{}, err
	}
	defer crumbResp.Body.Close()
	var crumb JenkinsCrumb
	if err := json.NewDecoder(crumbResp.Body).Decode(&crumb); err != nil {
		return JenkinsCrumb{}, err
	}
	return crumb, nil
}

func parseQueueID(locationHeader string) int {
	// Location: http://127.0.0.1:8080/queue/item/42/
	parts := strings.Split(strings.TrimRight(locationHeader, "/"), "/")
	if len(parts) == 0 {
		return 0
	}
	id, _ := strconv.Atoi(parts[len(parts)-1])
	return id
}

// TriggerAndFollowBuild triggers the Jenkins job and streams console output until completion.
// Returns nil on SUCCESS, error on FAILURE or timeout.
func TriggerAndFollowBuild(jenkinsBaseURL, gitURL, appName string) error {
	base := strings.TrimSuffix(strings.TrimSpace(jenkinsBaseURL), "/")

	fmt.Println("[INFO] Triggering Jenkins CI/CD pipeline...")
	queueID, err := TriggerIntegratedBuild(base, gitURL, "main", appName)
	if err != nil {
		return fmt.Errorf("trigger build: %w", err)
	}
	fmt.Printf("[OK] Build queued (queue item: %d)\n", queueID)

	// Wait for build to start (queue → running)
	buildNum, err := waitForQueuedBuild(base, queueID)
	if err != nil {
		return fmt.Errorf("waiting for build to start: %w", err)
	}
	fmt.Printf("[OK] Jenkins build #%d started\n", buildNum)
	fmt.Printf("[INFO] Console: %s/job/devenv/job/local-ci-cd/%d/console\n", base, buildNum)
	fmt.Println()
	fmt.Println("════════════════════════ Jenkins Console Output ════════════════════════")

	// Stream console output
	result, err := followBuildOutput(base, buildNum)
	if err != nil {
		return fmt.Errorf("follow build: %w", err)
	}

	fmt.Println("════════════════════════════════════════════════════════════════════════")

	if result != "SUCCESS" {
		return fmt.Errorf("jenkins build #%d finished with result: %s", buildNum, result)
	}

	fmt.Printf("[OK] Jenkins build #%d completed successfully\n", buildNum)
	return nil
}

// waitForQueuedBuild polls the queue item until the build actually starts.
func waitForQueuedBuild(base string, queueID int) (int, error) {
	if queueID <= 0 {
		// Fallback: poll lastBuild
		return waitForLastBuild(base)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	deadline := time.Now().Add(3 * time.Minute)

	for time.Now().Before(deadline) {
		url := fmt.Sprintf("%s/queue/item/%d/api/json", base, queueID)
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		req.SetBasicAuth("admin", "admin123")

		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		var result struct {
			Executable struct {
				Number int `json:"number"`
			} `json:"executable"`
			Cancelled bool   `json:"cancelled"`
			Why       string `json:"why"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		if result.Cancelled {
			return 0, fmt.Errorf("build was cancelled")
		}
		if result.Executable.Number > 0 {
			return result.Executable.Number, nil
		}
		if result.Why != "" {
			fmt.Printf("[INFO] Waiting: %s\n", result.Why)
		}
		time.Sleep(3 * time.Second)
	}

	return 0, fmt.Errorf("timeout waiting for queued build to start")
}

func waitForLastBuild(base string) (int, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	deadline := time.Now().Add(2 * time.Minute)

	for time.Now().Before(deadline) {
		url := fmt.Sprintf("%s/job/devenv/job/local-ci-cd/lastBuild/api/json", base)
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		req.SetBasicAuth("admin", "admin123")
		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		var result struct {
			Number   int  `json:"number"`
			Building bool `json:"building"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		if result.Number > 0 && result.Building {
			return result.Number, nil
		}
		time.Sleep(2 * time.Second)
	}
	return 0, fmt.Errorf("timeout waiting for build to appear")
}

// followBuildOutput streams the console output and returns the final build result.
func followBuildOutput(base string, buildNumber int) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	var offset int64
	deadline := time.Now().Add(45 * time.Minute)

	for time.Now().Before(deadline) {
		logURL := fmt.Sprintf("%s/job/devenv/job/local-ci-cd/%d/logText/progressiveText?start=%d",
			base, buildNumber, offset)
		req, _ := http.NewRequest(http.MethodGet, logURL, nil)
		req.SetBasicAuth("admin", "admin123")

		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if len(body) > 0 {
			lines := strings.Split(string(body), "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				// Skip Jenkins pipeline noise
				if trimmed == "" ||
					trimmed == "{" || trimmed == "}" ||
					strings.HasPrefix(trimmed, "[Pipeline]") ||
					strings.HasPrefix(trimmed, "[Checks API]") ||
					strings.Contains(trimmed, "(hide)") {
					continue
				}
				fmt.Println(line)
			}
		}

		// Update offset from X-Text-Size header
		if newSize := resp.Header.Get("X-Text-Size"); newSize != "" {
			if n, err := strconv.ParseInt(newSize, 10, 64); err == nil {
				offset = n
			}
		}

		// Check if build is still running via X-More-Data header
		if resp.Header.Get("X-More-Data") != "true" {
			// Build finished — get final result
			return getBuildResult(base, buildNumber)
		}

		time.Sleep(2 * time.Second)
	}

	return "", fmt.Errorf("timeout following build output")
}

func getBuildResult(base string, buildNumber int) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("%s/job/devenv/job/local-ci-cd/%d/api/json", base, buildNumber)
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.SetBasicAuth("admin", "admin123")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Result string `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Result == "" {
		return "UNKNOWN", nil
	}
	return result.Result, nil
}

// SyncJobFromJenkinsfile reads the project Jenkinsfile and creates/updates the Jenkins job.
func SyncJobFromJenkinsfile(jenkinsBaseURL, jenkinsfilePath, appName string) error {
	data, err := os.ReadFile(jenkinsfilePath)
	if err != nil {
		return fmt.Errorf("read Jenkinsfile: %w", err)
	}
	script := string(data)
	if strings.TrimSpace(script) == "" {
		return fmt.Errorf("Jenkinsfile is empty")
	}

	configXML := buildJobXML(script)
	base := strings.TrimSuffix(strings.TrimSpace(jenkinsBaseURL), "/")

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 30 * time.Second}

	// Fetch CSRF crumb (required for all POST requests)
	crumb, err := fetchCrumb(base, client)
	if err != nil {
		return fmt.Errorf("fetch crumb: %w", err)
	}

	// Ensure folder exists
	if err := ensureFolder(base, "devenv", client, crumb); err != nil {
		fmt.Printf("[WARN] Jenkins folder: %v\n", err)
	}

	// Try to update existing job first
	updateURL := fmt.Sprintf("%s/job/devenv/job/local-ci-cd/config.xml", base)
	updReq, _ := http.NewRequest(http.MethodPost, updateURL, strings.NewReader(configXML))
	updReq.SetBasicAuth("admin", "admin123")
	updReq.Header.Set("Content-Type", "application/xml")
	updReq.Header.Set(crumb.CrumbRequestField, crumb.Crumb)
	updResp, err := client.Do(updReq)
	if err == nil && updResp.StatusCode >= 200 && updResp.StatusCode < 300 {
		updResp.Body.Close()
		fmt.Println("[OK] Jenkins job updated with project Jenkinsfile")
		return nil
	}
	if updResp != nil {
		updResp.Body.Close()
	}

	// Create new job
	createURL := fmt.Sprintf("%s/job/devenv/createItem?name=local-ci-cd", base)
	createReq, _ := http.NewRequest(http.MethodPost, createURL, strings.NewReader(configXML))
	createReq.SetBasicAuth("admin", "admin123")
	createReq.Header.Set("Content-Type", "application/xml")
	createReq.Header.Set(crumb.CrumbRequestField, crumb.Crumb)
	createResp, err := client.Do(createReq)
	if err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	defer createResp.Body.Close()

	if createResp.StatusCode == 200 || createResp.StatusCode == 201 || createResp.StatusCode == 400 {
		fmt.Println("[OK] Jenkins job synced with project Jenkinsfile")
		return nil
	}

	b, _ := io.ReadAll(createResp.Body)
	return fmt.Errorf("sync job HTTP %d: %s", createResp.StatusCode, strings.TrimSpace(string(b)))
}

func ensureFolder(base, folder string, client *http.Client, crumb JenkinsCrumb) error {
	// Check if exists
	checkReq, _ := http.NewRequest(http.MethodGet, base+"/job/"+folder+"/api/json", nil)
	checkReq.SetBasicAuth("admin", "admin123")
	checkResp, err := client.Do(checkReq)
	if err == nil && checkResp.StatusCode == 200 {
		checkResp.Body.Close()
		return nil
	}
	if checkResp != nil {
		checkResp.Body.Close()
	}

	xml := `<com.cloudbees.hudson.plugins.folder.Folder plugin="cloudbees-folder"><name>` + folder + `</name></com.cloudbees.hudson.plugins.folder.Folder>`
	createReq, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/createItem?name=%s&mode=com.cloudbees.hudson.plugins.folder.Folder", base, folder),
		strings.NewReader(xml))
	createReq.SetBasicAuth("admin", "admin123")
	createReq.Header.Set("Content-Type", "application/xml")
	createReq.Header.Set(crumb.CrumbRequestField, crumb.Crumb)
	resp, err := client.Do(createReq)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func buildJobXML(script string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(script))
	return fmt.Sprintf(`<?xml version='1.1' encoding='UTF-8'?>
<flow-definition plugin="workflow-job">
  <actions/>
  <description>Integrated LocalCiCd pipeline (shared registry and cluster with devenv)</description>
  <keepDependencies>false</keepDependencies>
  <properties>
    <hudson.model.ParametersDefinitionProperty>
      <parameterDefinitions>
        <hudson.model.StringParameterDefinition>
          <name>GIT_URL</name>
          <description>Git repository URL (optional for local source)</description>
          <defaultValue></defaultValue>
          <trim>true</trim>
        </hudson.model.StringParameterDefinition>
        <hudson.model.StringParameterDefinition>
          <name>GIT_BRANCH</name>
          <defaultValue>main</defaultValue>
          <trim>true</trim>
        </hudson.model.StringParameterDefinition>
        <hudson.model.StringParameterDefinition>
          <name>APP_NAME</name>
          <description>Optional app name override</description>
          <defaultValue></defaultValue>
          <trim>true</trim>
        </hudson.model.StringParameterDefinition>
      </parameterDefinitions>
    </hudson.model.ParametersDefinitionProperty>
  </properties>
  <definition class="org.jenkinsci.plugins.workflow.cps.CpsFlowDefinition" plugin="workflow-cps">
    <script>%s</script>
    <sandbox>true</sandbox>
  </definition>
  <triggers/>
  <disabled>false</disabled>
</flow-definition>`, buf.String())
}

// EnsureJenkinsJob auto-creates Jenkins folder + pipeline using the real project Jenkinsfile.
func EnsureJenkinsJob(base string, jenkinsfilePath string) error {

	base = strings.TrimSuffix(strings.TrimSpace(base), "/")

	jar, _ := cookiejar.New(nil)

	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	// Fetch crumb
	crumb, err := fetchCrumb(base, client)
	if err != nil {
		return err
	}

	// Ensure folder exists
	if err := ensureFolder(base, "devenv", client, crumb); err != nil {
		fmt.Printf("[WARN] Jenkins folder: %v\n", err)
	}

	// Read the real Jenkinsfile directly
	var script string
	data, err := os.ReadFile(jenkinsfilePath)
	if err != nil || len(data) < 50 {
		fmt.Printf("[WARN] Could not read Jenkinsfile at %s: %v — using fallback\n", jenkinsfilePath, err)
		script = resolveJenkinsfileContent()
	} else {
		script = string(data)
		fmt.Printf("[OK] Loaded Jenkinsfile: %s (%d bytes)\n", jenkinsfilePath, len(data))
	}

	configXML := buildJobXML(script)

	// Check if job already exists
	checkJobReq, _ := http.NewRequest(http.MethodGet, base+"/job/devenv/job/local-ci-cd/api/json", nil)
	checkJobReq.SetBasicAuth("admin", "admin123")
	checkJobResp, err := client.Do(checkJobReq)

	if err == nil && checkJobResp.StatusCode == 200 {
		checkJobResp.Body.Close()
		// Job exists — update it with the real Jenkinsfile
		updateURL := fmt.Sprintf("%s/job/devenv/job/local-ci-cd/config.xml", base)
		updReq, _ := http.NewRequest(http.MethodPost, updateURL, strings.NewReader(configXML))
		updReq.SetBasicAuth("admin", "admin123")
		updReq.Header.Set("Content-Type", "application/xml")
		updReq.Header.Set(crumb.CrumbRequestField, crumb.Crumb)
		updResp, err := client.Do(updReq)
		if err == nil && updResp.StatusCode >= 200 && updResp.StatusCode < 300 {
			updResp.Body.Close()
			fmt.Println("[OK] Jenkins pipeline updated with real Jenkinsfile")
			return nil
		}
		if updResp != nil {
			updResp.Body.Close()
		}
		fmt.Println("[INFO] Jenkins pipeline already exists")
		return nil
	}
	if checkJobResp != nil {
		checkJobResp.Body.Close()
	}

	// Create pipeline
	jobURL := fmt.Sprintf("%s/job/devenv/createItem?name=local-ci-cd", base)
	jobReq, err := http.NewRequest(http.MethodPost, jobURL, strings.NewReader(configXML))
	if err != nil {
		return err
	}

	jobReq.SetBasicAuth("admin", "admin123")
	jobReq.Header.Set(crumb.CrumbRequestField, crumb.Crumb)
	jobReq.Header.Set("Content-Type", "application/xml")

	jobResp, err := client.Do(jobReq)
	if err != nil {
		return err
	}
	defer jobResp.Body.Close()

	if jobResp.StatusCode == 200 || jobResp.StatusCode == 201 || jobResp.StatusCode == 400 {
		fmt.Println("[OK] Jenkins pipeline ready")
		return nil
	}

	body, _ := io.ReadAll(jobResp.Body)
	return fmt.Errorf("jenkins job creation failed: %s", string(body))
}

// resolveJenkinsfileContent finds and reads the real project Jenkinsfile.
func resolveJenkinsfileContent() string {
	// Try project Jenkinsfile first
	projectPath := strings.TrimSpace(os.Getenv("DEVENV_PROJECT_PATH"))
	if projectPath != "" {
		jf := projectPath + "/Jenkinsfile"
		if data, err := os.ReadFile(jf); err == nil && len(data) > 100 {
			fmt.Printf("[OK] Using project Jenkinsfile: %s\n", jf)
			return string(data)
		}
	}

	// Try sample apps
	candidates := []string{
		"sample apps/react-demo/Jenkinsfile",
		"../sample apps/react-demo/Jenkinsfile",
	}
	wd, _ := os.Getwd()
	if wd != "" {
		candidates = append(candidates, wd+"/sample apps/react-demo/Jenkinsfile")
		candidates = append(candidates, wd+"/../sample apps/react-demo/Jenkinsfile")
	}
	for _, c := range candidates {
		if data, err := os.ReadFile(c); err == nil && len(data) > 100 {
			fmt.Printf("[OK] Using Jenkinsfile: %s\n", c)
			return string(data)
		}
	}

	// Try Jenkinsfile.integrated from team gamma
	gammaCandidates := []string{
		"team gamma/pipelines/Jenkinsfile.integrated",
		"../team gamma/pipelines/Jenkinsfile.integrated",
	}
	if wd != "" {
		gammaCandidates = append(gammaCandidates, wd+"/../team gamma/pipelines/Jenkinsfile.integrated")
	}
	for _, c := range gammaCandidates {
		if data, err := os.ReadFile(c); err == nil && len(data) > 100 {
			fmt.Printf("[OK] Using Jenkinsfile.integrated: %s\n", c)
			return string(data)
		}
	}

	// Fallback — should not happen, but better than empty
	fmt.Println("[WARN] No project Jenkinsfile found — using minimal pipeline")
	return `pipeline {
    agent any
    stages {
        stage('Build') { steps { sh 'echo Build successful' } }
        stage('Deploy') { steps { sh 'echo Deploy successful' } }
    }
}`
}

// GitRemoteOrigin returns origin URL for a project directory, if configured.
func GitRemoteOrigin(projectPath string) (string, error) {

	out, err := exec.Command(
		"git",
		"-C",
		projectPath,
		"remote",
		"get-url",
		"origin",
	).Output()

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

// JenkinsURLFromState reads Jenkins URL from platform configmap (cluster) or defaults.
func JenkinsURLFromState() string {

	out, err := exec.Command(
		"kubectl",
		"get",
		"configmap",
		"devenv-platform-config",
		"-n",
		"devenv-system",
		"-o",
		"jsonpath={.data.platform\\.json}",
	).Output()

	if err != nil {
		return "http://127.0.0.1:8080"
	}

	var cfg struct {
		JenkinsURL string `json:"jenkins_url"`
	}

	if json.Unmarshal(out, &cfg) == nil &&
		cfg.JenkinsURL != "" {

		return cfg.JenkinsURL
	}

	return "http://127.0.0.1:8080"
}