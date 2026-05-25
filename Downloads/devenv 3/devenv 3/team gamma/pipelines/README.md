# Team Gamma - Industrial CI/CD Pipeline

## Overview

This directory contains the complete industrial-grade Jenkins CI/CD pipeline implementation for Team Gamma, supporting multiple languages and frameworks with comprehensive security scanning, policy validation, and deployment automation.

## Features

### Multi-Language Support
- **Node.js** (React, Express, Next.js)
- **Python** (Django, FastAPI, Flask)
- **Java** (Spring Boot with Maven/Gradle)
- **Go** (Standard Go applications)

### Pipeline Stages

1. **Initialize Pipeline** - Setup workspace and artifact directories
2. **Clone Repository** - Checkout code with Git metadata
3. **Detect Project Type** - Automatic language/framework detection
4. **Code Validation** - File existence, syntax checking, encoding validation
5. **Linting** - Language-specific linters (ESLint, pylint, Checkstyle, gofmt)
6. **Unit Testing** - Framework-specific test execution (Jest, pytest, JUnit, go test)
7. **Security Scanning**
   - Trivy filesystem scan
   - Gitleaks secret detection
   - Semgrep SAST analysis
8. **Docker Build** - Multi-stage Docker image creation
9. **Docker Image Scan** - Trivy container image vulnerability scan
10. **OPA Policy Validation** - Conftest policy enforcement
11. **Registry Push** - Push to Docker registry
12. **Kubernetes Deployment** - Deploy to K8s cluster
13. **Deployment Verification** - Health checks and smoke tests
14. **Generate Reports** - Build summary and deployment reports

### Security Features

#### Trivy Scanning
- Filesystem vulnerability scanning
- Container image scanning
- Configurable severity thresholds
- SBOM generation support
- Compliance checking

#### Gitleaks Secret Detection
- Comprehensive secret patterns
- Custom allowlist support
- Multi-format output
- Zero false-positive mode

#### Semgrep SAST
- Security vulnerabilities
- Code quality issues
- Best practice violations
- Custom rule support

#### OPA/Conftest Policies
- No `latest` tags
- Resource limits enforcement
- Security context validation
- Mandatory labels
- Registry validation
- Liveness/readiness probes
- Privileged container prevention

### Reliability Features

- **Retry Logic** - Automatic retry with exponential backoff
- **Timeout Handling** - Configurable timeouts per stage
- **Fail-Fast Execution** - Quick failure on critical errors
- **Detailed Logging** - Comprehensive logs at each stage
- **Build Summaries** - Formatted build reports
- **Artifact Archiving** - Automated artifact collection

### Deployment Features

- **Pod Health Verification** - Ensure pods are running and ready
- **Rollout Status Monitoring** - Track deployment progress
- **Service Accessibility Checks** - Verify service endpoints
- **Smoke Testing** - Basic HTTP health checks
- **Automatic Rollback** - On deployment failure (configurable)

## Directory Structure

```
pipelines/
├── Jenkinsfile                          # Main pipeline definition
├── README.md                            # This file
├── pod-templates/
│   └── multi-language-pod.yaml          # Kubernetes pod template for builds
├── opa-policies/
│   ├── kubernetes.rego                  # K8s manifest policies
│   └── docker.rego                      # Dockerfile policies
├── helpers/
│   ├── vars/
│   │   └── pipelineHelpers.groovy      # Reusable pipeline functions
│   └── validators.groovy                # Project validation utilities
├── security/
│   ├── trivy.yaml                       # Trivy configuration
│   ├── gitleaks.toml                    # Gitleaks rules
│   └── semgrep.yaml                     # Semgrep rules
├── artifacts/                           # Generated artifacts (gitignored)
│   ├── build/                           # Build artifacts
│   ├── test/                            # Test results
│   ├── lint/                            # Lint reports
│   ├── security/                        # Security scan results
│   └── deployment/                      # Deployment metadata
├── reports/                             # Generated reports (gitignored)
│   ├── trivy/                           # Trivy scan reports
│   ├── gitleaks/                        # Gitleaks reports
│   ├── semgrep/                         # Semgrep reports
│   └── opa/                             # OPA policy reports
└── logs/                                # Pipeline logs (gitignored)
```

## Usage

### Basic Usage

```groovy
// Reference in your Jenkinsfile
@Library('team-gamma-pipeline') _

pipeline {
    agent any
    stages {
        stage('CI/CD') {
            steps {
                // Pipeline runs automatically
            }
        }
    }
}
```

