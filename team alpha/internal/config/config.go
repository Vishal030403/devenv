package config

import (
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	ClusterName string `mapstructure:"clusterName"`
	Namespace   string `mapstructure:"namespace"`

	Timeouts struct {
		Rollout       int `mapstructure:"rollout"`
		Ingress       int `mapstructure:"ingress"`
		SmokeTest     int `mapstructure:"smokeTest"`
		Cleanup       int `mapstructure:"cleanup"`
		NodeReady     int `mapstructure:"nodeReady"`
		ClusterCreate int `mapstructure:"clusterCreate"`
	} `mapstructure:"timeouts"`

	Retries struct {
		Deployment int `mapstructure:"deployment"`
		Ingress    int `mapstructure:"ingress"`
		Cleanup    int `mapstructure:"cleanup"`
	} `mapstructure:"retries"`

	Ports struct {
		Jenkins  int `mapstructure:"jenkins"`
		Registry int `mapstructure:"registry"`
	} `mapstructure:"ports"`

	URLs struct {
		JenkinsURL  string `mapstructure:"jenkinsURL"`
		RegistryURL string `mapstructure:"registryURL"`
	} `mapstructure:"urls"`

	Logging struct {
		Verbose bool `mapstructure:"verbose"`
	} `mapstructure:"logging"`
}

var AppConfig Config

func TargetNamespace() string {

	if AppConfig.Namespace != "" {
		return AppConfig.Namespace
	}

	return "dev-apps"
}

func TargetClusterName() string {

	if AppConfig.ClusterName != "" {
		return AppConfig.ClusterName
	}

	wd, err := os.Getwd()

	if err != nil {
		return "devenv-default"
	}

	projectName := filepath.Base(wd)

	projectName = strings.ToLower(projectName)

	projectName = strings.ReplaceAll(projectName, " ", "-")

	return "devenv-" + projectName
}

func TargetRegistryPort() int {

	if AppConfig.Ports.Registry != 0 {
		return AppConfig.Ports.Registry
	}

	return 5000
}

func TargetJenkinsPort() int {

	if AppConfig.Ports.Jenkins != 0 {
		return AppConfig.Ports.Jenkins
	}

	return 8080
}
