# Drift

> Opinionated CLI for Supabase-backed iOS, macOS, and web development workflows

Drift standardizes and abstracts common development operations like environment switching, git worktree management, database operations, edge function deployment, and migration pushing into a single, cohesive CLI.

## Features

- **Environment Management** - Auto-detect git branches and generate configuration files (`Config.xcconfig` for iOS/macOS, `.env.local` for web)
- **Edge Function Management** - Deploy, compare, and manage Edge Functions across environments
- **Secrets Management** - View, compare, and copy secrets between Supabase branches
- **Database Operations** - Backup and restore with Supabase Storage integration
- **Git Worktrees** - Manage parallel development environments with full setup
- **Migrations** - Push and track database migrations across environments
- **CI/CD Ready** - Full support for GitHub Actions and other CI/CD pipelines

## Installation

### Homebrew (recommended for macOS/Linux)

```bash
brew tap Harris-A-Khan/tap
brew install drift
```

### Quick Install Script

```bash
curl -fsSL https://raw.githubusercontent.com/Harris-A-Khan/drift/main/install.sh | bash
```

### Direct Download

```bash
# macOS Apple Silicon (M1/M2/M3)
curl -L -o drift https://github.com/Harris-A-Khan/drift/releases/latest/download/drift-darwin-arm64
chmod +x drift && sudo mv drift /usr/local/bin/

# macOS Intel
curl -L -o drift https://github.com/Harris-A-Khan/drift/releases/latest/download/drift-darwin-amd64
chmod +x drift && sudo mv drift /usr/local/bin/

# Linux x86_64
curl -L -o drift https://github.com/Harris-A-Khan/drift/releases/latest/download/drift-linux-amd64
chmod +x drift && sudo mv drift /usr/local/bin/

# Linux ARM64
curl -L -o drift https://github.com/Harris-A-Khan/drift/releases/latest/download/drift-linux-arm64
chmod +x drift && sudo mv drift /usr/local/bin/
```

### Go Install

```bash
go install github.com/Harris-A-Khan/drift/cmd/drift@latest
```

> **Note:** Requires `$GOPATH/bin` in your PATH. Add to `~/.zshrc` or `~/.bashrc`:
> ```bash
> export PATH="$PATH:$(go env GOPATH)/bin"
> ```

### From Source

```bash
git clone https://github.com/Harris-A-Khan/drift.git
cd drift
make install    # Builds and installs to /usr/local/bin
```

## Quick Start

```bash
# Initialize drift in your project
drift init

# Check dependencies
drift doctor

# Generate config for current branch
drift env setup

# Deploy edge functions
drift deploy functions

# Create a new worktree for a feature (with full setup)
drift worktree create feat/new-ui --open

# Push migrations
drift migrate push
```

## Requirements

- **Required:**
  - Git
  - Supabase CLI (`brew install supabase/tap/supabase`)

- **For database operations:**
  - PostgreSQL (`brew install postgresql@16`)

- **Optional:**
  - fzf (for interactive selection)
  - VS Code (for opening worktrees)

## Commands Overview

| Command | Description |
|---------|-------------|
| `drift init` | Initialize drift in a project |
| `drift doctor` | Check system dependencies |
| `drift status` | Show project status overview |
| `drift env` | Environment management |
| `drift config` | Configuration management |
| `drift worktree` / `drift wt` | Git worktree management |
| `drift functions` | Edge Function management |
| `drift secrets` | Secrets management |
| `drift deploy` | Deployment operations |
| `drift db` | Database operations |
| `drift migrate` | Migration management |
| `drift backup` | Cloud backup management |
| `drift build` | Build helpers (Xcode) |
| `drift open` | Open Supabase dashboard |

## Environment Variables

| Variable | Description | Required For |
|----------|-------------|--------------|
| `SUPABASE_ACCESS_TOKEN` | Supabase access token for API access | CI/CD, secrets copy |
| `SUPABASE_DB_PASSWORD` | Database password for migrations | migrate push/history |
| `PROD_PASSWORD` | Production database password | db dump/push |
| `DEV_PASSWORD` | Development database password | db dump/push |

## CI/CD Usage

Drift works in GitHub Actions and other CI/CD pipelines:

```yaml
- name: Install Drift CLI
  run: |
    curl -fsSL https://raw.githubusercontent.com/Harris-A-Khan/drift/main/install.sh | bash

- name: Deploy Edge Functions
  env:
    SUPABASE_ACCESS_TOKEN: ${{ secrets.SUPABASE_ACCESS_TOKEN }}
  run: drift deploy functions -y
```

See the [CI/CD documentation](cicd.md) for complete setup instructions.

## Getting Help

```bash
drift --help
drift <command> --help
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--config <path>` | Use a specific config file |
| `--verbose, -v` | Verbose output (shows underlying commands) |
| `--yes, -y` | Skip confirmation prompts |
| `--no-color` | Disable colored output |
| `--version` | Show version |
