# drift storage

Manage cloud storage for backups.

## Usage

```bash
drift storage <subcommand> [flags]
```

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `setup` | Set up storage bucket for backups |
| `status` | Show storage bucket status |

## drift storage setup

Create the storage bucket and configure RLS policies.

```bash
drift storage setup
```

This is a one-time setup command that:

1. Creates the `database-backups` bucket in Supabase Storage
2. Sets up Row Level Security (RLS) policies
3. Creates the initial folder structure

**Prerequisites:**

- Supabase CLI installed and logged in
- Project linked or `PROD_PROJECT_REF` environment variable set

**Example:**

```bash
$ drift storage setup

╔══════════════════════════════════════════════════════════════╗
║  Storage Setup                                               ║
╚══════════════════════════════════════════════════════════════╝

  Bucket:       database-backups
  Project Ref:  abcdefghij

⚠ This will create a storage bucket and set up RLS policies
? Continue? [Y/n]

  Creating storage bucket... ✓
  Setting up RLS policies... ✓
  Creating folder structure... ✓

✓ Storage setup complete!

───── Bucket Structure
  database-backups/
    backups/
      prod/
      dev/
      metadata.json

───── Next Steps
  1. Test upload: drift backup upload <file> prod
  2. Test download: drift backup download prod
  3. Set up automated backups in CI/CD
```

## drift storage status

Show the current status of the backup storage bucket.

```bash
$ drift storage status

╔══════════════════════════════════════════════════════════════╗
║  Storage Status                                              ║
╚══════════════════════════════════════════════════════════════╝

  Bucket:       database-backups

───── Production Backups
  • backup-2024-01-15-143022.sql.gz
  • backup-2024-01-14-120000.sql.gz
→ Total: 2 backups

───── Development Backups
  • backup-2024-01-15-100000.sql.gz
→ Total: 1 backup
```

## RLS Policies

The setup creates the following RLS policies on `storage.objects`:

| Policy | Role | Permission |
|--------|------|------------|
| Upload backups | service_role | INSERT |
| Update backups | service_role | UPDATE |
| Delete backups | service_role | DELETE |
| Read backups | service_role | SELECT |
| Download backups | authenticated | SELECT |

## Bucket Configuration

The bucket name can be configured in `drift.yaml`:

```yaml
backup:
  bucket: database-backups
```

## See Also

- [Backup Operations](backup.md)
- [Database Backups Guide](../guides/backups.md)
