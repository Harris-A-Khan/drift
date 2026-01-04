# Drift

> A CLI tool for Supabase-backed iOS/macOS development workflows

Drift automates environment management, edge function deployment, database backups, and git worktree workflows for projects using Supabase as a backend.

## Features

- **Environment Management** - Auto-detect git branches and generate Xcode configuration
- **Edge Function Deployment** - Deploy functions with environment-aware secrets
- **Database Backups** - Backup and restore with Supabase Storage integration
- **Git Worktrees** - Manage parallel development environments
- **Branch Sync** - Keep Supabase branches in sync with git branches

## Quick Example

```bash
# Initialize drift in your project
drift init

# Generate xcconfig for current branch
drift env setup

# Deploy edge functions
drift deploy functions

# Create a new worktree for a feature
drift worktree create feature/new-ui
```

## Installation

```bash
# Using Go
go install github.com/undrift/drift/cmd/drift@latest

# Or build from source
git clone https://github.com/Harris-A-Khan/drift.git
cd drift
go build -o drift ./cmd/drift
```

## Requirements

- Go 1.21+
- Supabase CLI installed and authenticated
- Git repository

## Getting Help

```bash
drift --help
drift <command> --help
```
