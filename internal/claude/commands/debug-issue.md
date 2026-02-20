<!-- drift-skill v1 -->
# Debug Issue

Diagnose and fix common drift and Supabase issues.

**User request:** $ARGUMENTS

## Diagnostic Steps

### 1. Run doctor
Run `drift doctor` to check the development environment for issues. This verifies:
- Required CLI tools are installed (supabase, git, etc.)
- Configuration files are present and valid
- Supabase project is linked

### 2. Check status
Run `drift status` to see:
- Current git branch and Supabase branch resolution
- Environment classification
- Connection status

### 3. Review configuration
Run `drift config show` to display the full configuration including:
- Override and fallback branch settings
- Secrets configuration
- Project settings

### 4. Check migration state
Run `drift migrate history` to see applied migrations and detect any drift between local and remote state.

### 5. Review environment
Run `drift env show` to check environment variable resolution and credential configuration.

## Common Issues and Fixes

### "No Supabase branch found"
- The git branch may not have a matching Supabase preview branch
- Create a PR to trigger preview branch creation
- Or use `drift config set-branch` to manually target a branch

### "Migration failed"
- Check `drift migrate history` for the current state
- Review the migration SQL for syntax errors
- Ensure the target branch database is accessible

### "Secrets not deploying"
- Check `supabase.secrets_to_push` in `.drift.yaml`
- Verify `environments.<env>.skip_secrets` isn't excluding the key
- Run `drift config show` to see the merged secrets configuration

### "Wrong environment detected"
- Check `drift status` for the branch classification
- Use `drift config set-branch` to override if automatic detection is wrong
- Verify the Supabase branch properties (persistent vs ephemeral)
