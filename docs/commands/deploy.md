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

## drift deploy functions

Deploy all Edge Functions to the target environment.

```bash
drift deploy functions [--branch <branch>]
```

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

Set APNs and debug secrets for the target environment.

```bash
drift deploy secrets [--branch <branch>]
```

**What It Sets:**

- `APNS_TEAM_ID` - Apple Developer Team ID
- `APNS_BUNDLE_ID` - App Bundle Identifier
- `APNS_KEY_ID` - APNs Key ID
- `APNS_PRIVATE_KEY` - APNs Private Key (from .p8 file)
- `APNS_ENVIRONMENT` - "sandbox" or "production"
- `DEBUG_SWITCH` - Enabled for non-production environments

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
  • DEBUG_SWITCH

→ Total: 5 secrets
```

## Production Safeguards

When deploying to production, Drift requires confirmation:

```bash
$ drift deploy all

⚠ You are about to deploy to PRODUCTION!
? Continue? [y/N]
```

Use `--yes` or `-y` to skip confirmation (for CI/CD).

## See Also

- [CI/CD Setup](../guides/cicd.md)
- [Environment Management](env.md)
