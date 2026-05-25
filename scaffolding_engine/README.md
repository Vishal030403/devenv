# 🚀 Pipeline Scaffolding Engine

> A CLI tool that **auto-detects your project's framework** and instantly generates production-ready `Dockerfile` and Kubernetes manifests — so you can focus on code, not config.

---

## 📖 Table of Contents

1. [What Does It Do?](#-what-does-it-do)
2. [Supported Frameworks](#-supported-frameworks)
3. [How Detection Works](#-how-detection-works)
4. [Template Variables](#-template-variables)
5. [Generated File Structure](#-generated-file-structure)
6. [Getting Started](#-getting-started)
7. [Testing Each Framework](#-testing-each-framework)
8. [Project Architecture](#-project-architecture)
9. [Adding a New Framework](#-adding-a-new-framework)
10. [Troubleshooting](#-troubleshooting)

---

## 🪄 What Does It Do?

When you run `pipeline init` inside your project folder, the tool does three things automatically:

```
Your project folder
       │
       ▼
  [1] DETECT ──────── Reads signal files (package.json, manage.py, pom.xml …)
       │                       │
       ▼                       ▼
  [2] LOAD ───────── Picks the matching template set for your framework
       │
       ▼
  [3] GENERATE ────── Renders & writes Dockerfile + full k8s manifests into your folder
```

**What you get in seconds:**

| File | Purpose |
|---|---|
| `Dockerfile` | Multi-stage, production-ready, runs as non-root |
| `k8s/base/deployment.yml` | Kubernetes Deployment |
| `k8s/base/service.yml` | Kubernetes Service (ClusterIP) |
| `k8s/base/ingress.yml` | Kubernetes Ingress |
| `k8s/base/kustomization.yml` | Kustomize base manifest list |
| `k8s/overlays/local/` | Local dev overrides (`.local` hostname) |
| `k8s/overlays/prod/` | Production overrides (TLS + cert-manager) |

---

## 🗂️ Supported Frameworks

| Framework | Detection Signal | Port | Base Image |
|---|---|---|---|
| **React** | `package.json` with `"react"` or `"react-scripts"` | 80 | `node:18-alpine` → `nginx:alpine` |
| **Node / Express** | `package.json` (any, without React) | 3000 | `node:18-alpine` |
| **FastAPI** | `main.py` or `app.py` containing `FastAPI`, OR `requirements.txt` containing `fastapi` | 8000 | `python:3.10-slim` |
| **Django** | `manage.py` exists at project root | 8000 | `python:3.9-slim` |
| **Java Spring Boot** | `pom.xml` or `build.gradle` exists | 8080 | `eclipse-temurin:17-jdk-alpine` |

---

## 🔍 How Detection Works

The detector (`internal/detector/detector.go`) checks signal files in the **current working directory** in a fixed priority order. **The first match wins.**

```
Priority 1 ──► pom.xml OR build.gradle  →  java_spring_boot
     │
Priority 2 ──► manage.py               →  django
     │
Priority 3 ──► main.py (contains "FastAPI")
            OR app.py  (contains "FastAPI")   →  fast_api
            OR requirements.txt (contains "fastapi")
     │
Priority 4 ──► package.json (has "react" or "react-scripts")  →  react
     │
Priority 5 ──► package.json (any other)  →  node_express
     │
     └──► none of the above  →  unknown  (no files generated)
```

> **Why this order?**  
> Java and Django have unique, unambiguous signal files (`pom.xml`, `manage.py`).  
> Python projects are checked before Node to avoid a `requirements.txt`/`package.json` clash.  
> React is checked before plain Node because both use `package.json`.

### FastAPI — Three Ways to Detect

Because FastAPI projects use different entry-point naming conventions, the detector checks all three:

```
✅  main.py  containing the word "FastAPI"   (most common tutorial style)
✅  app.py   containing the word "FastAPI"   (production / larger projects)
✅  requirements.txt  containing "fastapi"   (containerised / no local .py at root)
```

---

## 🔧 Template Variables

Every template (`.tmpl` file) can use these Go template variables with `{{ .variable_name }}` syntax:

| Variable | Type | Example Value | Used In |
|---|---|---|---|
| `{{ .app_name }}` | string | `my-app` | All k8s manifests, Dockerfile labels |
| `{{ .app_port }}` | int | `8000` | Dockerfile `EXPOSE`, k8s `containerPort` |
| `{{ .run_command }}` | string | `["uvicorn", "main:app", ...]` | Dockerfile `CMD` |
| `{{ .python_version }}` | string | `3.10` | FastAPI / Django `FROM python:X-slim` |
| `{{ .node_version }}` | string | `18` | React / Node `FROM node:X-alpine` |
| `{{ .java_version }}` | string | `17` | Spring Boot `FROM eclipse-temurin:X` |

> **`app_name` is auto-sanitized.**  
> The raw folder name is converted to lowercase and all spaces/underscores are replaced with hyphens, making it safe for Kubernetes resource names (RFC 1123 DNS label format).  
> Example: `My_Cool App` → `my-cool-app`

---

## 📁 Generated File Structure

After running `pipeline init` in your project, you'll find:

```
your-project/
├── Dockerfile                          ← production-ready, non-root, multi-stage
└── k8s/
    ├── base/                           ← shared manifests (apply to all environments)
    │   ├── deployment.yml
    │   ├── service.yml
    │   ├── ingress.yml
    │   └── kustomization.yml
    └── overlays/
        ├── local/                      ← for local / minikube testing
        │   ├── kustomization.yml
        │   └── patch-ingress.yml       ← sets host to <app-name>.local
        └── prod/                       ← for production cluster
            ├── kustomization.yml
            └── patch-ingress.yml       ← adds TLS + cert-manager annotation
```

### Applying with Kustomize

```bash
# Deploy to local cluster (minikube, kind, etc.)
kubectl apply -k k8s/overlays/local

# Deploy to production
kubectl apply -k k8s/overlays/prod
```

---

## 🛠️ Getting Started

### Prerequisites

- [Go 1.20+](https://golang.org/dl/) installed and on your `PATH`

### Build the Binary

```bash
# From the scaffolding_engine directory:

# 1. Download dependencies
go mod tidy

# 2. Build the CLI executable
go build -o pipeline.exe main.go
```

You now have a `pipeline.exe` (Windows) or `pipeline` (Linux/macOS) binary.

### Run It

Navigate into **any** project folder and run:

```bash
cd /path/to/your/project
/path/to/pipeline.exe init
```

Example output:

```
Detected framework: fast_api
Generated C:\projects\my-api\Dockerfile
Generated C:\projects\my-api\k8s\base\deployment.yml
Generated C:\projects\my-api\k8s\base\service.yml
Generated C:\projects\my-api\k8s\base\ingress.yml
Generated C:\projects\my-api\k8s\base\kustomization.yml
Generated C:\projects\my-api\k8s\overlays\local\kustomization.yml
Generated C:\projects\my-api\k8s\overlays\local\patch-ingress.yml
Generated C:\projects\my-api\k8s\overlays\prod\kustomization.yml
Generated C:\projects\my-api\k8s\overlays\prod\patch-ingress.yml
Scaffolding generation successful!
```

---

## 🧪 Testing Each Framework

Use these quick commands to fake a project and test each detection path.

### React

```bash
mkdir test_react && cd test_react
echo {"dependencies": {"react": "^18.0.0"}} > package.json
..\pipeline.exe init
# Expected: Detected framework: react
```

### Node / Express

```bash
mkdir test_node && cd test_node
echo {"name": "my-api", "main": "index.js"} > package.json
..\pipeline.exe init
# Expected: Detected framework: node_express
```

### FastAPI (via requirements.txt)

```bash
mkdir test_fastapi && cd test_fastapi
echo fastapi==0.110.0 > requirements.txt
..\pipeline.exe init
# Expected: Detected framework: fast_api
```

### FastAPI (via app.py)

```bash
mkdir test_fastapi2 && cd test_fastapi2
echo from fastapi import FastAPI > app.py
..\pipeline.exe init
# Expected: Detected framework: fast_api
```

### Django

```bash
mkdir test_django && cd test_django
echo "" > manage.py
..\pipeline.exe init
# Expected: Detected framework: django
```

### Java Spring Boot

```bash
mkdir test_java && cd test_java
echo "" > pom.xml
..\pipeline.exe init
# Expected: Detected framework: java_spring_boot
```

---

## 🏗️ Project Architecture

```
scaffolding_engine/
├── main.go                         ← Entry point, calls cmd.Execute()
│
├── cmd/
│   ├── root.go                     ← Cobra root command ("pipeline")
│   └── init.go                     ← "pipeline init" subcommand logic
│
├── internal/
│   ├── detector/
│   │   └── detector.go             ← Framework detection logic
│   └── generator/
│       └── generator.go            ← Template loading, rendering, file writing
│
└── templates/                      ← One folder per supported framework
    ├── react/
    │   ├── Dockerfile.tmpl
    │   └── k8s/
    │       ├── base/               ← deployment, service, ingress, kustomization
    │       └── overlays/
    │           ├── local/          ← patch-ingress (hostname: <app>.local)
    │           └── prod/           ← patch-ingress (TLS + cert-manager)
    ├── fast_api/  (same structure)
    ├── django/    (same structure)
    ├── node_express/ (same structure)
    └── java_spring_boot/ (same structure)
```

### How the Generator Works

1. Reads the framework name from the detector
2. Looks for templates in `<exe-dir>/templates/<framework>/` (falls back to CWD if not found — useful for `go run`)
3. Walks every `.tmpl` file recursively
4. Renders each file with Go's `text/template` engine using the variable map
5. Writes the rendered output to the same relative path in the project folder, stripping the `.tmpl` extension

---

## ➕ Adding a New Framework

1. **Add a detection rule** in `internal/detector/detector.go`
   - Check for a unique signal file (e.g., `Gemfile` for Ruby, `go.mod` for Go)
   - Insert at the right priority position in `DetectFramework`

2. **Create a template folder** under `templates/<your_framework>/`
   - Must contain `Dockerfile.tmpl`
   - Must contain the full `k8s/base/` and `k8s/overlays/local|prod/` structure

3. **Register defaults** in `internal/generator/generator.go`
   - Add an entry to the `defaults` map with `app_port` and `run_command`

4. **Rebuild:** `go build -o pipeline.exe main.go`

That's all — no other files need to change.

---

## 🔒 Security Defaults (Applied to Every Template)

All generated Dockerfiles follow these hardening rules out of the box:

| Practice | Detail |
|---|---|
| **Non-root user** | A dedicated `appuser` is created and set via `USER appuser` |
| **Minimal base image** | Alpine or `-slim` variants only |
| **Multi-stage builds** | Build tools are not present in the final image |
| **No secrets baked in** | No ENV vars with credentials in any template |

---

## 🩺 Troubleshooting

| Problem | Cause | Fix |
|---|---|---|
| `templates directory not found` | Binary run from wrong location | Run from the `scaffolding_engine` dir, or place `templates/` next to `pipeline.exe` |
| `Detected framework: unknown` | No signal file found | Check the [detection table](#-supported-frameworks) and create the right file |
| FastAPI not detected | Entry point is named something other than `main.py` / `app.py` | Add `fastapi` to your `requirements.txt` |
| Kubernetes name validation error | Folder name had spaces or special chars | The tool auto-sanitizes to lowercase-hyphens; verify with `kubectl apply --dry-run=client` |
| nginx container exits immediately | PID file permission issue | Fixed in current version — rebuild with `go build -o pipeline.exe main.go` |
| Spring Boot container exits immediately | Wrong main class | Fixed in current version — Dockerfile now uses fat-jar (`java -jar /app.jar`) |
