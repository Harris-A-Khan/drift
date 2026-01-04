# Database Backups

This guide covers database backup strategies with Drift.

## Overview

Drift provides tools to:

- Create database backups (pg_dump)
- Store backups in Supabase Storage
- Restore backups to any environment
- Sync data between environments

## Initial Setup

### 1. Set Up Storage

First, create the storage bucket:

```bash
drift storage setup
```

This creates:
- `database-backups` bucket
- RLS policies for secure access
- Folder structure (`prod/`, `dev/`)

### 2. Configure Database Access

Set up PostgreSQL tools:

```yaml
# drift.yaml
database:
  pg_bin: /opt/homebrew/opt/postgresql@16/bin
  pooler_host: aws-0-us-east-1.pooler.supabase.com
  pooler_port: 6543
```

Install PostgreSQL client:

```bash
brew install postgresql@16
```

## Creating Backups

### Manual Backup

```bash
# Backup current environment
drift backup create

# Backup specific branch
drift backup create --branch main

# Custom output path
drift backup create -o my-backup.sql.gz

# Schema only (no data)
drift backup create --schema-only
```

### Upload to Cloud

```bash
# Create and upload to production folder
drift backup create -o backup.sql.gz
drift backup upload backup.sql.gz prod

# Create and upload to development folder
drift backup upload backup.sql.gz dev
```

## Restoring Backups

### From Local File

```bash
# Restore to development
drift backup restore backup.sql.gz --branch development

# Clean restore (drop existing objects)
drift backup restore backup.sql.gz --branch development --clean
```

### From Cloud Storage

```bash
# Download latest production backup
drift backup download prod

# Download specific backup
drift backup download prod --file backup-2024-01-15.sql.gz

# Restore after download
drift backup restore latest-prod.sql.gz --branch development
```

## Sync Workflows

### Production → Development

Sync production data to development for testing:

```bash
# On production branch
drift backup create -o prod-sync.sql.gz

# Switch to development
git checkout develop
drift backup restore prod-sync.sql.gz --branch development
```

### Using Cloud Storage

```bash
# Upload from production
drift backup upload prod-sync.sql.gz prod

# Download and restore to development
drift backup download prod --latest
drift backup restore latest-prod.sql.gz --branch development
```

## Automated Backups

### Daily Backup Script

```bash
#!/bin/bash
# backup-daily.sh

DATE=$(date +%Y-%m-%d)
BACKUP_FILE="backup-${DATE}.sql.gz"

# Create backup
drift backup create -o "$BACKUP_FILE" --branch main

# Upload to cloud
drift backup upload "$BACKUP_FILE" prod

# Clean up local file
rm "$BACKUP_FILE"

echo "Backup completed: $BACKUP_FILE"
```

### GitHub Actions

```yaml
name: Daily Backup

on:
  schedule:
    - cron: '0 2 * * *'  # 2 AM daily

jobs:
  backup:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup
        run: |
          go install github.com/undrift/drift/cmd/drift@latest
          sudo apt-get install postgresql-client

      - name: Backup
        env:
          SUPABASE_ACCESS_TOKEN: ${{ secrets.SUPABASE_ACCESS_TOKEN }}
          PROD_PASSWORD: ${{ secrets.PROD_PASSWORD }}
        run: |
          drift backup create -o backup.sql.gz --branch main
          drift backup upload backup.sql.gz prod
```

## Backup Retention

Manage backup retention manually:

```bash
# List all backups
drift backup list

# Delete old backups via Supabase dashboard or CLI
supabase storage rm database-backups/backups/prod/old-backup.sql.gz
```

## Security Considerations

1. **Never commit passwords** - Use environment variables
2. **Encrypt sensitive backups** - Use GPG for extra security
3. **Limit access** - RLS policies restrict to service_role
4. **Audit access** - Enable Supabase logging

### Encrypted Backups

```bash
# Create encrypted backup
drift backup create -o backup.sql.gz
gpg --symmetric --cipher-algo AES256 backup.sql.gz
drift backup upload backup.sql.gz.gpg prod

# Restore encrypted backup
drift backup download prod --file backup.sql.gz.gpg
gpg --decrypt backup.sql.gz.gpg > backup.sql.gz
drift backup restore backup.sql.gz --branch development
```

## Troubleshooting

### Connection Issues

```bash
# Test database connection
PGPASSWORD=$PROD_PASSWORD psql -h aws-0-us-east-1.pooler.supabase.com \
  -p 6543 -U postgres.abcdefghij -d postgres -c "SELECT 1"
```

### Permission Denied

Ensure your database user has sufficient privileges:

```sql
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO postgres;
```

## See Also

- [Storage Setup](../commands/storage.md)
- [Backup Commands](../commands/backup.md)
- [CI/CD Setup](cicd.md)
