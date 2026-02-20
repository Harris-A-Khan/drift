<!-- drift-skill v1 -->
# Project Initialization

Set up drift in a new or existing project.

**User request:** $ARGUMENTS

## Steps

### 1. Run diagnostics
Run `drift doctor` to check the development environment. Ensure required tools are installed (supabase CLI, git, etc.). Help fix any issues found.

### 2. Initialize drift
Run `drift init` to create the `.drift.yaml` configuration file. Guide the user through the interactive prompts (project name, type, Supabase linking).

### 3. Set up the environment
Run `drift env setup` to configure the development environment with credentials from the linked Supabase branch.

### 4. Verify the setup
Run `drift status` to confirm everything is connected and working correctly.

### 5. Check gitignore
Verify that `.drift.local.yaml` is in `.gitignore`. If not, add it — this file contains developer-specific overrides and should not be committed.

### 6. Summary
Show the user what was configured and suggest next steps:
- `drift deploy` to deploy edge functions
- `drift worktree create` to set up feature branches
- `drift db dump` to create a database backup
