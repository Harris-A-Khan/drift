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
internal/cmd/            → Cobra command implementations (root, env, worktree, deploy, db, migrate, backup, build, doctor, init)
internal/config/         → .drift.yaml loading, defaults, merging
internal/git/            → Git operations (repo, branch, worktree management)
internal/database/       → pg_dump/pg_restore wrappers
internal/supabase/       → Supabase CLI/API interactions (branches, functions, secrets, storage)
internal/ui/             → Terminal UI (colors, spinners, prompts, tables)
internal/xcode/          → xcconfig generation, version file management
pkg/shell/               → Shell command execution with context support
```

### Key Design Patterns

1. **Shell abstraction**: `pkg/shell` provides `Run()`, `RunInDir()`, `RunWithEnv()`, `RunWithTimeout()` - all return `*Result` with stdout, stderr, exit code, and duration. Use `Runner` interface for testability.

2. **Config loading**: `config.Load()` searches up directory tree for `.drift.yaml`/`.drift.yml`. `MergeWithDefaults()` fills missing values. Access paths via `cfg.GetFunctionsPath()`, etc.

3. **Git worktree naming**: `GetWorktreePath(project, branch, pattern)` uses `{project}-{branch}` pattern. Branch names are sanitized (slashes → hyphens).

4. **Environment mapping**: Git branches map to Supabase environments. `main`/`master` → production, `dev`/`development` → development, others → feature branches.

5. **Protected branches**: Defined in config, checked before destructive operations on production.

### Command Flow

Commands in `internal/cmd/` follow this pattern:
- `init()` registers flags and subcommands
- `Run` functions validate inputs, load config, execute operations
- UI feedback via `ui.Success()`, `ui.Warning()`, `ui.Error()`, spinners
- Confirmations via `ui.PromptYesNo()`, can be skipped with `--yes` flag

### Testing Approach

Tests create temporary git repositories via `setupTestRepo(t)` helper. For path comparisons on macOS, use `filepath.EvalSymlinks()` to handle `/var` → `/private/var` symlinks.
