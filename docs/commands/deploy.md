# drift deploy

Deploy Edge Functions and manage secrets.

## Usage

```bash
drift deploy <subcommand> [flags]
```

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `functions` | Deploy edge functions only |
| `secrets` | Set APNs and other secrets |
| `all` | Deploy functions and set secrets |
| `status` | Show deployment status |
| `list-secrets` | List secrets on target environment |

## Common Flags

| Flag | Description |
|------|-------------|
| `--branch`, `-b` | Target Supabase branch (auto-detected from git) |
| `--fallback-branch` | Fallback branch when no exact match exists |

## drift deploy functions

Deploy all Edge Functions to the target environment.

```bash
drift deploy functions [--branch <branch>]
```

If no exact branch exists, Drift resolves fallback in this order:
1. `--fallback-branch`
2. `supabase.fallback_branch` from `.drift.local.yaml`
3. Interactive non-production selection

**Example:**

```bash
$ drift deploy functions

╔══════════════════════════════════════════════════════════════╗
║  Deploy Edge Functions                                       ║
╚══════════════════════════════════════════════════════════════╝

  Environment:      Feature
  Supabase Branch:  feature-new-ui
  Project Ref:      abcdefghij

→ Found 3 functions to deploy
  Deploying hello-world... ✓
  Deploying send-notification... ✓
  Deploying process-payment... ✓

✓ Successfully deployed 3 functions
```

## drift deploy secrets

Set configured secrets for the target environment.

```bash
drift deploy secrets [--branch <branch>] [--key-search-dir <path>...]
```

**What It Sets:**

- `APNS_TEAM_ID` - Apple Developer Team ID
- `APNS_BUNDLE_ID` - App Bundle Identifier
- `APNS_KEY_ID` - APNs Key ID
- `APNS_PRIVATE_KEY` - APNs Private Key (from .p8 file)
- `APNS_ENVIRONMENT` - "sandbox" or "production"
- Any configured keys from `environments.<env>.secrets`

If `supabase.secrets_to_push` is set in `.drift.yaml`, only those keys are pushed.
Use `.drift.local.yaml` for secret values.

## drift deploy all

Deploy functions and set all secrets in one command.

```bash
drift deploy all [--branch <branch>]
```

## drift deploy status

Show current deployment status without making changes.

```bash
$ drift deploy status

╔══════════════════════════════════════════════════════════════╗
║  Deployment Status                                           ║
╚══════════════════════════════════════════════════════════════╝

  Git Branch:       feature/new-ui
  Environment:      Feature
  Supabase Branch:  feature-new-ui
  Project Ref:      abcdefghij

───── Edge Functions
  • hello-world
  • send-notification
  • process-payment
→ Total: 3 functions
```

## drift deploy list-secrets

List all secrets configured on the target environment.

```bash
$ drift deploy list-secrets

╔══════════════════════════════════════════════════════════════╗
║  Secrets                                                     ║
╚══════════════════════════════════════════════════════════════╝

  Environment:      Feature
  Supabase Branch:  feature-new-ui
  Project Ref:      abcdefghij

───── Configured Secrets
  • APNS_TEAM_ID
  • APNS_BUNDLE_ID
  • APNS_KEY_ID
  • APNS_PRIVATE_KEY
  • API_BASE_URL

→ Total: 5 secrets
```

## Per-Environment Secrets

Configure environment-specific secrets in `.drift.yaml`:

```yaml
environments:
  production:
    secrets:
      STRIPE_KEY: "sk_live_xxx"
    push_key: "AuthKey_PROD.p8"
  development:
    secrets:
      STRIPE_KEY: "sk_test_xxx"
    push_key: "AuthKey_DEV.p8"
```

When deploying secrets, Drift automatically uses the appropriate values for each environment.

**Example:**

```bash
$ drift deploy secrets

╔══════════════════════════════════════════════════════════════╗
║  Deploy Secrets                                              ║
╚══════════════════════════════════════════════════════════════╝

  Environment:      Development
  Supabase Branch:  development

───── Environment-Specific Secrets
  Setting APNS_KEY_ID... ✓
  Setting STRIPE_KEY... ✓

✓ Successfully set configured secrets
```

## Function Restrictions

Prevent certain functions from being deployed to specific environments:

```yaml
functions:
  restricted:
    - name: "dangerous-migration"
      environments: ["production"]
    - name: "test-helper"
      environments: ["production", "development"]
```

When a restricted function would be deployed, Drift shows a warning and skips it:

```bash
$ drift deploy functions

  Deploying hello-world... ✓
  Skipping dangerous-migration (blocked in production)
  Deploying send-notification... ✓

⚠ 1 function skipped due to environment restrictions
```

## Production Safeguards

When deploying to production (or protected branches), Drift requires strict confirmation.
Deploying to development also requires confirmation.

```bash
$ drift deploy all

⚠ You are about to deploy to PRODUCTION!
Type 'yes' to confirm: _
```

Use `--yes` or `-y` to skip confirmation (for CI/CD).

### Enhanced Protection

For destructive operations on production, Drift may require typing "yes" to confirm:

```bash
$ drift migrate push

⚠ You are about to push migrations on PRODUCTION!
Type 'yes' to confirm: _
```

## See Also

- [CI/CD Setup](../guides/cicd.md)
- [Environment Management](env.md)