### Custom Configuration

```groovy
// Override default configuration
def config = [
    timeout: 45,                    // Pipeline timeout in minutes
    retryAttempts: 5,               // Number of retry attempts
    registryHost: 'myregistry:5000', // Custom registry
    namespace: 'production',        // K8s namespace
]

// Use in pipeline
environment {
    PIPELINE_CONFIG = "${config}"
}
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `REGISTRY_HOST` | Docker registry host | `localhost:5000` |
| `K8S_NAMESPACE` | Kubernetes namespace | `default` |
| `DOCKER_BUILDKIT` | Enable Docker BuildKit | `1` |
| `DOCKER_TLS_VERIFY` | Docker TLS verification | `0` |
| `REGISTRY_INSECURE` | Allow insecure registry | `true` |

## Project Detection

The pipeline automatically detects your project type based on manifest files:

| File | Detected Type |
|------|---------------|
| `package.json` | Node.js (React, Express, etc.) |
| `requirements.txt` | Python (Django, FastAPI, Flask) |
| `pom.xml` | Java (Maven + Spring Boot) |
| `build.gradle` | Java (Gradle + Spring Boot) |
| `go.mod` | Go |

## Linting Support

### ESLint (Node.js)
```bash
npx eslint . --format json --output-file reports/lint-eslint.json
```

### pylint (Python)
```bash
pylint **/*.py --output-format=json > reports/lint-pylint.json
```

### Checkstyle (Java)
```bash
mvn checkstyle:checkstyle
```

### gofmt + go vet (Go)
```bash
gofmt -l . > reports/lint-gofmt.txt
go vet ./... > reports/lint-govet.txt
```

## Testing Support

### Jest (Node.js)
```bash
npm test -- --ci --coverage --watchAll=false
```

### pytest (Python)
```bash
pytest --junitxml=reports/test-results.xml --cov=. --cov-report=html
```

### JUnit (Java)
```bash
mvn test
```

### go test (Go)
```bash
go test -v ./... -coverprofile=reports/coverage.out
```

## Security Scanning

### Trivy
```bash
# Filesystem scan
trivy fs --severity HIGH,CRITICAL --format json --output reports/trivy-fs.json .

# Image scan
trivy image --severity HIGH,CRITICAL --format json --output reports/trivy-image.json myimage:tag
```

### Gitleaks
```bash
gitleaks detect --source . --report-path reports/gitleaks-report.json --config security/gitleaks.toml
```

### Semgrep
```bash
semgrep --config auto --json --output reports/semgrep-results.json .
```

## OPA Policy Validation

### Conftest
```bash
conftest test --policy pipelines/opa-policies --output json k8s/base/*.yml > reports/opa-results.json
```

### Policy Examples

**Deny `latest` tag:**
```rego
deny[msg] {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    endswith(container.image, ":latest")
    msg := "Container uses 'latest' tag"
}
```

**Enforce resource limits:**
```rego
deny[msg] {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    not container.resources.limits
    msg := "Container must have resource limits"
}
```

## Artifact Generation

### Structure
```
artifacts/
├── build/
│   ├── docker-inspect.json           # Docker image metadata
│   ├── docker-history.txt            # Image layer history
│   └── build-summary.txt             # Build summary
├── test/
│   ├── test-results.xml              # JUnit test results
│   ├── coverage.xml                  # Code coverage
│   └── test-report.html              # HTML test report
├── lint/
│   ├── eslint-report.json            # ESLint results
│   └── pylint-report.json            # pylint results
├── security/
│   ├── trivy-fs-scan.json            # Trivy FS scan
│   ├── trivy-image-scan.json         # Trivy image scan
│   ├── gitleaks-report.json          # Gitleaks results
│   └── semgrep-results.json          # Semgrep results
└── deployment/
    ├── deployment-report.md           # Deployment details
    ├── rollout-status.log             # Rollout logs
    └── service-endpoints.yaml         # Service info
```

## Execution Flow

### Complete Flow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Initialize Pipeline                                      │
│    ├─ Create artifact directories                          │
│    └─ Setup workspace                                       │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│ 2. Clone Repository                                         │
│    ├─ Checkout code                                         │
│    ├─ Get Git metadata                                      │
│    └─ Generate image tag                                    │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│ 3. Detect Project Type                                      │
│    ├─ Check for package.json → Node.js                     │
│    ├─ Check for requirements.txt → Python                  │
│    ├─ Check for pom.xml → Java (Maven)                     │
│    ├─ Check for build.gradle → Java (Gradle)               │
│    └─ Check for go.mod → Go                                │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│ 4. Code Validation (Parallel)                               │
│    ├─ File existence checks                                 │
│    ├─ Syntax validation                                     │
│    └─ Encoding verification                                 │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│ 5. Linting                                                   │
│    ├─ ESLint (Node.js)                                      │
│    ├─ pylint (Python)                                       │
│    ├─ Checkstyle (Java)                                     │
│    └─ gofmt + go vet (Go)                                   │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│ 6. Unit Testing                                              │
│    ├─ Jest (Node.js)                                        │
│    ├─ pytest (Python)                                       │
│    ├─ JUnit (Java)                                          │
│    └─ go test (Go)                                          │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│ 7. Security Scanning (Parallel)                             │
│    ├─ Trivy FS Scan ────────┐                              │
│    ├─ Gitleaks Secret Scan ─┼─ All run in parallel         │
│    └─ Semgrep SAST ─────────┘                              │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│ 8. Docker Build                                              │
│    ├─ Build with BuildKit                                   │
│    ├─ Tag with build number + commit                        │
│    └─ Add build metadata labels                             │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│ 9. Docker Image Scan                                         │
│    ├─ Trivy container scan                                  │
│    ├─ Check vulnerability threshold                         │
│    └─ Generate SBOM                                         │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│ 10. OPA Policy Validation                                    │
│     ├─ Validate K8s manifests                               │
│     ├─ Check security policies                              │
│     └─ Enforce best practices                               │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│ 11. Registry Push                                            │
│     ├─ Push with build tag                                  │
│     ├─ Push with latest tag                                 │
│     └─ Verify push success                                  │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│ 12. Kubernetes Deployment                                    │
│     ├─ Create namespace                                     │
│     ├─ Apply manifests                                      │
│     ├─ Update image                                         │
│     └─ Annotate with build info                             │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│ 13. Deployment Verification                                  │
│     ├─ Wait for rollout                                     │
│     ├─ Verify pod health                                    │
│     ├─ Check service accessibility                          │
│     └─ Run smoke tests                                      │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│ 14. Generate Reports                                         │
│     ├─ Build summary                                        │
│     ├─ Deployment report                                    │
│     └─ Archive artifacts                                    │
└─────────────────────────────────────────────────────────────┘
                           ↓
                    ┌──────────┐
                    │ SUCCESS! │
                    └──────────┘
```

### Security Flow

```
Source Code
     ↓
┌─────────────────┐
│ Gitleaks Scan   │ → Detects secrets in code/commits
└─────────────────┘
     ↓
┌─────────────────┐
│ Semgrep SAST    │ → Analyzes code for vulnerabilities
└─────────────────┘
     ↓
┌─────────────────┐
│ Trivy FS Scan   │ → Scans dependencies/files
└─────────────────┘
     ↓
Docker Build
     ↓
┌─────────────────┐
│ Trivy Image Scan│ → Scans container image
└─────────────────┘
     ↓
┌─────────────────┐
│ OPA Validation  │ → Enforces security policies
└─────────────────┘
     ↓
Registry Push
     ↓
K8s Deployment
```

### Registry Flow

```
Docker Build
     ↓
┌──────────────────────────────┐
│ Tag with build number + SHA  │
│ myapp:123-a1b2c3d             │
└──────────────────────────────┘
     ↓
┌──────────────────────────────┐
│ Tag with latest              │
│ myapp:latest                 │
└──────────────────────────────┘
     ↓
┌──────────────────────────────┐
│ Push to Registry             │
│ localhost:5000/myapp:123-... │
└──────────────────────────────┘
     ↓
┌──────────────────────────────┐
│ Verify in Registry           │
│ docker manifest inspect ...  │
└──────────────────────────────┘
     ↓
K8s pulls image from registry
```

### Kubernetes Deployment Flow

```
Registry Image
     ↓
┌─────────────────────────────┐
│ Create/Verify Namespace     │
│ kubectl create namespace... │
└─────────────────────────────┘
     ↓
┌─────────────────────────────┐
│ Apply Manifests             │
│ kubectl apply -k k8s/...    │
└─────────────────────────────┘
     ↓
┌─────────────────────────────┐
│ Update Deployment Image     │
│ kubectl set image...        │
└─────────────────────────────┘
     ↓
┌─────────────────────────────┐
│ Add Build Annotations       │
│ build.number, git.commit    │
└─────────────────────────────┘
     ↓
┌─────────────────────────────┐
│ Wait for Rollout            │
│ kubectl rollout status...   │
└─────────────────────────────┘
     ↓
┌─────────────────────────────┐
│ Verify Pods Running         │
│ Check phase = Running       │
└─────────────────────────────┘
     ↓
┌─────────────────────────────┐
│ Check Service Endpoints     │
│ kubectl get endpoints...    │
└─────────────────────────────┘
     ↓
┌─────────────────────────────┐
│ Run Smoke Tests             │
│ HTTP health checks          │
└─────────────────────────────┘
     ↓
  ┌─────────┐
  │ SUCCESS │
  └─────────┘
```

## Failure Handling

### Retry Behavior
- Network operations: 3 retries with exponential backoff
- Docker commands: 3 retries
- Kubectl commands: 2 retries
- Test failures: No retry (fail immediately)

### Timeout Behavior
| Stage | Default Timeout |
|-------|-----------------|
| Clone | 5 minutes |
| Linting | 10 minutes |
| Testing | 15 minutes |
| Security Scan | 10 minutes each |
| Docker Build | 20 minutes |
| Image Scan | 15 minutes |
| Deployment | 10 minutes |
| Verification | 10 minutes |

### Fail-Fast Execution
Pipeline stops immediately on:
- Test failures
- Critical security vulnerabilities (> threshold)
- Secrets detected in code
- OPA policy violations
- Deployment failures

### Rollback on Failure
- Automatic rollback on deployment verification failure
- Previous version restored
- Failed pods terminated
- Services updated to previous endpoints

## Integration with Existing Teams

### Team Alpha
- Orchestrates overall CI/CD workflow
- Calls Gamma for Jenkins-based builds
- Manages environment setup and teardown

### Team Beta
- Provides Kubernetes cluster
- Manages ingress and networking
- Handles cluster health monitoring

### Team Delta
- Detects project framework
- Generates scaffolding
- Provides project templates

## Troubleshooting

### Common Issues

#### 1. Docker Build Fails
```bash
# Check Docker daemon
docker ps

# Check registry connectivity
curl -v http://localhost:5000/v2/

# View build logs
cat logs/docker-build.log
```

#### 2. Security Scan Failures
```bash
# Update Trivy database
trivy image --download-db-only

# Check Gitleaks config
gitleaks detect --config pipelines/security/gitleaks.toml --verbose

# Test Semgrep rules
semgrep --config pipelines/security/semgrep.yaml --test
```

#### 3. OPA Policy Violations
```bash
# Test policies locally
conftest test --policy pipelines/opa-policies k8s/base/deployment.yml

# View policy violations
cat reports/opa/conftest-results.txt
```

#### 4. Deployment Failures
```bash
# Check pod status
kubectl get pods -n <namespace>

# View pod logs
kubectl logs <pod-name> -n <namespace>

# Check events
kubectl get events -n <namespace> --sort-by='.lastTimestamp'
```

## Performance Optimization

### Build Caching
- Maven dependencies cached in PVC
- npm packages cached in Docker layers
- Go modules cached in build container
- Docker layer caching enabled

### Parallel Execution
- Security scans run in parallel
- Code validation runs in parallel
- Independent stages parallelized

### Resource Limits
Configure in pod template:
```yaml
resources:
  requests:
    memory: "512Mi"
    cpu: "250m"
  limits:
    memory: "2Gi"
    cpu: "1000m"
```

## Contributing

### Adding New Language Support

1. Update `detectProject()` function in Jenkinsfile
2. Add validation logic in `helpers/validators.groovy`
3. Create lint/test stages for new language
4. Update documentation

### Adding Custom OPA Policies

1. Create new `.rego` file in `opa-policies/`
2. Test with `conftest test`
3. Update this README with policy description

### Adding Security Rules

1. **Trivy**: Update `security/trivy.yaml`
2. **Gitleaks**: Add patterns to `security/gitleaks.toml`
3. **Semgrep**: Add rules to `security/semgrep.yaml`

## License

Team Gamma Industrial CI/CD Pipeline
Copyright © 2026

## Support

For issues or questions:
- Check logs in `logs/` directory
- Review artifacts in `artifacts/` directory
- Check security reports in `reports/` directory
- Review Jenkins console output

---

**Generated by Team Gamma** | Version 1.0.0 | Last Updated: 2026-05-14
