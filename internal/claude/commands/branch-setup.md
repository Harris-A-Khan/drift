<!-- drift-skill v1 -->
# Branch Setup

Set up a new Supabase feature branch with data, functions, and secrets so it works like production.

**User request:** $ARGUMENTS

## When to use this

When a PR creates a new Supabase branch (because it has database migrations), that branch starts empty — no data, no edge functions, no secrets. This skill sets it up.

## Step-by-step workflow

### 1. Check current state
Run `drift status` to see the current branch, its Supabase mapping, and whether it's a new feature branch.

### 2. Dump source database (optional)
Only needed if there isn't already a recent backup in the `backups/` directory. Check with `drift db list` first.

- **Default to development** — run `drift db dump dev` unless the user specifically asks for a production dump.
- Production dumps are rarely needed and require extra confirmation. Only use `drift db dump prod` when explicitly requested.
- If a recent backup already exists (check timestamps with `drift db list`), skip the dump and use the existing one.

### 3. Push database to the feature branch
Run `drift db push` to restore the backup to the new feature branch. This is interactive — it will:
- Auto-detect the current Supabase branch
- Show available backups and suggest the most recent one
- Ask for confirmation before replacing the target database

### 4. Apply pending migrations
Run `drift migrate push` to apply any migrations that exist in the codebase but haven't been applied to the branch yet. This ensures the schema matches what the PR expects.

### 5. Deploy functions and secrets
Run `drift deploy all` to deploy edge functions and secrets in one step. This handles:
- All edge functions (respecting any restrictions in `.drift.yaml`)
- All secrets (APNs keys, environment secrets, default secrets)

## Quick version (for experienced users)
If the user just wants it done fast:
```
drift db list                  # Check for existing backups
drift db dump dev              # Only if no recent backup exists
drift db push                  # Push backup to feature branch
drift migrate push             # Apply pending migrations
drift deploy all               # Deploy functions + secrets
```

## Notes
- Always dump **dev** by default, not prod. Dev has the same schema and representative data.
- If `drift db push` shows "Expected backup dev.backup not found", it will offer to select from available backups — pick the most recent one.
- Each step is interactive and confirms before making changes, so it's safe to run sequentially.
- After setup, run `drift status` to verify everything is healthy.
