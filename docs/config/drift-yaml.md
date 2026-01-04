# drift.yaml Configuration

The `drift.yaml` file configures Drift for your project.

## Location

Drift looks for `drift.yaml` (or `drift.yml`) in:

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
  pg_bin: /opt/homebrew/opt/postgresql@16/bin  # PostgreSQL binaries path
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
```

| Field | Description | Default |
|-------|-------------|---------|
| `project_ref` | Supabase project reference ID | Set by `drift init` |
| `project_name` | Supabase project display name | Set by `drift init` |
| `functions_dir` | Path to Edge Functions | `supabase/functions` |
| `migrations_dir` | Path to migrations | `supabase/migrations` |

The `project_ref` replaces the need for a separate `.supabase-project-ref` file.

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

```yaml
git:
  protected_branches:
    - main
    - master
    - production
```

Protected branches require confirmation for destructive operations.

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
  pg_bin: /opt/homebrew/opt/postgresql@16/bin
  pooler_host: aws-0-us-east-1.pooler.supabase.com
  pooler_port: 6543
```

| Field | Description | Default |
|-------|-------------|---------|
| `pg_bin` | PostgreSQL binaries path | `/opt/homebrew/opt/postgresql@16/bin` |
| `pooler_host` | Supabase pooler host | `aws-0-us-east-1.pooler.supabase.com` |
| `pooler_port` | Pooler port | `6543` |

### backup

```yaml
backup:
  bucket: database-backups
```

| Field | Description | Default |
|-------|-------------|---------|
| `bucket` | Supabase Storage bucket | `database-backups` |

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
