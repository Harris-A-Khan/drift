# drift init

Initialize Drift in a project directory.

## Usage

```bash
drift init [flags]
```

## Description

The `init` command creates a `.drift.yaml` configuration file in your project root with sensible defaults. It detects your project structure and pre-configures paths for Supabase functions, migrations, and Xcode files.

It also prompts you to select and link a Supabase project, storing the project reference directly in `.drift.yaml`.

## Flags

| Flag | Description |
|------|-------------|
| `--force`, `-f` | Overwrite existing configuration |
| `--supabase-project`, `-s` | Supabase project name to link |
| `--skip-link` | Skip Supabase project linking |
| `--fallback-branch` | Seed `supabase.fallback_branch` in `.drift.local.yaml` |

## What It Does

1. Checks if already initialized (looks for `.drift.yaml`)
2. Detects project name from directory or `.xcodeproj`
3. Lists available Supabase projects and prompts for selection
4. Links the selected Supabase project
5. Creates `.drift.yaml` and `.drift.local.yaml`
6. Adds `.drift.local.yaml` to `.gitignore`

## Example

```bash
$ drift init

╔══════════════════════════════════════════════════════════════╗
║  Initialize Drift                                            ║
╚══════════════════════════════════════════════════════════════╝

───── Detected Settings
  Project Name:     MyApp
  Project Type:     ios

───── Configuration
? Project name: MyApp
? Project type: ios
? Select Supabase project: pause [gkhvtzjajeykbashnavj, us-east-2, active]

✓ Created .drift.yaml
  Linking Supabase project... ✓ Linked to Supabase project: pause

───── Next Steps
  1. Run 'drift env setup' to generate Config.xcconfig
  2. Add Config.xcconfig to your Xcode project
```

### Quick Init with Project Name

```bash
# Initialize and link by project name
drift init --supabase-project pause

# Skip linking (configure later)
drift init --skip-link

# Set local fallback branch during init
drift init --fallback-branch development
```

## Generated Configuration

The generated `.drift.yaml` includes:

```yaml
project:
  name: MyApp
  type: ios

supabase:
  project_ref: gkhvtzjajeykbashnavj
  project_name: pause
  functions_dir: supabase/functions
  migrations_dir: supabase/migrations
  protected_branches:
    - main
    - master

xcode:
  xcconfig_output: Config.xcconfig
  version_file: Version.xcconfig

backup:
  bucket: database-backups
```

The `project_ref` is automatically populated when you select a Supabase project during init, eliminating the need for a separate `.supabase-project-ref` file.

## See Also

- [Configuration](../config/drift-yaml.md)
- [Quick Start](../quickstart.md)
