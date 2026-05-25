package config

const (
	ClusterName       = "dev-cluster"
	RegistryStartPort = 5000
	// JenkinsUIPort is localhost for the Jenkins UI (Team Gamma kubectl port-forward).
	// MUST stay equal to portalloc.JenkinsReservedLocalPort — application forwards must never bind here.
	JenkinsUIPort = 8080
)
