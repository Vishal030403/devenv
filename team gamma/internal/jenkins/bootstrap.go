package jenkins

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"devenv-gamma/pkg/integration"
	"devenv-gamma/pkg/state"

	"pipeline-cli/scaffolding_engine/core/generator"
	"pipeline-cli/scaffolding_engine/core/validator"
)

const (
	integratedJobName   = "local-ci-cd"
	integratedJobFolder = "devenv"
	pipelineConfigMap   = "jenkins-devenv-pipeline"
)

// BootstrapIntegrated seeds Jenkins with the shared devenv pipeline job and RBAC to read platform config.
func BootstrapIntegrated(projectPath, clusterName, defaultApp string) error {
	fmt.Println("[INFO] Bootstrapping integrated Jenkins pipeline (shared with devenv)...")

	cfg, err := integration.LoadPlatformConfigFromState(projectPath, clusterName, defaultApp)
	if err != nil {
		return fmt.Errorf("load platform config: %w", err)
	}
	if err := integration.SyncPlatformConfigToCluster(cfg); err != nil {
		return fmt.Errorf("sync platform configmap: %w", err)
	}
	fmt.Println("[OK] Platform ConfigMap synced (devenv-system/devenv-platform-config)")

	if err := applyJenkinsRBAC(); err != nil {
		fmt.Printf("[WARN] Jenkins RBAC apply: %v\n", err)
	}

	if err := applyPipelineConfigMap(); err != nil {
		return err
	}

	if err := ensureJenkinsPortForwardReady(); err != nil {
		return fmt.Errorf("jenkins not reachable for job seed: %w", err)
	}

	if err := seedIntegratedJob(cfg); err != nil {
		return err
	}

	_ = state.UpdateJenkinsState(func(js *state.JenkinsState) {
		js.Enabled = true
	})

	jobURL := fmt.Sprintf("%s/job/%s/job/%s/", cfg.JenkinsURL, integratedJobFolder, integratedJobName)
	fmt.Printf("[OK] Jenkins integrated job: %s\n", jobURL)
	fmt.Println("[INFO] Build with parameter GIT_URL (required). Local uncommitted edits: use devenv run")
	return nil
}

func ensureJenkinsPortForwardReady() error {
	if err := EnsureJenkinsPortForward(); err != nil {
		return err
	}
	url := jenkinsUIURL(jenkinsLocalUIPort, "/login")
	return validateJenkinsUILoginURL(url, 15*time.Second)
}

func applyPipelineConfigMap() error {
	scriptPath, cleanup, err := resolvePipelineScriptPath()
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		return fmt.Errorf("read pipeline script: %w", err)
	}
	if err := validator.ValidateContent(string(data), scriptPath, true); err != nil {
		return fmt.Errorf("Jenkins pipeline validation failed: %w", err)
	}
	fmt.Println("[OK] Jenkins pipeline syntax validated")

	cmd := exec.Command("kubectl", "create", "configmap", pipelineConfigMap,
		"-n", jenkinsNamespace,
		fmt.Sprintf("--from-file=Jenkinsfile.integrated=%s", scriptPath),
		"--dry-run=client", "-o", "yaml")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("generate pipeline configmap: %w", err)
	}
	apply := exec.Command("kubectl", "apply", "-f", "-")
	apply.Stdin = bytes.NewReader(out)
	applyOut, err := apply.CombinedOutput()
	if err != nil {
		return fmt.Errorf("apply pipeline configmap: %w\n%s", err, strings.TrimSpace(string(applyOut)))
	}
	fmt.Println("[OK] Jenkins pipeline script ConfigMap applied")
	return nil
}

// resolvePipelineScriptPath returns a Jenkins pipeline script path for the integrated job.
// Priority: scaffolded project Jenkinsfile → committed Jenkinsfile.integrated → rendered template.
func resolvePipelineScriptPath() (path string, cleanup func(), err error) {
	if p := strings.TrimSpace(os.Getenv("DEVENV_PROJECT_PATH")); p != "" {
		jf := filepath.Join(p, "Jenkinsfile")
		if st, e := os.Stat(jf); e == nil && !st.IsDir() {
			abs, _ := filepath.Abs(jf)
			fmt.Printf("[OK] Using project Jenkinsfile: %s\n", abs)
			return abs, nil, nil
		}
	}

	if integrated, e := locateIntegratedJenkinsfile(); e == nil {
		return integrated, nil, nil
	}

	rendered, e := generator.RenderDefaultIntegratedPipeline()
	if e != nil {
		return "", nil, fmt.Errorf("render pipeline template: %w", e)
	}
	tmp, e := os.CreateTemp("", "devenv-jenkinsfile-*.groovy")
	if e != nil {
		return "", nil, e
	}
	if _, e := tmp.WriteString(rendered); e != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", nil, e
	}
	_ = tmp.Close()
	fmt.Println("[OK] Using rendered pipeline template (Team Delta pipeline/Jenkinsfile.tmpl)")
	return tmp.Name(), func() { _ = os.Remove(tmp.Name()) }, nil
}

func locateIntegratedJenkinsfile() (string, error) {
	candidates := []string{
		filepath.Join("pipelines", "Jenkinsfile.integrated"),
		filepath.Join("..", "team gamma", "pipelines", "Jenkinsfile.integrated"),
		filepath.Join("team gamma", "pipelines", "Jenkinsfile.integrated"),
	}
	wd, _ := os.Getwd()
	candidates = append(candidates, filepath.Join(wd, "pipelines", "Jenkinsfile.integrated"))

	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs, nil
		}
	}
	return "", fmt.Errorf("Jenkinsfile.integrated not found")
}

