# drift env

Manage environment configuration and Xcode xcconfig generation.

## Usage

```bash
drift env <subcommand> [flags]
```

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `show` | Show current environment info |
| `setup` | Generate Config.xcconfig for current branch |
| `switch` | Generate xcconfig for a specific Supabase branch |
| `validate` | Validate environment configuration |
| `diff` | Compare environments between branches |

## drift env show

Display the current environment configuration.

```bash
drift env show
```

**Output:**
```
╔══════════════════════════════════════════════════════════════╗
║  Environment Info                                            ║
╚══════════════════════════════════════════════════════════════╝

  Git Branch:       feature/new-ui
  Environment:      Feature
  Supabase Branch:  feature-new-ui
  Project Ref:      abcdefghij
  API URL:          https://abcdefghij.supabase.co

───── Xcconfig Status
  Config File:      Config.xcconfig
  Configured Env:   Feature
```

## drift env setup

Generate environment configuration for the current git branch.

- **Web projects:** Generates `.env.local`
- **iOS/macOS projects:** Generates `Config.xcconfig`

```bash
drift env setup [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--branch`, `-b` | Override Supabase branch selection |
| `--copy-env` | Copy custom variables from another worktree (interactive picker) |
| `--copy-custom-from` | Copy custom variables from a specific file path |
| `--build-server` | Also generate buildServer.json for sourcekit-lsp (iOS/macOS only) |
| `--scheme` | Xcode scheme to use for buildServer.json (requires --build-server) |

**What It Does:**

1. Detects current git branch
2. Links Supabase if not already linked (prompts for project ref if needed)
3. Resolves matching Supabase branch (or falls back to development)
4. Fetches API keys from Supabase
5. Generates the appropriate config file

**Example:**

```bash
$ drift env setup

  Resolving Supabase branch... ✓
  Fetching API keys... ✓
  Generating .env.local... ✓

  Environment:      Feature
  Supabase Branch:  feature-new-ui
  Project Ref:      abcdefghij
  Output:           .env.local
```

### Copying Custom Variables

When switching environments, you may have custom variables that aren't managed by drift (e.g., `STRIPE_KEY`, `ANALYTICS_ID`). Use `--copy-env` for an interactive picker or `--copy-custom-from` for a specific path:

```bash
# Interactive: pick from existing worktrees
drift env setup --copy-env

# Specific path
drift env setup --copy-custom-from ../main-project/.env.local
```

The `--copy-env` flag shows a list of worktrees that have the relevant config file (`.env.local` for web, `Config.xcconfig` for iOS/macOS) and lets you select one to copy from.

This copies all variables that appear **after** the drift-managed section, preserving your custom configuration.

### With buildServer.json (iOS/macOS):

```bash
# Auto-detect scheme based on environment
drift env setup --build-server

# Specify a specific scheme
drift env setup --build-server --scheme "MyApp (Development)"
```

This generates `buildServer.json` for sourcekit-lsp support in VS Code. By default, the scheme is auto-detected based on the current environment. Use `--scheme` to override this and specify exactly which Xcode scheme to use.

## drift env switch

Generate xcconfig for a specific Supabase branch, regardless of current git branch.

```bash
drift env switch <branch>
```

**Example:**

```bash
# Generate config for production while on a feature branch
drift env switch main
```

## drift env validate

Perform comprehensive validation of your environment configuration.

```bash
drift env validate
```

**Validation Checks:**

1. Config file exists and is valid YAML
2. Required Supabase credentials are set (`SUPABASE_URL`, `SUPABASE_ANON_KEY`)
3. Drift markers are intact (`=== DRIFT MANAGED ===`)
4. Configured Xcode schemes exist (if applicable)
5. DB_SCHEMA_VERSION matches latest migration (optional)

**Example Output (Success):**

```
╔══════════════════════════════════════════════════════════════╗
║  Environment Validation                                      ║
╚══════════════════════════════════════════════════════════════╝

───── Config File
  ✓ Config.xcconfig exists
  ✓ Drift markers intact

───── Supabase Credentials
  ✓ SUPABASE_URL set
  ✓ SUPABASE_ANON_KEY set
  ✓ SUPABASE_PROJECT_REF set

───── Xcode Schemes
  ✓ production: MyApp (Production)
  ✓ development: MyApp (Development)
  ✓ feature: MyApp (Debug)

✓ All validation checks passed
```

**Example Output (Errors):**

```
╔══════════════════════════════════════════════════════════════╗
║  Environment Validation                                      ║
╚══════════════════════════════════════════════════════════════╝

───── Config File
  ✓ Config.xcconfig exists
  ✗ Drift markers missing or corrupt

───── Supabase Credentials
  ✓ SUPABASE_URL set
  ✗ SUPABASE_ANON_KEY missing

⚠ 2 validation errors found
  Run 'drift env setup' to regenerate configuration
```

## drift env diff

Compare environment configuration between two branches.

```bash
drift env diff <branch1> <branch2>
```

Shows differences in:
- Supabase URL and Project Ref
- API keys (masked for security)
- Custom variables

**Example:**

```bash
$ drift env diff main development

╔══════════════════════════════════════════════════════════════╗
║  Environment Comparison                                      ║
╚══════════════════════════════════════════════════════════════╝

                    main                    development
  ─────────────────────────────────────────────────────────────
  Environment       Production              Development
  Project Ref       abcdefghij              klmnopqrst
  SUPABASE_URL      https://abc...          https://klm...
  SUPABASE_ANON_KEY eyJ...abc               eyJ...xyz

───── Custom Variables
  DEBUG_SWITCH      (not set)               true
  FEATURE_FLAG      enabled                 enabled

→ 3 differences found
```

## Environment Types

Drift recognizes three environment types:

| Environment | Criteria | Color |
|-------------|----------|-------|
| Production | Default Supabase branch | Red |
| Development | Persistent non-default branch | Yellow |
| Feature | Ephemeral branch | Green |

## Generated xcconfig

The generated `Config.xcconfig` contains:

```
// Config.xcconfig
// Auto-generated by drift - DO NOT EDIT

// Supabase Configuration
SUPABASE_URL = https://abcdefghij.supabase.co
SUPABASE_ANON_KEY = eyJ...
SUPABASE_PROJECT_REF = abcdefghij

// Environment
DRIFT_ENVIRONMENT = Feature
DRIFT_GIT_BRANCH = feature/new-ui
DRIFT_SUPABASE_BRANCH = feature-new-ui
```

## See Also

- [Xcode Integration](../guides/xcode.md)
- [Configuration](../config/drift-yaml.md)
