# drift branch

Manage Supabase branches.

## Usage

```bash
drift branch <subcommand> [flags]
```

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `list` | List all Supabase branches |
| `create` | Create a new Supabase branch |
| `delete` | Delete a Supabase branch |
| `status` | Show branch status and mapping |

## drift branch list

List all Supabase branches for the linked project.

```bash
$ drift branch list

╔══════════════════════════════════════════════════════════════╗
║  Supabase Branches                                           ║
╚══════════════════════════════════════════════════════════════╝

  NAME                  STATUS      TYPE
  main                  active      Production
  development           active      Development
  feature-new-ui        active      Feature
  feature-api-update    active      Feature

→ Total: 4 branches
```

## drift branch create

Create a new Supabase branch.

```bash
drift branch create <name> [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--from`, `-f` | Source branch to fork from |
| `--persistent` | Create as persistent (development) branch |

**Examples:**

```bash
# Create feature branch
drift branch create feature-new-ui

# Create from specific branch
drift branch create feature-new-ui --from development

# Create persistent development branch
drift branch create staging --persistent
```

## drift branch delete

Delete a Supabase branch.

```bash
drift branch delete <name> [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--force`, `-f` | Skip confirmation |

**Example:**

```bash
drift branch delete feature-old-experiment
```

## drift branch status

Show the mapping between git branches and Supabase branches.

```bash
$ drift branch status

╔══════════════════════════════════════════════════════════════╗
║  Branch Mapping                                              ║
╚══════════════════════════════════════════════════════════════╝

  Git Branch          Supabase Branch     Environment
  main                main                Production
  develop             development         Development
  feature/new-ui      feature-new-ui      Feature
  feature/api         (fallback)          Development

→ Current: feature/new-ui → feature-new-ui (Feature)
```

## Branch Types

| Type | Description | Persistence |
|------|-------------|-------------|
| Production | Default Supabase branch (usually `main`) | Permanent |
| Development | Persistent development branch | Permanent |
| Feature | Ephemeral branches for features | Temporary |

## Fallback Behavior

If no Supabase branch exists for your git branch, Drift falls back to:

1. Look for a branch with a similar name
2. Use the development branch
3. Use the production branch (last resort)

You'll see a warning when fallback is used:

```
⚠ No Supabase branch for 'feature/experiment', using fallback to development
```

## See Also

- [Worktree Management](worktree.md)
- [Environment Management](env.md)
