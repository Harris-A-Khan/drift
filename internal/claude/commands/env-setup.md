<!-- drift-skill v1 -->
# Environment Setup

Configure drift environment settings for the current branch.

**User request:** $ARGUMENTS

## Steps

### 1. Check current status
Run `drift status` to see the current git branch, linked Supabase branch, and environment classification (production/development/feature).

### 2. Set up the environment
Run `drift env setup` to pull credentials and configure the local environment from the matched Supabase branch.

If the automatic branch matching fails:
- Run `drift config set-branch` to interactively select a Supabase branch
- Or run `drift config set-branch <branch-name>` if the user knows the target

### 3. Verify the configuration
Run `drift env show` to display the resolved environment settings and confirm they are correct.

### 4. Branch override (if needed)
If the user wants to target a different Supabase branch than what git branch detection resolves:
- `drift config set-branch <branch>` — set a local override
- `drift config clear-branch` — return to automatic detection

Explain that overrides are stored in `.drift.local.yaml` and are developer-specific (not committed to git).

## Notes
- Production branches cannot be set as override targets (safety check)
- The fallback branch is used when no exact match is found for the git branch
- Use `drift config show` to see the full resolution chain
