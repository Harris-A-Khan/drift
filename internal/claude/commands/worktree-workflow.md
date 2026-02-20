<!-- drift-skill v1 -->
# Worktree Workflow

Manage git worktrees for parallel feature development with drift.

**User request:** $ARGUMENTS

## Available Operations

### Create a worktree
Run `drift worktree create <branch-name>` to create a new git worktree with its own working directory. This allows working on multiple branches simultaneously without stashing.

### List worktrees
Run `drift worktree list` to see all active worktrees, their branches, and paths.

### Open a worktree
Run `drift worktree open <branch-name>` to navigate to an existing worktree. Use this to switch context to a different feature branch.

### Sync a worktree
Run `drift worktree sync` to pull the latest changes from the base branch into the current worktree.

### Clean up worktrees
Run `drift worktree cleanup` to remove worktrees for branches that have been merged or deleted.

### Delete a worktree
Run `drift worktree delete <branch-name>` to remove a specific worktree and optionally its branch.

## Tips
- Each worktree gets its own `.drift.local.yaml` so branch overrides are worktree-specific
- After creating a worktree, run `drift env setup` in it to configure the environment
- Use `drift status` in any worktree to see its branch resolution
