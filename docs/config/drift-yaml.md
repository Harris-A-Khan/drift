# .drift.yaml Configuration

The `.drift.yaml` file configures Drift for your project.

## Location

Drift looks for `.drift.yaml` (or `.drift.yml`) in:

1. Current directory
2. Parent directories (up to git root)

## Full Configuration Reference

```yaml
# Project metadata
project:
  name: MyApp                    # Project name (auto-detected from .xcodeproj)

# Supabase configuration
supabase:
  project_ref: abcdefghij               # Supabase project reference ID
  project_name: my-project              # Supabase project display name
  functions_dir: supabase/functions     # Path to edge functions
  migrations_dir: supabase/migrations   # Path to migrations
  override_branch: v2/migration         # Optional: override Supabase branch detection

# Xcode configuration
xcode:
  xcconfig_path: Config.xcconfig        # Output path for generated config
  version_file: Version.xcconfig        # Version file path
  schemes:                              # Optional scheme mappings
    production: MyApp-Prod
    development: MyApp-Dev
    feature: MyApp

# Git settings
git:
  protected_branches:                   # Branches that require confirmation
    - main
    - master
    - production

# APNs configuration
apns:
  team_id: XXXXXXXXXX                   # Apple Developer Team ID
  bundle_id: com.example.myapp          # App Bundle ID
  key_pattern: "AuthKey_*.p8"           # Pattern to find .p8 key file
  environment: sandbox                  # sandbox or production

# Database settings
database:
  pooler_host: aws-0-us-east-1.pooler.supabase.com
  pooler_port: 6543

# Backup configuration
backup:
  bucket: database-backups              # Supabase Storage bucket name
```

## Section Details

### project

```yaml
project:
  name: MyApp
```

| Field | Description | Default |
|-------|-------------|---------|
| `name` | Project name | Detected from `.xcodeproj` |

### supabase

```yaml
supabase:
  project_ref: abcdefghij
  project_name: my-project
  functions_dir: supabase/functions
  migrations_dir: supabase/migrations
  protected_branches:
    - main
    - master
  override_branch: v2/migration
```

| Field | Description | Default |
|-------|-------------|---------|
| `project_ref` | Supabase project reference ID | Set by `drift init` |
| `project_name` | Supabase project display name | Set by `drift init` |
| `functions_dir` | Path to Edge Functions | `supabase/functions` |
| `migrations_dir` | Path to migrations | `supabase/migrations` |
| `protected_branches` | Branches requiring confirmation | `["main", "master"]` |
| `override_branch` | Force use of specific Supabase branch | (none) |

The `project_ref` replaces the need for a separate `.supabase-project-ref` file.

#### Branch Resolution

By default, drift resolves Supabase branches as follows:

1. **main** git branch → production Supabase branch
2. **development** git branch → development Supabase branch
3. **Other branches** → find matching Supabase branch by name, fallback to development

#### Override Branch

The `override_branch` field forces drift to use a specific Supabase branch instead of automatic detection. This is useful when:

- You're iterating on migrations in a child branch but want to use an existing Supabase branch
- You want to test against a specific environment
- No Supabase branch exists for your current git branch

Set the override interactively:
```bash
drift config set-branch           # Interactive selection
drift config set-branch v2/migration  # Direct set
drift config clear-branch         # Remove override
```

When an override is active, drift will display:
```
Override: using v2/migration instead of v2/migration-refinements
```

### xcode

```yaml
xcode:
  xcconfig_path: Config.xcconfig
  version_file: Version.xcconfig
  schemes:
    production: MyApp-Prod
    development: MyApp-Dev
    feature: MyApp
```

| Field | Description | Default |
|-------|-------------|---------|
| `xcconfig_path` | Generated config output | `Config.xcconfig` |
| `version_file` | Version info file | `Version.xcconfig` |
| `schemes` | Environment to scheme mapping | Auto-detected |

### git

> **Note:** Protected branches are now configured under `supabase.protected_branches`.

Protected branches require confirmation for destructive operations like `drift migrate push`.

### apns

```yaml
apns:
  team_id: XXXXXXXXXX
  bundle_id: com.example.myapp
  key_pattern: "AuthKey_*.p8"
  environment: sandbox
```

| Field | Description | Required |
|-------|-------------|----------|
| `team_id` | Apple Developer Team ID | Yes |
| `bundle_id` | App Bundle Identifier | Yes |
| `key_pattern` | Glob pattern for .p8 file | Yes |
| `environment` | `sandbox` or `production` | No (auto-detected) |

### database

```yaml
database:
  pooler_host: aws-0-us-east-1.pooler.supabase.com
  pooler_port: 6543
```

| Field | Description | Default |
|-------|-------------|---------|
| `pooler_host` | Supabase pooler host | `aws-0-us-east-1.pooler.supabase.com` |
| `pooler_port` | Pooler port | `6543` |

> **Note:** PostgreSQL binaries (`pg_dump`, `pg_restore`) must be in your PATH. Install with `brew install postgresql@16` and add to your shell profile: `export PATH="/opt/homebrew/opt/postgresql@16/bin:$PATH"`

### backup

```yaml
backup:
  bucket: database-backups
```

| Field | Description | Default |
|-------|-------------|---------|
| `bucket` | Supabase Storage bucket | `database-backups` |

### environments

Configure environment-specific settings for production and development:

```yaml
environments:
  production:
    secrets:
      APNS_KEY_ID: "KEY_PROD_123"
      STRIPE_KEY: "sk_live_xxx"
      CUSTOM_SECRET: "value"
    push_key: "AuthKey_PROD.p8"
  development:
    secrets:
      APNS_KEY_ID: "KEY_DEV_456"
      STRIPE_KEY: "sk_test_xxx"
    push_key: "AuthKey_DEV.p8"
```

| Field | Description |
|-------|-------------|
| `secrets` | Key-value map of secrets specific to this environment |
| `push_key` | APNs .p8 key file for this environment |

When running `drift deploy secrets`, the appropriate environment-specific secrets are automatically used.

### functions

Configure function deployment behavior:

```yaml
functions:
  restricted:
    - name: "dangerous-function"
      environments: ["production"]
    - name: "test-helper"
      environments: ["production", "development"]
```

| Field | Description |
|-------|-------------|
| `restricted` | List of functions with deployment restrictions |
| `restricted[].name` | Function name (directory name in supabase/functions) |
| `restricted[].environments` | Environments where this function should NOT be deployed |

## Minimal Configuration

The minimum required configuration:

```yaml
project:
  name: MyApp
```

All other values use sensible defaults.

## See Also

- [Environment Variables](environment.md)
- [Quick Start](../quickstart.md)
