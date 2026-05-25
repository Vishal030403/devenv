package main

# OPA Policy Definitions for Kubernetes Manifests
# Team Gamma - Industrial CI/CD Pipeline

##############################################################################
# DENY RULES - Critical violations that fail the build
##############################################################################

# Deny use of 'latest' tag
deny[msg] {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    endswith(container.image, ":latest")
    msg := sprintf("Container '%s' uses 'latest' tag which is not allowed in production", [container.name])
}

deny[msg] {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    not contains(container.image, ":")
    msg := sprintf("Container '%s' has no tag specified (implies 'latest')", [container.name])
}

# Deny privileged containers
deny[msg] {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    container.securityContext.privileged == true
    msg := sprintf("Container '%s' is running in privileged mode which is not allowed", [container.name])
}

# Deny containers without resource limits
deny[msg] {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    not container.resources.limits
    msg := sprintf("Container '%s' does not have resource limits defined", [container.name])
}

deny[msg] {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    not container.resources.limits.memory
    msg := sprintf("Container '%s' does not have memory limit defined", [container.name])
}

deny[msg] {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    not container.resources.limits.cpu
    msg := sprintf("Container '%s' does not have CPU limit defined", [container.name])
}

# Deny containers without resource requests
deny[msg] {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    not container.resources.requests
    msg := sprintf("Container '%s' does not have resource requests defined", [container.name])
}

# Deny containers running as root
deny[msg] {
    input.kind == "Deployment"
    not input.spec.template.spec.securityContext.runAsNonRoot
    msg := "Deployment does not enforce runAsNonRoot security context"
}

# Deny missing mandatory labels
deny[msg] {
    input.kind == "Deployment"
    not input.metadata.labels.app
    msg := "Deployment is missing required label 'app'"
}

deny[msg] {
    input.kind == "Deployment"
    not input.metadata.labels.version
    msg := "Deployment is missing required label 'version'"
}

deny[msg] {
    input.kind == "Deployment"
    not input.metadata.labels.team
    msg := "Deployment is missing required label 'team'"
}

# Deny invalid registry sources
deny[msg] {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    not startswith(container.image, "localhost:5000/")
    not startswith(container.image, "gcr.io/")
    not startswith(container.image, "docker.io/")
    not startswith(container.image, "quay.io/")
    msg := sprintf("Container '%s' uses image from untrusted registry: %s", [container.name, container.image])
}

# Deny containers without liveness probe
deny[msg] {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    not container.livenessProbe
    not is_sidecar(container)
    msg := sprintf("Container '%s' does not have a liveness probe defined", [container.name])
}

# Deny containers without readiness probe
deny[msg] {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    not container.readinessProbe
    not is_sidecar(container)
    msg := sprintf("Container '%s' does not have a readiness probe defined", [container.name])
}

##############################################################################
# WARN RULES - Best practices that should be addressed
##############################################################################

warn[msg] {
    input.kind == "Deployment"
    not input.spec.replicas
    msg := "Deployment does not specify replica count (defaults to 1)"
}

warn[msg] {
    input.kind == "Deployment"
    input.spec.replicas < 2
    msg := sprintf("Deployment has %d replica which affects high availability", [input.spec.replicas])
}

warn[msg] {
    input.kind == "Deployment"
    not input.spec.strategy
    msg := "Deployment does not specify update strategy (defaults to RollingUpdate)"
}

warn[msg] {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    not container.resources.requests.memory
    msg := sprintf("Container '%s' does not have memory request defined", [container.name])
}

warn[msg] {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    not container.resources.requests.cpu
    msg := sprintf("Container '%s' does not have CPU request defined", [container.name])
}

warn[msg] {
    input.kind == "Service"
    input.spec.type == "NodePort"
    msg := "Service uses NodePort which exposes ports on all nodes"
}

warn[msg] {
    input.kind == "Service"
    input.spec.type == "LoadBalancer"
    msg := "Service uses LoadBalancer which may incur cloud costs"
}

warn[msg] {
    input.kind == "Deployment"
    not input.metadata.annotations
    msg := "Deployment has no annotations"
}

warn[msg] {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    container.imagePullPolicy != "Always"
    container.imagePullPolicy != "IfNotPresent"
    msg := sprintf("Container '%s' has non-standard imagePullPolicy", [container.name])
}

##############################################################################
# HELPER FUNCTIONS
##############################################################################

