# drift worktree

Manage Git worktrees for parallel development.

## Usage

```bash
drift worktree <subcommand> [flags]
```

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `list` | List all worktrees |
| `create` | Create a new worktree |
| `remove` | Remove a worktree |
| `goto` | Open a worktree directory |

## What Are Worktrees?

Git worktrees allow you to have multiple working directories for the same repository. This is useful for:

- Working on multiple features simultaneously
- Testing changes without stashing current work
- Reviewing PRs while keeping your work intact
- Running parallel builds

## drift worktree list

List all worktrees with their status.

```bash
$ drift worktree list

╔══════════════════════════════════════════════════════════════╗
║  Git Worktrees                                               ║
╚══════════════════════════════════════════════════════════════╝

  PATH                              BRANCH              STATUS
  /path/to/project                  main                (main)
  /path/to/project-feature-ui       feature/new-ui
  /path/to/project-feature-api      feature/api-update

→ Total: 3 worktrees
```

## drift worktree create

Create a new worktree for a branch.

```bash
drift worktree create <branch> [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--path`, `-p` | Custom path for the worktree |
| `--base`, `-b` | Base branch to create from |

**Examples:**

```bash
# Create worktree from current branch
drift worktree create feature/new-ui

# Create from a specific base branch
drift worktree create feature/new-ui --base develop

# Specify custom path
drift worktree create feature/new-ui --path ~/projects/myapp-ui
```

**What It Does:**

1. Creates a new directory alongside your main project
2. Checks out the specified branch (creating it if needed)
3. Sets up the worktree with a clean working directory
4. Runs `drift env setup` to configure the environment

**Default Path:**

By default, worktrees are created at:
```
../project-name-sanitized-branch-name
```

For example:
```
../myapp-feature-new-ui
```

## drift worktree remove

Remove a worktree.

```bash
drift worktree remove <branch> [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--force`, `-f` | Force removal even with uncommitted changes |

**Example:**

```bash
# Remove worktree
drift worktree remove feature/new-ui

# Force remove with uncommitted changes
drift worktree remove feature/new-ui --force
```

## drift worktree goto

Open a worktree directory in your shell or editor.

```bash
drift worktree goto <branch>
```

## Worktree Workflow

```bash
# Start a new feature
drift worktree create feature/payment-v2

# Navigate to it
cd ../myapp-feature-payment-v2

# Work on the feature...
# Main branch remains untouched

# Set up environment for this worktree
drift env setup

# Deploy to feature environment
drift deploy all

# When done, clean up
cd ../myapp
drift worktree remove feature/payment-v2
```

## See Also

- [Branch Management](branch.md)
- [Environment Management](env.md)
