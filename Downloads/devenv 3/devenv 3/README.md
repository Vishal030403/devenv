# devenv — Local CI/CD Platform

A single CLI that sets up a full local CI/CD environment on your machine:
Kubernetes cluster, Jenkins, Docker registry, project scaffolding, Docker build, deployment, and health checks — all automated.

---

## Prerequisites

Install all of these before running anything.

| Tool | Purpose | Check |
|---|---|---|
| [Go 1.21+](https://golang.org/dl) | Build and run the CLI | `go version` |
| [Docker Desktop](https://www.docker.com/products/docker-desktop) | Build images, run containers | `docker ps` |
| [kubectl](https://kubernetes.io/docs/tasks/tools) | Talk to the cluster | `kubectl version --client` |
| [kind](https://kind.sigs.k8s.io) | Create local Kubernetes cluster | `kind version` |
| [Helm](https://helm.sh/docs/intro/install) | Deploy Jenkins | `helm version` |

> Docker Desktop must be **running** (not just installed) before you start.

---

## Quick Start

### Step 1 — Build the CLI

**Windows (PowerShell)**

```powershell
cd "team alpha"
go build -o devenv.exe .
```

**macOS / Linux (zsh or bash)** — same binary name is fine on Mac; invoke it with `./`:

```bash
cd "team alpha"
go build -o devenv.exe .
# or: go build -o devenv .
```

The CLI is **not** on your `PATH` unless you install it yourself. Use `./devenv.exe` (or `./devenv`) from the `team alpha` directory, not bare `devenv`.

On **macOS / Linux**, use `./` to run a binary in the current folder. Do **not** use Windows-style `.\devenv.exe` — in zsh that is not treated as “run this file” and you may see `command not found: .devenv.exe`.

### Step 2 — Set up the environment (run once)

```powershell
.\devenv.exe setup
```

**macOS / Linux:** `./devenv.exe setup`

This will:
- Run preflight checks (Docker, kubectl, kind, port availability)
- Create a Kind Kubernetes cluster (`dev-cluster`)
- Install NGINX Ingress Controller
- Start a local Docker registry on port `5000`
- Deploy Jenkins via Helm into the cluster
- Detect your project framework and generate scaffolding

Takes about **5 minutes** on first run.

### Step 3 — Run the full CI/CD pipeline

```powershell
.\devenv.exe run
```

**macOS / Linux:** `./devenv.exe run`

This will:
- Lint project structure
- Run unit tests (if a test script exists)
- Run security scan (if hadolint or trivy is installed)
- Build your Docker image
- Validate Kubernetes manifests
- Load the image into the Kind cluster
- Deploy to Kubernetes
- Wait for rollout to be ready
- Run health verification (auto-rollback on failure)
- Start port-forward automatically

Takes about **3 minutes**. When done, open:

```
http://localhost:8080
```

### Pipeline validation (offline)

Validate generated Jenkinsfiles before they reach Jenkins or deployment:

```powershell
.\devenv.exe pipeline test security   # project Jenkinsfile + k8s dry-run + structure
.\devenv.exe pipeline test all          # security + integrated script + all framework templates
```

`devenv run` also runs **Jenkinsfile Validation** after scaffolding (fail-fast).

### Jenkins lifecycle (optional)

Jenkins is deployed during `setup` via Team Gamma (Helm in-cluster + `kubectl port-forward` to **http://127.0.0.1:8080**). You can manage it without re-running full setup:

```powershell
.\devenv.exe pipeline jenkins start    # registry + Jenkins + UI :8080 + integrated job
.\devenv.exe pipeline jenkins status   # Helm pod health + UI reachability
.\devenv.exe pipeline jenkins stop     # stop UI port-forward only (cluster release stays)
.\devenv.exe pipeline jenkins stop --full   # uninstall Jenkins Helm release
```

Agent pods use **Docker-in-Docker** (not host `/var/run/docker.sock`) to build images; pushes use `host.docker.internal:<registry-port>` from the platform ConfigMap.

### Step 4 — Check status

```powershell
.\devenv.exe status
```

**macOS / Linux:** `./devenv.exe status` (or `./devenv status` if you built that name).

Shows cluster state, all pods, registry health, and Jenkins status.

### Step 5 — Tear everything down

```powershell
.\devenv.exe down
```

**macOS / Linux:** `./devenv.exe down`

Removes Jenkins, the Kind cluster, the Docker registry, and the built image.

---

## Troubleshooting

### `zsh: command not found: devenv`

You ran `devenv` as a global command. After `go build`, the binary lives in `team alpha/` only. Use:

```bash
./devenv.exe status
# or add team alpha to PATH / copy the binary to a directory on PATH
```

### `zsh: command not found: .devenv.exe`

You used **Windows** PowerShell syntax (`.\devenv.exe`). On macOS/Linux use **`./devenv.exe`** (forward slash), e.g. `./devenv.exe run`.

### `devenv run is already executing (PID …)`

`devenv run` uses a lock file so two pipelines do not run at once. That message means **some OS process with that PID is still alive** (often a previous `./devenv.exe run` still working, or stuck).

1. Check whether a run is still going: `./devenv.exe status` (same binary you use for `run`).
2. If nothing should be running, remove the stale lock file (same path the error message lists). On macOS in a normal Terminal session:

```bash
rm -f "$TMPDIR/.devenv.lock"
```

If `TMPDIR` is empty, use the full lock path shown in the error message (it is under `os.TempDir()`, e.g. `/tmp` on some Linux systems).

3. If a real `devenv` process is still running, wait for it or stop it (Activity Monitor / `ps aux | grep devenv`).

### `setup` fails on Team Gamma with `syntax error: unexpected <<`

Unresolved Git merge conflict markers in `team gamma/cmd/*.go`. Fix conflicts, then `go build` again from `team alpha`.

---

## Target App

By default, devenv uses the bundled sample app at:

```
sample apps/react-demo
```

To use your own project, set the path before running:

```powershell
$env:DEVENV_PROJECT_PATH = "C:\path\to\your\app"
.\devenv.exe setup
.\devenv.exe run
```

### Scaffolded `Jenkinsfile`

`devenv setup` (Team Delta) generates a **declarative** `Jenkinsfile` in your project with stages aligned to `devenv run`: Linting, Unit Tests, Security Scan, Docker Build, Registry Push, Kubernetes Deployment, Rollout Verification, Smoke Testing, and Rollback Handling. Registry and cluster hosts come from the shared platform ConfigMap — not hardcoded in the file.

### Supported frameworks (auto-detected)

| Framework | Detected by |
|---|---|
| React | `package.json` with react dependency |
| Express | `package.json` with express dependency |
| Django | `requirements.txt` + `manage.py` |
| FastAPI | `requirements.txt` + `main.py` / `app.py` |
| Java Spring Boot | `pom.xml` or `build.gradle` |

---

## Project Structure

```
devenv-cli/
├── team alpha/       Main CLI and orchestrator (Cobra, Go)
├── team beta/        Preflight checks, Kind cluster, NGINX ingress
├── team gamma/       Jenkins and local Docker registry
├── team delta/       Project detection and scaffolding templates
└── sample apps/
    └── react-demo/   Demo React app (Vite + Nginx + K8s manifests)
```

### How the teams connect

Team Alpha owns the `devenv` binary. It delegates to each other team by calling their CLIs as subprocesses:

```
devenv setup
  └── team beta  →  go run . up       (cluster + ingress)
  └── team gamma →  go run . up       (Jenkins + registry)
  └── team delta →  go run . up --path (scaffolding)

devenv run
  └── Tier 1: re-runs beta + delta (developer layer)
  └── Tier 2: lint → tests → docker build → k8s validation
  └── Tier 3: kind load → kubectl apply → rollout wait → health check → port-forward

devenv status
  └── team beta  →  go run . status
  └── team gamma →  go run . status
  └── team delta →  go run . status --path

devenv down
  └── team gamma →  go run . down
  └── team beta  →  go run . down
  └── team delta →  go run . down --path
  └── team alpha →  docker rmi (removes built image)
```

---

## Optional — Security Scanners

Install either of these for the security scan step in `devenv run`:

- **hadolint** — Dockerfile linter: https://github.com/hadolint/hadolint
- **trivy** — Image vulnerability scanner: https://aquasecurity.github.io/trivy

If neither is installed, the step is skipped with a warning and the pipeline continues.
