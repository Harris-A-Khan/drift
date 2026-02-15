# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Drift is an opinionated CLI for Supabase-backed iOS, macOS, and web development workflows. It wraps common operations (environment switching, git worktrees, database operations, edge function deployment, migrations) into a cohesive tool.

## Build & Development Commands

```bash
# Build
make build                    # Build to bin/drift
make install                  # Build and install to /usr/local/bin

# Test
go test ./...                 # Run all tests
go test -v ./internal/git/... # Run specific package tests
go test ./... -cover          # With coverage

# Lint & Format
make lint                     # Run golangci-lint (or go vet)
make fmt                      # Format code
make vet                      # Run go vet

# Dependencies
make tidy                     # go mod tidy
make deps                     # go mod download
```

## Architecture

### Package Structure

```
cmd/drift/main.go        → Entry point, injects version via ldflags
internal/cmd/            → Cobra command implementations (root, env, worktree, deploy, db, migrate, backup, build, doctor, init, device, xcode)
internal/cmd/branch_resolution.go → Shared Supabase branch resolution + fallback policy
internal/cmd/protection.go → Environment-aware confirmation helpers
internal/config/         → .drift.yaml loading, defaults, merging
internal/config/local.go → .drift.local.yaml handling (developer-specific settings)
internal/git/            → Git operations (repo, branch, worktree management)
internal/database/       → pg_dump/pg_restore wrappers
internal/supabase/       → Supabase CLI/API interactions (branches, functions, secrets, storage)
internal/ui/             → Terminal UI (colors, spinners, prompts, tables)
internal/xcode/          → xcconfig generation, version file management
internal/xcode/schemes.go → Xcode scheme management and validation
pkg/shell/               → Shell command execution with context support
```

### Key Design Patterns

1. **Shell abstraction**: `pkg/shell` provides `Run()`, `RunInDir()`, `RunWithEnv()`, `RunWithTimeout()` and returns `*Result` (stdout/stderr/exit code/duration). Use `Runner` where testability matters.

2. **Config loading always includes local overrides**:
   - `config.Load()` loads only `.drift.yaml`
   - `config.LoadWithLocal()` loads `.drift.yaml` + `.drift.local.yaml` and merges
   - `config.LoadOrDefault()` currently calls `LoadWithLocal()` so most commands see merged config by default

3. **Split config contract**:
   - `.drift.yaml`: team/shared settings (committed)
   - `.drift.local.yaml`: developer/branch/worktree overrides (gitignored)
   - Local merge supports: `supabase.override_branch`, `supabase.fallback_branch`, `apple.key_search_paths`, and `environments.<env>.secrets`/`push_key`/`skip_secrets`

4. **Supabase branch resolution is centralized** (`internal/cmd/branch_resolution.go`):
   - Exact target first (`--branch` wins; otherwise local `override_branch` if set)
   - On no match, fallback order is:
     1) global `--fallback-branch`
     2) `supabase.fallback_branch` from merged config
     3) interactive non-production selection
   - Production-like branches cannot be used as override/fallback targets.

5. **Environment mapping + lookup normalization**:
   - Branch env classes: production / development / feature
   - `GetEnvironmentConfig()` normalizes names (`Production`/`production`, etc.)
   - Feature env config falls back to development config when feature-specific config is absent

6. **Deploy confirmation policy** (`ConfirmDeploymentOperation()`):
   - Production/protected branches: strict typed confirmation
   - Development: yes/no confirmation
   - Feature: no extra prompt
   - All respect global `--yes`

7. **Secrets deployment model** (`drift deploy secrets`):
   - `supabase.secrets_to_push`: allowed key list
   - `supabase.default_secrets`: baseline key/value map
   - APNs-derived secrets are auto-discovered
   - `environments.<env>.secrets` overlays defaults (local overrides shared)
   - `environments.<env>.skip_secrets` prevents specific keys from being pushed on that env
   - If `secrets_to_push` is empty, all discovered/merged keys are candidates

8. **Interactive secret setup** (`drift config set-secret`):
   - Wizard updates shared and local config with a short question flow
   - Updates `secrets_to_push`, `default_secrets`, env overrides, and production skip policy
   - Intended to encode secret hierarchy without manual YAML editing

9. **APNs key discovery is explicit and configurable**:
   - Uses `apple.push_key_pattern`
   - Search paths from `apple.key_search_paths` (or defaults: `secrets`, `.`, `..`)
   - CLI can override with repeated `--key-search-dir`
   - Deploy output shows search directories and matched key file

10. **Function restrictions**: `cfg.Supabase.Functions.Restricted` blocks specific functions per environment during deploy.

### Command Flow

Commands in `internal/cmd/` follow this pattern:
- `init()` registers flags and subcommands
- `Run` functions validate inputs, load merged config, resolve branch target, execute operations
- UI feedback via `ui.Success()`, `ui.Warning()`, `ui.Error()`, spinners
- Confirmations via `ui.PromptYesNo()`, can be skipped with `--yes` flag
- Verbose mode should explain branch resolution, fallback source, and secret selection decisions

### Testing Approach

Tests create temporary git repositories via `setupTestRepo(t)` helper. For path comparisons on macOS, use `filepath.EvalSymlinks()` to handle `/var` → `/private/var` symlinks.

In restricted sandbox environments, run tests with:

```bash
GOCACHE=/tmp/go-build GOPROXY=off go test ./internal/config ./internal/supabase ./internal/cmd
```
