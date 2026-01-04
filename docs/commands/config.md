# drift config

View and modify drift configuration.

## Usage

```bash
drift config <subcommand>
```

## Subcommands

| Command | Description |
|---------|-------------|
| `show` | Show current configuration |
| `set-branch` | Set the Supabase branch override |
| `clear-branch` | Clear the Supabase branch override |

---

## drift config show

Display the current drift configuration and branch resolution.

```bash
drift config show
```

### Example

```bash
$ drift config show

╔══════════════════════════════════════════════════════════════╗
║  Drift Configuration                                         ║
╚══════════════════════════════════════════════════════════════╝

───── Project
  Name:          MyApp
  Type:          ios
  Config File:   /path/to/project/.drift.yaml

───── Supabase
  Project Ref:     abcdefghij
  Project Name:    my-project
  Override Branch: v2/migration

───── Current Branch Resolution
  Git Branch:      v2/migration-refinements
  Supabase Branch: v2/migration
  Environment:     Feature
  ℹ (using override)
```

---

## drift config set-branch

Set an override branch to use instead of automatic git branch detection.

```bash
drift config set-branch [supabase-branch]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `supabase-branch` | Optional. The Supabase branch name to use. If omitted, shows interactive selection. |

### Interactive Mode

When no branch is specified, drift will:

1. Fetch all available Supabase branches
2. Show branches similar to your current git branch (fuzzy matching)
3. Present an interactive selection menu

```bash
$ drift config set-branch

Fetching Supabase branches... ✓
Similar branches to 'v2/migration-refinements':
  • v2/migration (v2/migration)

? Select Supabase branch to use:
  > v2/migration [feature]
    development [development]
    main [production]

✓ Set override branch to 'v2/migration' (feature)
  All drift commands will now use this Supabase branch
```

### Direct Mode

Specify the branch directly:

```bash
$ drift config set-branch v2/migration

✓ Set override branch to 'v2/migration' (feature)
  All drift commands will now use this Supabase branch
```

### Fuzzy Matching

If the specified branch doesn't exist, drift suggests similar branches:

```bash
$ drift config set-branch v2/migrat

⚠ Branch 'v2/migrat' not found. Did you mean:
  • v2/migration
```

---

## drift config clear-branch

Remove the override branch, returning to automatic branch detection.

```bash
drift config clear-branch
```

### Example

```bash
$ drift config clear-branch

✓ Cleared override branch
  Drift will now use automatic branch detection based on git branch
```

---

## Branch Resolution

When no override is set, drift resolves Supabase branches automatically:

| Git Branch | Supabase Branch |
|------------|-----------------|
| `main` | Production branch (default) |
| `development` | Development branch (persistent) |
| Other branches | Match by name, fallback to development |

### Override Behavior

When `override_branch` is set in `.drift.yaml`:

- All drift commands use the override branch
- Command-line `--branch` flag still takes precedence
- The override is displayed in command output

### Common Use Cases

**Working on a feature branch without a Supabase branch:**
```bash
# Git branch: feature/new-auth
# No matching Supabase branch exists
drift config set-branch development
```

**Iterating on migrations in a child branch:**
```bash
# Git branch: v2/migration-refinements
# Want to use existing v2/migration Supabase branch
drift config set-branch v2/migration
```

**Testing against production (read-only):**
```bash
drift config set-branch main
drift env show  # View production config
drift config clear-branch  # Return to normal
```

## Configuration File

The override is stored in `.drift.yaml`:

```yaml
supabase:
  project_ref: abcdefghij
  project_name: my-project
  override_branch: v2/migration  # This field
```

Remove the `override_branch` line or use `drift config clear-branch` to disable.

## See Also

- [Configuration Reference](../config/drift-yaml.md)
- [drift env](env.md)
