# drift worktree

Manage Git worktrees for parallel development.

## Usage

```bash
drift worktree <subcommand> [flags]
drift wt <subcommand> [flags]  # Short alias
```

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `list` | List all worktrees |
| `create` | Create a new worktree with full setup |
| `delete` | Delete a worktree |
| `open` | Open a worktree in VS Code, Finder, or Terminal |
| `path` | Print the absolute path to a worktree |
| `prune` | Clean stale worktree entries |
| `info` | Show detailed worktree info (ahead/behind, changes) |
| `cleanup` | Clean up merged worktrees interactively |
| `sync` | Interactive multi-select sync across worktrees |

## What Are Worktrees?

Git worktrees allow you to have multiple working directories for the same repository. This is useful for:

- Working on multiple features simultaneously
- Testing changes without stashing current work
- Reviewing PRs while keeping your work intact
- Running parallel builds

## drift worktree create

Create a new worktree for a branch with full setup.

```bash
drift worktree create [branch] [flags]
```

If no branch is specified, interactive mode prompts you to enter a branch name and select the base branch.

**Flags:**

| Flag | Description |
|------|-------------|
| `--from` | Base branch to create from (default: development) |
| `--open` | Open in VS Code after setup |
| `--no-setup` | Skip file copying and environment setup |

**What It Does:**

1. Creates a new directory alongside your main project
2. Checks out the specified branch (creating it if needed)
3. Copies configured files (.env, .p8 keys, etc.)
4. Generates environment config (.env.local for web, Config.xcconfig for iOS)
5. Optionally opens in VS Code

**Examples:**

```bash
# Interactive mode - prompts for branch name and base
drift worktree create

# Create worktree for a new feature branch
drift worktree create feat/new-feature

# Create and open in VS Code
drift worktree create feat/new-feature --open

# Create from main instead of development
drift worktree create hotfix/urgent --from main

# Just create worktree without setup (bare)
drift worktree create feat/quick-test --no-setup
```

**Default Path:**

Worktrees are created at:
```
../project-name-sanitized-branch-name
```

For example, if your project is `myapp` and branch is `feat/new-ui`:
```
../myapp-feat-new-ui
```

## drift worktree list

List all worktrees with their status.

```bash
drift worktree list
```

**Example Output:**

```
╔══════════════════════════════════════════════════════════════╗
║  Git Worktrees                                               ║
╚══════════════════════════════════════════════════════════════╝

  main         /path/to/project                   ← you are here
  development  /path/to/project-development
  feat/new-ui  /path/to/project-feat-new-ui

→ Total: 3 worktrees
```

## drift worktree delete

Delete a worktree.

```bash
drift worktree delete [branch] [flags]
```

If no branch is specified, shows an interactive picker.

**Flags:**

| Flag | Description |
|------|-------------|
| `--force`, `-f` | Force removal even with uncommitted changes |

**Example:**

```bash
# Interactive selection
drift worktree delete

# Delete specific worktree
drift worktree delete feat/new-ui

# Force delete with uncommitted changes
drift worktree delete feat/new-ui --force
```

## drift worktree open

Open a worktree in VS Code, Finder, or Terminal.

```bash
drift worktree open [branch] [flags]
```

If no branch is specified, shows an interactive picker.

**Flags:**

| Flag | Description |
|------|-------------|
| `--finder` | Open in Finder instead of VS Code |
| `--terminal` | Open in Terminal instead of VS Code |

**Example:**

```bash
# Interactive selection, open in VS Code
drift worktree open

# Open specific worktree
drift worktree open feat/new-ui

# Open in Finder
drift worktree open feat/new-ui --finder

# Open in Terminal
drift worktree open feat/new-ui --terminal
```

## drift worktree path

Print the absolute path to a worktree.

```bash
drift worktree path <branch>
```

Useful for scripting:

```bash
cd $(drift worktree path feat/new-ui)
```

## drift worktree prune

Remove stale worktree entries for worktrees that no longer exist on disk.

```bash
drift worktree prune
```

## drift worktree info

Show detailed information about a worktree.

```bash
drift worktree info [branch]
```

If no branch is specified, shows info for the current worktree.

**Information Shown:**

- Commits ahead/behind origin
- Uncommitted changes count
- Supabase branch mapping
- Environment (production/development/feature)

**Example Output:**

```
╔══════════════════════════════════════════════════════════════╗
║  Worktree Info: feat/new-feature                             ║
╚══════════════════════════════════════════════════════════════╝

  Path:           /path/to/project-feat-new-feature
  Branch:         feat/new-feature
  Environment:    Feature

───── Git Status
  Ahead:          3 commits
  Behind:         0 commits
  Uncommitted:    2 files modified

───── Supabase
  Branch:         feat-new-feature
  Project Ref:    abcdefghij
```

## drift worktree cleanup

Find and delete worktrees for branches that have been merged into main.

```bash
drift worktree cleanup
```

This command:

1. Detects branches merged into main/master
2. Shows which worktrees can be safely deleted
3. Prompts for confirmation before each deletion
4. Optionally deletes the remote branch as well

**Example:**

```bash
$ drift worktree cleanup

╔══════════════════════════════════════════════════════════════╗
║  Cleanup Merged Worktrees                                    ║
╚══════════════════════════════════════════════════════════════╝

Found 2 worktrees for merged branches:

  feat/old-feature      merged 3 days ago
  fix/resolved-bug      merged 1 week ago

? Delete worktree for feat/old-feature? [y/N] y
  ✓ Deleted worktree
? Also delete remote branch? [y/N] y
  ✓ Deleted remote branch

? Delete worktree for fix/resolved-bug? [y/N] n
  Skipped
```

## drift worktree sync

Interactively select worktrees to sync with their remote branches.

```bash
drift worktree sync
```

Shows all worktrees with checkboxes for selection. For each selected worktree, pulls the latest changes from origin.

**Example:**

```bash
$ drift worktree sync

╔══════════════════════════════════════════════════════════════╗
║  Sync Worktrees                                              ║
╚══════════════════════════════════════════════════════════════╝

Select worktrees to sync:
  [x] main           (2 behind)
  [x] development    (5 behind)
  [ ] feat/my-work   (0 behind, 3 uncommitted)
  [x] feat/other     (1 behind)

Syncing main...
  ✓ Pulled 2 commits

Syncing development...
  ✓ Pulled 5 commits

Syncing feat/other...
  ✓ Pulled 1 commit

✓ Synced 3 worktrees
```

## Typical Workflow

```bash
# Start a new feature (interactive)
drift worktree create
# → Enter branch name: payment-v2
# → Select format: feat/payment-v2
# → Select base: development

# Or directly
drift worktree create feat/payment-v2 --open

# Work on the feature in VS Code...
# Main branch remains untouched

# When done, clean up
drift worktree delete feat/payment-v2
```

## See Also

- [Environment Management](env.md)
- [Configuration](../config/drift-yaml.md)
