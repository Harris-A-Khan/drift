# Drift

> A CLI tool for Supabase-backed iOS, macOS, and web development workflows

Drift automates environment management, edge function deployment, database backups, and git worktree workflows for projects using Supabase as a backend.

## Features

- **Environment Management** - Auto-detect git branches and generate configuration files (`Config.xcconfig` for iOS/macOS, `.env.local` for web)
- **Edge Function Deployment** - Deploy functions with environment-aware secrets
- **Database Backups** - Backup and restore with Supabase Storage integration
- **Git Worktrees** - Manage parallel development environments with full setup
- **Branch Sync** - Keep Supabase branches in sync with git branches
- **Configuration Override** - Force use of a specific Supabase branch regardless of git branch

## Quick Example

```bash
# Initialize drift in your project
drift init

# Generate config for current branch
drift env setup

# Deploy edge functions
drift deploy functions

# Create a new worktree for a feature (with full setup)
drift worktree create feat/new-ui --open
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
