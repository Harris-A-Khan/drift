# .drift.local.yaml

The `.drift.local.yaml` file provides developer-specific configuration that is automatically gitignored.

## Purpose

Use `.drift.local.yaml` for settings that:

- Should NOT be committed to git
- Are specific to your local development environment
- Override values from the main `.drift.yaml`
- Contain personal preferences

## Location

The file should be placed in the same directory as your `.drift.yaml`:

```
project/
├── .drift.yaml           # Shared team config (committed)
├── .drift.local.yaml     # Personal config (gitignored)
└── ...
```

## Creating the File

### During Init

When running `drift init`, Drift creates `.drift.local.yaml` automatically:

```bash
drift init
```

This creates both files and automatically adds `.drift.local.yaml` to `.gitignore`.

### Manually

Create the file manually:

```yaml
# .drift.local.yaml - Local/developer-specific configuration (GITIGNORED)

supabase:
  override_branch: "feat-my-feature"
  fallback_branch: "development"

environments:
  development:
    secrets:
      API_BASE_URL: "https://dev-api.example.com"
      ENABLE_DEBUG_SWITCH: "true"
  production:
    skip_secrets:
      - ENABLE_DEBUG_SWITCH

apple:
  key_search_paths:
    - "../shared-keys"

device:
  default_device: "My iPhone"

preferences:
  verbose: true
  editor: "cursor"
  auto_open_worktree: true
```

## Available Settings

### supabase

```yaml
supabase:
  override_branch: "feat-my-feature"
  fallback_branch: "development"
```

| Field | Description |
|-------|-------------|
| `override_branch` | Force use of a specific Supabase branch regardless of git branch |
| `fallback_branch` | Default non-production fallback branch when no match exists |

The `override_branch` is useful when:
- Iterating on a feature branch but using an existing Supabase branch
- Testing against a specific environment
- Your git branch doesn't have a matching Supabase branch

Use `fallback_branch` to avoid repeatedly selecting a branch when no match exists.

### environments (local values)

Store branch/environment-specific secret values locally:

```yaml
environments:
  development:
    secrets:
      API_BASE_URL: "https://dev-api.example.com"
  production:
    secrets:
      API_BASE_URL: "https://api.example.com"
    skip_secrets:
      - ENABLE_DEBUG_SWITCH
```

These values override `.drift.yaml` values and are ideal for sensitive secrets.
`skip_secrets` can be used locally to prevent accidental pushes for specific environments.

### apple

```yaml
apple:
  key_search_paths:
    - "secrets"
    - "../shared-keys"
```

| Field | Description |
|-------|-------------|
| `key_search_paths` | Local override for APNs key search directories |

### device

```yaml
device:
  default_device: "My iPhone"
```

| Field | Description |
|-------|-------------|
| `default_device` | Your preferred test device (name or UDID) |

### preferences

```yaml
preferences:
  verbose: false
  editor: "cursor"
  auto_open_worktree: true
```

| Field | Description | Default |
|-------|-------------|---------|
| `verbose` | Show verbose output for all commands | `false` |
| `editor` | Default editor for open commands | `code` (VS Code) |
| `auto_open_worktree` | Open worktree in editor after creation | `false` |

## How Merging Works

When Drift loads configuration:

1. First loads `.drift.yaml` (main config)
2. Then loads `.drift.local.yaml` (local config)
3. Local values override main config values

This means you can:
- Keep all shared config in `.drift.yaml`
- Override specific values in `.drift.local.yaml`
- Not worry about merge conflicts in team settings

## Example Configurations

### Feature Branch Development

Working on a feature that needs to use a specific Supabase branch:

```yaml
supabase:
  override_branch: "feat-payments-v2"
  fallback_branch: "development"
```

### Verbose Output

Enable detailed logging for debugging:

```yaml
preferences:
  verbose: true
```

### Preferred Test Device

Always build to your specific device:

```yaml
device:
  default_device: "00008120-001234567890"  # UDID
```

### Custom Editor

Use a different editor than VS Code:

```yaml
preferences:
  editor: "cursor"  # or "vim", "zed", etc.
  auto_open_worktree: true
```

## Full Reference

```yaml
# .drift.local.yaml - Complete reference

# Supabase branch overrides
supabase:
  override_branch: "branch-name"  # Force specific Supabase branch
  fallback_branch: "development"  # Fallback when no match exists

# Local secret values (preferred for sensitive values)
environments:
  development:
    secrets:
      API_BASE_URL: "https://dev-api.example.com"
  production:
    skip_secrets:
      - ENABLE_DEBUG_SWITCH

# Local APNs key lookup overrides
apple:
  key_search_paths:
    - "secrets"
    - "../shared-keys"

# Device preferences
device:
  default_device: "Device Name or UDID"

# Developer preferences
preferences:
  verbose: false                 # Verbose output
  editor: "code"                 # Editor for open commands
  auto_open_worktree: false      # Auto-open worktrees after create
```

## Gitignore

The file is automatically added to `.gitignore` during `drift init`. If you need to add it manually:

```gitignore
# Drift local config (developer-specific)
.drift.local.yaml
```

## See Also

- [.drift.yaml Configuration](drift-yaml.md)
- [Environment Variables](environment.md)
- [Configuration Command](../commands/config.md)
