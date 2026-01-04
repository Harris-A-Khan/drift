# drift init

Initialize Drift in a project directory.

## Usage

```bash
drift init [flags]
```

## Description

The `init` command creates a `drift.yaml` configuration file in your project root with sensible defaults. It detects your project structure and pre-configures paths for Supabase functions, migrations, and Xcode files.

## Flags

| Flag | Description |
|------|-------------|
| `--force`, `-f` | Overwrite existing configuration |

## What It Does

1. Checks if already initialized (looks for `drift.yaml`)
2. Detects project name from directory or `.xcodeproj`
3. Scans for existing Supabase and Xcode files
4. Creates `drift.yaml` with detected configuration

## Example

```bash
$ drift init

╔══════════════════════════════════════════════════════════════╗
║  Drift Initialization                                        ║
╚══════════════════════════════════════════════════════════════╝

  Project:          MyApp
  Functions Path:   supabase/functions
  Migrations Path:  supabase/migrations

✓ Created drift.yaml

Next steps:
  1. Review drift.yaml and adjust settings
  2. Run 'drift env setup' to generate xcconfig
  3. Run 'drift deploy status' to check deployment
```

## Generated Configuration

The generated `drift.yaml` includes:

```yaml
project:
  name: MyApp

supabase:
  functions_path: supabase/functions
  migrations_path: supabase/migrations

xcode:
  xcconfig_path: Config.xcconfig
  version_file: Version.xcconfig

git:
  protected_branches:
    - main
    - master
    - production

backup:
  bucket: database-backups
```

## See Also

- [Configuration](../config/drift-yaml.md)
- [Quick Start](../quickstart.md)
