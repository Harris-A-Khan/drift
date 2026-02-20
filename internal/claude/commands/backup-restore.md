<!-- drift-skill v1 -->
# Backup & Restore

Manage database backups, restores, and seed data with drift.

**User request:** $ARGUMENTS

## Operations

### Create a local backup
Run `drift db dump` to create a pg_dump backup of the current Supabase branch database. Backups are saved locally with timestamps.

### Push/restore a backup
Run `drift db push` to restore a backup to the target Supabase branch. This uses pg_restore under the hood.

### List backups
Run `drift db list` to show available local backups with their timestamps and sizes.

### Cloud backups
Run `drift backup list` to see cloud backups stored in Supabase storage.
Run `drift backup create` to create a cloud backup.

### Generate seed data
Run `drift db dump --seed` (if supported) or export specific tables to create reproducible seed data for development branches.

## Workflow: Safe Schema Migration
1. `drift db dump` — backup before making changes
2. Apply your migration
3. If something goes wrong: `drift db push` to restore the backup

## Notes
- Always create a backup before destructive operations
- Backups are branch-specific — verify the target with `drift status`
- Use `--branch` flag to target a specific Supabase branch
