<!-- drift-skill v1 -->
# Deploy

Deploy edge functions, secrets, or everything to a Supabase branch.

**User request:** $ARGUMENTS

## Operations

### Deploy everything
Run `drift deploy all` to deploy both edge functions and secrets to the resolved Supabase branch. This is the most common deployment command.

### Deploy edge functions only
Run `drift deploy functions` to deploy Supabase edge functions. Restricted functions (configured in `.drift.yaml`) are automatically skipped for the target environment.

### Deploy secrets only
Run `drift deploy secrets` to push secrets to the Supabase branch. Secrets are merged from:
1. `supabase.default_secrets` (baseline values)
2. `environments.<env>.secrets` (environment-specific overrides)
3. APNs keys (auto-discovered from `apple.push_key_pattern`)

Keys listed in `environments.<env>.skip_secrets` are excluded.

### Deploy with a specific branch target
Use `--branch <name>` to target a specific Supabase branch instead of the auto-resolved one.

## Confirmation Tiers
- **Production**: requires typing the branch name to confirm
- **Development**: requires yes/no confirmation
- **Feature**: no extra confirmation needed
- All can be bypassed with `--yes` (use with caution)

## Pre-deploy Checklist
1. Run `drift status` to confirm the target branch
2. Run `drift config show` to verify secrets configuration
3. Ensure migrations are applied before deploying functions that depend on schema changes