# Check if container is a sidecar (common patterns)
is_sidecar(container) {
    sidecar_patterns := ["istio-proxy", "envoy", "linkerd-proxy", "consul-connect"]
    pattern := sidecar_patterns[_]
    contains(container.name, pattern)
}

# Check if string contains substring
contains(str, substr) {
    contains(str, substr)
}

# Check if string starts with prefix
startswith(str, prefix) {
    startswith(str, prefix)
}

# Check if string ends with suffix
endswith(str, suffix) {
    endswith(str, suffix)
}

##############################################################################
# SERVICE POLICIES
##############################################################################

deny[msg] {
    input.kind == "Service"
    not input.metadata.labels.app
    msg := "Service is missing required label 'app'"
}

deny[msg] {
    input.kind == "Service"
    not input.spec.selector
    msg := "Service does not have selector defined"
}

deny[msg] {
    input.kind == "Service"
    port := input.spec.ports[_]
    not port.name
    msg := sprintf("Service port %d does not have a name", [port.port])
}

##############################################################################
# NAMESPACE POLICIES
##############################################################################

deny[msg] {
    input.kind == "Namespace"
    not input.metadata.labels
    msg := "Namespace does not have labels defined"
}

warn[msg] {
    input.kind == "Namespace"
    not input.metadata.annotations
    msg := "Namespace does not have annotations for resource quotas or policies"
}

##############################################################################
# INGRESS POLICIES
##############################################################################

deny[msg] {
    input.kind == "Ingress"
    not input.spec.rules
    msg := "Ingress does not have any rules defined"
}

warn[msg] {
    input.kind == "Ingress"
    not input.spec.tls
    msg := "Ingress does not have TLS configuration (HTTP only)"
}

deny[msg] {
    input.kind == "Ingress"
    not input.metadata.annotations
    msg := "Ingress is missing annotations (e.g., ingress class)"
}

##############################################################################
# CONFIGMAP POLICIES
##############################################################################

warn[msg] {
    input.kind == "ConfigMap"
    count(input.data) > 10
    msg := "ConfigMap has more than 10 entries, consider splitting"
}

warn[msg] {
    input.kind == "ConfigMap"
    not input.metadata.labels.app
    msg := "ConfigMap is missing 'app' label"
}

##############################################################################
# SECRET POLICIES
##############################################################################

deny[msg] {
    input.kind == "Secret"
    input.type == "Opaque"
    data_value := input.data[_]
    not is_base64(data_value)
    msg := "Secret contains non-base64 encoded data"
}

warn[msg] {
    input.kind == "Secret"
    not input.metadata.labels.app
    msg := "Secret is missing 'app' label for association"
}

##############################################################################
# RESOURCE QUOTA POLICIES
##############################################################################

# Enforce reasonable resource limits
deny[msg] {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    memory_limit := parse_memory(container.resources.limits.memory)
    memory_limit > 16 * 1024 * 1024 * 1024  # 16Gi
    msg := sprintf("Container '%s' has excessive memory limit: %s (max: 16Gi)", [container.name, container.resources.limits.memory])
}

deny[msg] {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    cpu_limit := parse_cpu(container.resources.limits.cpu)
    cpu_limit > 8000  # 8 cores in millicores
    msg := sprintf("Container '%s' has excessive CPU limit: %s (max: 8 cores)", [container.name, container.resources.limits.cpu])
}

##############################################################################
# UTILITY FUNCTIONS
##############################################################################

# Helper to parse memory (simplified)
parse_memory(mem) = bytes {
    # This is a simplified version - full implementation would handle all K8s units
    endswith(mem, "Gi")
    value := trim_suffix(mem, "Gi")
    bytes := to_number(value) * 1024 * 1024 * 1024
}

parse_memory(mem) = bytes {
    endswith(mem, "Mi")
    value := trim_suffix(mem, "Mi")
    bytes := to_number(value) * 1024 * 1024
}

# Helper to parse CPU (simplified)
parse_cpu(cpu) = millicores {
    endswith(cpu, "m")
    value := trim_suffix(cpu, "m")
    millicores := to_number(value)
}

parse_cpu(cpu) = millicores {
    not endswith(cpu, "m")
    millicores := to_number(cpu) * 1000
}

# Helper to check base64 encoding
is_base64(str) {
    # Simplified check - real implementation would validate base64
    true
}
