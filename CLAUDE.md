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
internal/cmd/protection.go → Production protection helpers (confirmation prompts)
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

1. **Shell abstraction**: `pkg/shell` provides `Run()`, `RunInDir()`, `RunWithEnv()`, `RunWithTimeout()` - all return `*Result` with stdout, stderr, exit code, and duration. Use `Runner` interface for testability. Enhanced verbose mode shows command output in real-time.

2. **Config loading**: `config.Load()` searches up directory tree for `.drift.yaml`/`.drift.yml`. `MergeWithDefaults()` fills missing values. Access paths via `cfg.GetFunctionsPath()`, etc.

3. **Split configuration**: `.drift.yaml` for team settings (committed), `.drift.local.yaml` for developer-specific settings (gitignored). Use `config.LoadWithLocal()` to load both and merge.

4. **Git worktree naming**: `GetWorktreePath(project, branch, pattern)` uses `{project}-{branch}` pattern. Branch names are sanitized (slashes → hyphens).

5. **Environment mapping**: Git branches map to Supabase environments. `main`/`master` → production, `dev`/`development` → development, others → feature branches.

6. **Protected branches**: Defined in config, checked before destructive operations on production.

7. **Production protection**: Use `ConfirmProductionOperation()` for standard confirmations, `RequireProductionConfirmation()` for strict confirmations requiring "yes" input. Both respect `--yes` flag for automation.

8. **Per-environment configuration**: `cfg.Environments` map holds environment-specific secrets and push keys. Access via `cfg.Environments["production"].Secrets`.

9. **Function restrictions**: `cfg.Functions.Restricted` lists functions blocked from certain environments. Check before deployment.

### Command Flow

Commands in `internal/cmd/` follow this pattern:
- `init()` registers flags and subcommands
- `Run` functions validate inputs, load config, execute operations
- UI feedback via `ui.Success()`, `ui.Warning()`, `ui.Error()`, spinners
- Confirmations via `ui.PromptYesNo()`, can be skipped with `--yes` flag

### Testing Approach

Tests create temporary git repositories via `setupTestRepo(t)` helper. For path comparisons on macOS, use `filepath.EvalSymlinks()` to handle `/var` → `/private/var` symlinks.
