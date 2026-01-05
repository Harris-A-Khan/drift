# Commands Overview

Drift provides a comprehensive set of commands for managing Supabase-backed iOS, macOS, and web projects.

## Command Structure

```
drift <command> [subcommand] [flags]
```

## Available Commands

| Command | Description |
|---------|-------------|
| `init` | Initialize drift in a project |
| `env` | Environment and xcconfig management |
| `deploy` | Edge function deployment |
| `branch` | Supabase branch management |
| `worktree` | Git worktree management |
| `backup` | Database backup operations |
| `storage` | Cloud storage setup |
| `version` | Version and build number management |

## Global Flags

| Flag | Description |
|------|-------------|
| `--help`, `-h` | Help for any command |
| `--version`, `-v` | Print version information |
| `--yes`, `-y` | Skip confirmation prompts |

## Common Workflows

### Daily Development

```bash
# Switch to a feature branch
git checkout feature/new-ui

# Update xcconfig for new environment
drift env setup

# Work on code...

# Deploy to feature branch environment
drift deploy all
```

### Release Workflow

```bash
# Increment build number
drift version bump

# Deploy to production
drift deploy all --branch main
```

### Multi-branch Development

```bash
# Create worktree for parallel development
drift worktree create feature/experiment

# Work in worktree, original branch untouched
cd ../project-feature-experiment

# When done, remove worktree
drift worktree remove feature/experiment
```
