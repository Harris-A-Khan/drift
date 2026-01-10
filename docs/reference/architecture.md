# Architecture

This document describes the internal architecture of Drift.

## Project Structure

```
drift/
├── cmd/
│   └── drift/
│       └── main.go          # Entry point
├── internal/
│   ├── cmd/                  # Command implementations
│   │   ├── root.go          # Root command, global flags
│   │   ├── init.go          # drift init
│   │   ├── env.go           # drift env
│   │   ├── deploy.go        # drift deploy
│   │   ├── branch.go        # drift branch
│   │   ├── worktree.go      # drift worktree
│   │   ├── backup.go        # drift backup
│   │   ├── storage.go       # drift storage
│   │   └── version.go       # drift version
│   ├── config/              # Configuration management
│   │   └── config.go        # .drift.yaml parsing
│   ├── git/                 # Git operations
│   │   ├── git.go           # Core git functions
│   │   ├── branch.go        # Branch operations
│   │   └── worktree.go      # Worktree operations
│   ├── supabase/            # Supabase CLI wrapper
│   │   ├── client.go        # API client
│   │   ├── branches.go      # Branch management
│   │   ├── functions.go     # Edge functions
│   │   ├── secrets.go       # Secrets management
│   │   └── storage.go       # Storage operations
│   ├── xcode/               # Xcode integration
│   │   ├── xcconfig.go      # Config generation
│   │   └── version.go       # Version management
│   ├── database/            # Database operations
│   │   └── backup.go        # Backup/restore
│   └── ui/                  # Terminal UI
│       ├── colors.go        # Color utilities
│       ├── spinner.go       # Progress spinners
│       ├── prompt.go        # User prompts
│       └── table.go         # Table formatting
├── pkg/
│   └── shell/               # Shell command execution
│       └── shell.go
└── docs/                    # Documentation (this site)
```

## Core Packages

### internal/cmd

Command implementations using [Cobra](https://github.com/spf13/cobra).

Each command file defines:
- Command structure (`*cobra.Command`)
- Flag definitions
- `RunE` handler function

### internal/config

Configuration management:

```go
type Config struct {
    Project   ProjectConfig
    Supabase  SupabaseConfig
    Xcode     XcodeConfig
    Git       GitConfig
    APNS      APNSConfig
    Database  DatabaseConfig
    Backup    BackupConfig
}
```

Loads from `.drift.yaml` with sensible defaults.

### internal/git

Git operations wrapper:

- Branch management (create, delete, checkout)
- Worktree operations
- Status and diff queries
- Repository inspection

### internal/supabase

Supabase CLI wrapper:

- Project linking and status
- Branch management
- API key retrieval
- Function deployment
- Secret management
- Storage operations

### internal/xcode

Xcode project integration:

- xcconfig generation
- Version file management
- Scheme detection
- buildServer.json generation

### internal/ui

Terminal UI components:

- Colored output (via [fatih/color](https://github.com/fatih/color))
- Spinners (via [briandowns/spinner](https://github.com/briandowns/spinner))
- Prompts (via [manifoldco/promptui](https://github.com/manifoldco/promptui))
- Tables (via [olekukonko/tablewriter](https://github.com/olekukonko/tablewriter))

### pkg/shell

Shell command execution:

```go
type Result struct {
    Stdout   string
    Stderr   string
    ExitCode int
}

func Run(name string, args ...string) (*Result, error)
func CommandExists(name string) bool
```

## Data Flow

### Environment Setup

```
drift env setup
    │
    ├─► git.CurrentBranch()
    │       └─► git rev-parse --abbrev-ref HEAD
    │
    ├─► supabase.GetBranchInfo()
    │       ├─► supabase branches list
    │       └─► Match git branch to Supabase branch
    │
    ├─► supabase.GetAnonKey()
    │       └─► supabase projects api-keys
    │
    └─► xcode.Generate()
            └─► Write Config.xcconfig
```

### Deployment

```
drift deploy all
    │
    ├─► getDeployTarget()
    │       ├─► git.CurrentBranch()
    │       └─► supabase.GetBranchInfo()
    │
    ├─► supabase.ListFunctions()
    │       └─► Scan supabase/functions/
    │
    ├─► supabase.DeployFunction() (for each)
    │       └─► supabase functions deploy
    │
    └─► supabase.SetSecrets()
            └─► supabase secrets set
```

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/spf13/viper` | Configuration |
| `gopkg.in/yaml.v3` | YAML parsing |
| `github.com/fatih/color` | Terminal colors |
| `github.com/briandowns/spinner` | Progress spinners |
| `github.com/manifoldco/promptui` | Interactive prompts |
| `github.com/olekukonko/tablewriter` | Table formatting |

## Testing

Tests are organized by package:

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./internal/xcode/...

# Verbose output
go test ./... -v
```

Test files follow Go conventions: `*_test.go`

## Building

```bash
# Development build
go build -o drift ./cmd/drift

# Release build with version
go build -ldflags "-X main.version=1.0.0" -o drift ./cmd/drift

# Cross-compile
GOOS=darwin GOARCH=arm64 go build -o drift-darwin-arm64 ./cmd/drift
GOOS=linux GOARCH=amd64 go build -o drift-linux-amd64 ./cmd/drift
```
