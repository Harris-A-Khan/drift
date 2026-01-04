# drift backup

Database backup and restore operations.

## Usage

```bash
drift backup <subcommand> [flags]
```

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `create` | Create a database backup |
| `restore` | Restore from a backup |
| `list` | List available backups |
| `upload` | Upload a backup to cloud storage |
| `download` | Download a backup from cloud storage |

## drift backup create

Create a backup of the database.

```bash
drift backup create [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--output`, `-o` | Output file path (default: timestamped file) |
| `--branch`, `-b` | Supabase branch to backup |
| `--schema-only` | Only backup schema, not data |

**Example:**

```bash
$ drift backup create

  Connecting to database... ✓
  Creating backup... ✓

  Output: backup-2024-01-15-143022.sql.gz
  Size: 2.4 MB
```

## drift backup restore

Restore a database from a backup file.

```bash
drift backup restore <file> [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--branch`, `-b` | Target Supabase branch |
| `--clean` | Drop existing objects before restore |

**Example:**

```bash
drift backup restore backup-2024-01-15-143022.sql.gz --branch development
```

## drift backup list

List available backups in cloud storage.

```bash
$ drift backup list

╔══════════════════════════════════════════════════════════════╗
║  Available Backups                                           ║
╚══════════════════════════════════════════════════════════════╝

───── Production Backups
  • backup-2024-01-15-143022.sql.gz (2.4 MB)
  • backup-2024-01-14-120000.sql.gz (2.3 MB)
  • latest.backup.gz → backup-2024-01-15-143022.sql.gz

───── Development Backups
  • backup-2024-01-15-100000.sql.gz (1.8 MB)

→ Total: 3 backups
```

## drift backup upload

Upload a backup file to Supabase Storage.

```bash
drift backup upload <file> <environment> [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `file` | Path to backup file |
| `environment` | Target environment: `prod` or `dev` |

**Example:**

```bash
drift backup upload backup-2024-01-15.sql.gz prod
```

## drift backup download

Download a backup from cloud storage.

```bash
drift backup download <environment> [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--output`, `-o` | Output file path |
| `--latest` | Download the latest backup (default) |
| `--file` | Specific backup file to download |

**Example:**

```bash
# Download latest production backup
drift backup download prod

# Download specific backup
drift backup download prod --file backup-2024-01-14-120000.sql.gz
```

## Backup Strategy

Recommended backup workflow:

```bash
# Daily: Create and upload production backup
drift backup create --branch main -o daily-backup.sql.gz
drift backup upload daily-backup.sql.gz prod

# Sync to development
drift backup download prod --latest
drift backup restore latest-prod.sql.gz --branch development
```

## See Also

- [Storage Setup](storage.md)
- [Database Backups Guide](../guides/backups.md)
