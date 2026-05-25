# Team Alpha - devenv CLI and Orchestrator

Team Alpha owns the main `devenv` CLI and the orchestration flow.

This folder is intentionally separate from `../team beta` and `../team delta`. Alpha calls each team's CLI by using sibling folder paths, so there are no hardcoded user-specific paths.

## Commands

From this folder:

```bash
go run . setup
go run . run
go run . status
go run . down
```

Build the Alpha CLI:

```bash
go build -o devenv.exe .
```

Run the built CLI:

```bash
.\devenv.exe setup
.\devenv.exe run
.\devenv.exe status
.\devenv.exe down
```

## Current Integration

Alpha delegates Team Beta work to the sibling folder:

```text
../team beta
```

Alpha delegates Team Delta work to the sibling folder:

```text
../team delta
```

Beta mapping:

- `devenv setup` calls `go run . up` inside `../team beta`
- `devenv status` calls `go run . status` inside `../team beta`
- `devenv down` calls `go run . down` inside `../team beta`

Delta mapping:

- `devenv setup` calls `go run . up --path <project-path>` inside `../team delta`
- `devenv status` calls `go run . status --path <project-path>` inside `../team delta`
- `devenv down` calls `go run . down --path <project-path>` inside `../team delta`

By default, `<project-path>` is the folder where `devenv` is run. You can override it:

```bash
$env:DEVENV_PROJECT_PATH = "..\sample-app"
.\devenv.exe setup
```

Gamma mapping:

- `devenv setup` calls `go run . up` inside `../team gamma`
- `devenv status` calls `go run . status` inside `../team gamma`
- `devenv down` calls `go run . down` inside `../team gamma`

## Expected Parent Layout

```text
project-root/
  team alpha/
    cmd/
    internal/
    orchestrator/
    main.go
    go.mod
    go.sum
    README.md

  team beta/
    cmd/
    internal/
    pkg/
    main.go
    go.mod
    go.sum

  team delta/
    core/
    templates/
    main.go
    go.mod

  team gamma/
    cmd/
    internal/
    main.go
    go.mod
```

## Requirements

- Go must be installed and available in PATH.
- Docker Desktop must be running for setup/status/down.
- kubectl must be installed and available in PATH.
- kind must be installed and available in PATH.
- helm must be installed and available in PATH for Jenkins setup.