func applyJenkinsRBAC() error {
	manifest := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: devenv-jenkins-integrator
rules:
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch", "update", "patch"]
  - apiGroups: [""]
    resources: ["pods", "services", "namespaces"]
    verbs: ["get", "list", "watch", "create", "update", "patch"]
  - apiGroups: ["networking.k8s.io"]
    resources: ["ingresses"]
    verbs: ["get", "list", "watch", "create", "update", "patch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: devenv-jenkins-integrator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: devenv-jenkins-integrator
subjects:
  - kind: ServiceAccount
    name: default
    namespace: jenkins
`
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	fmt.Println("[OK] Jenkins integrator RBAC applied")
	return nil
}

func seedIntegratedJob(cfg integration.PlatformConfig) error {
	script, err := readPipelineScriptFromCluster()
	if err != nil {
		scriptPath, cleanup, err2 := resolvePipelineScriptPath()
		if err2 != nil {
			return err
		}
		if cleanup != nil {
			defer cleanup()
		}
		b, err3 := os.ReadFile(scriptPath)
		if err3 != nil {
			return err3
		}
		script = string(b)
	}

	if err := createFolder(integratedJobFolder, cfg); err != nil {
		fmt.Printf("[WARN] create folder: %v\n", err)
	}
	return createOrUpdatePipelineJob(integratedJobFolder, integratedJobName, script, cfg)
}

func readPipelineScriptFromCluster() (string, error) {
	out, err := exec.Command("kubectl", "get", "configmap", pipelineConfigMap,
		"-n", jenkinsNamespace,
		"-o", "jsonpath={.data.Jenkinsfile\\.integrated}").Output()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(out)) == "" {
		return "", fmt.Errorf("empty pipeline script in configmap")
	}
	return string(out), nil
}

func createFolder(folder string, cfg integration.PlatformConfig) error {
	body := fmt.Sprintf(`<com.cloudbees.hudson.plugins.folder.Folder plugin="cloudbees-folder"><name>%s</name></com.cloudbees.hudson.plugins.folder.Folder>`, xmlEscape(folder))
	url := fmt.Sprintf("%s/createItem?name=%s&mode=com.cloudbees.hudson.plugins.folder.Folder", strings.TrimSuffix(cfg.JenkinsURL, "/"), folder)
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.SetBasicAuth("admin", "admin123")
	req.Header.Set("Content-Type", "application/xml")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 || resp.StatusCode == 201 || resp.StatusCode == 400 {
		return nil
	}
	b, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
}

func createOrUpdatePipelineJob(folder, name, script string, cfg integration.PlatformConfig) error {
	configXML := buildPipelineJobXML(script)
	itemPath := name
	if folder != "" {
		itemPath = folder + "/job/" + name
	}
	base := strings.TrimSuffix(cfg.JenkinsURL, "/")
	url := fmt.Sprintf("%s/createItem?name=%s", base, name)
	if folder != "" {
		url = fmt.Sprintf("%s/job/%s/createItem?name=%s", base, folder, name)
	}

	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(configXML))
	if err != nil {
		return err
	}
	req.SetBasicAuth("admin", "admin123")
	req.Header.Set("Content-Type", "application/xml")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		fmt.Printf("[OK] Created Jenkins job %s\n", itemPath)
		return nil
	}
	if resp.StatusCode == 400 && strings.Contains(readBody(resp), "Already exists") {
		return updatePipelineJob(folder, name, configXML, cfg)
	}
	if resp.StatusCode == 400 {
		return updatePipelineJob(folder, name, configXML, cfg)
	}
	b := readBody(resp)
	return fmt.Errorf("create job HTTP %d: %s", resp.StatusCode, b)
}

func updatePipelineJob(folder, name, configXML string, cfg integration.PlatformConfig) error {
	base := strings.TrimSuffix(cfg.JenkinsURL, "/")
	url := fmt.Sprintf("%s/job/%s/config.xml", base, name)
	if folder != "" {
		url = fmt.Sprintf("%s/job/%s/job/%s/config.xml", base, folder, name)
	}
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(configXML))
	if err != nil {
		return err
	}
	req.SetBasicAuth("admin", "admin123")
	req.Header.Set("Content-Type", "application/xml")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Printf("[OK] Updated Jenkins job %s/%s\n", folder, name)
		return nil
	}
	return fmt.Errorf("update job HTTP %d: %s", resp.StatusCode, readBody(resp))
}

func readBody(resp *http.Response) string {
	b, _ := io.ReadAll(resp.Body)
	return strings.TrimSpace(string(b))
}

func buildPipelineJobXML(script string) string {
	return fmt.Sprintf(`<?xml version='1.1' encoding='UTF-8'?>
<flow-definition plugin="workflow-job@2.40">
  <actions/>
  <description>Integrated LocalCiCd pipeline (shared registry and cluster with devenv)</description>
  <keepDependencies>false</keepDependencies>
  <properties>
    <hudson.model.ParametersDefinitionProperty>
      <parameterDefinitions>
        <hudson.model.StringParameterDefinition>
          <name>GIT_URL</name>
          <description>Git repository URL (required for Jenkins builds)</description>
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
  <definition class="org.jenkinsci.plugins.workflow.cps.CpsFlowDefinition" plugin="workflow-cps@2.90">
    <script>%s</script>
    <sandbox>true</sandbox>
  </definition>
  <triggers/>
  <disabled>false</disabled>
</flow-definition>`, xmlEscape(script))
}

func xmlEscape(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}
