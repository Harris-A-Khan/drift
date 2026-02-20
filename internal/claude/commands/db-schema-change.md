<!-- drift-skill v1 -->
# Database Schema Change Workflow

Walk the user through a complete database schema change using drift. This is a multi-step workflow that ensures safe migration creation, testing, and deployment.

**User request:** $ARGUMENTS

## Steps

### 1. Create the migration
Run `drift migrate new <descriptive-name>` to scaffold a new migration file. Help the user write the SQL based on their request.

### 2. Review the migration
Read the generated migration file and verify:
- The SQL is correct and safe
- Destructive operations (DROP, ALTER column type) have explicit user confirmation
- Indexes are added where needed for new columns used in queries

### 3. Test locally (if supabase CLI is available)
Run `drift migrate push --dry-run` if supported, or have the user review the SQL manually.

### 4. Commit the migration
Stage and commit the migration file with a descriptive message like: `db: add <table/column> migration`

### 5. Create or update the PR
Create a pull request or push to the existing branch so Supabase can create a preview branch.

### 6. Wait for the Supabase preview branch
Run `drift status` to check if a Supabase branch exists for the current git branch. If not, wait and retry. Explain that Supabase creates preview branches automatically from PRs.

### 7. Back up the target branch (if pushing to an existing branch)
Run `drift db push` to create a backup of the current database state before applying changes.

### 8. Set the branch target
Run `drift config set-branch` to point drift at the correct Supabase branch for this feature.

### 9. Push the migration
Run `drift migrate push` to apply the migration to the Supabase branch.

### 10. Deploy and verify
Run `drift deploy all` to deploy edge functions and secrets, then verify with `drift status`.

## Important Notes
- Always back up before destructive migrations
- Never run migrations directly against production — use the PR-based branch flow
- If a migration fails, check `drift migrate history` for the current state
