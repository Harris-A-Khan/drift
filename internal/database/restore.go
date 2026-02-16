package database

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/undrift/drift/pkg/shell"
)

// RestoreOptions holds options for database restore.
type RestoreOptions struct {
	Host       string
	Port       int
	Database   string
	User       string
	Password   string
	InputFile  string
	CleanFirst bool
	NoOwner    bool
	SingleTxn  bool
	Jobs       int // Number of parallel jobs

	// AuthCopyTables optionally overrides which auth.* tables are copied from
	// plain SQL backups during preprocessing. If empty, Drift auto-detects
	// auth tables with INSERT privilege on the target branch.
	AuthCopyTables []string

	// CopyAllInsertableTables switches plain SQL restore preprocessing from
	// "safe scope" (public + allowed auth + schema_migrations) to an
	// "all insertable tables" scope based on target INSERT privileges.
	CopyAllInsertableTables bool
}

// DefaultRestoreOptions returns default restore options.
func DefaultRestoreOptions() RestoreOptions {
	return RestoreOptions{
		Database:   "postgres",
		User:       "postgres",
		Port:       5432,
		NoOwner:    true,
		CleanFirst: true,
		Jobs:       4,
	}
}

// Restore restores a database from a backup file.
func Restore(opts RestoreOptions) error {
	// Check if input file exists
	if _, err := os.Stat(opts.InputFile); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", opts.InputFile)
	}

	// Auto-detect format: plain SQL files start with text, custom format starts with "PGDMP"
	isPlainSQL, err := isPlainSQLFormat(opts.InputFile)
	if err != nil {
		return fmt.Errorf("could not detect backup format: %w", err)
	}

	if isPlainSQL {
		return restoreSQL(opts)
	}

	return restoreCustom(opts)
}

// isPlainSQLFormat checks if a backup file is plain SQL (text) vs custom (binary) format.
func isPlainSQLFormat(filename string) (bool, error) {
	file, err := os.Open(filename)
	if err != nil {
		return false, err
	}
	defer file.Close()

	// Read first 5 bytes to check for PGDMP magic header
	header := make([]byte, 5)
	n, err := file.Read(header)
	if err != nil {
		return false, err
	}

	// Custom format starts with "PGDMP"
	if n >= 5 && string(header) == "PGDMP" {
		return false, nil
	}

	// Plain SQL format starts with text (usually "--" comments or SQL statements)
	// Check if first byte is printable ASCII
	if n > 0 && (header[0] == '-' || header[0] == 'S' || header[0] == '\n' || header[0] == ' ') {
		return true, nil
	}

	// Default to custom format if uncertain
	return false, nil
}

// restoreCustom restores from a custom format backup using pg_restore.
func restoreCustom(opts RestoreOptions) error {
	pgRestore, err := findPGTool("pg_restore")
	if err != nil {
		return err
	}

	args := []string{
		"-h", opts.Host,
		"-p", fmt.Sprintf("%d", opts.Port),
		"-U", opts.User,
		"-d", opts.Database,
	}

	if opts.CleanFirst {
		args = append(args, "-c")
	}

	if opts.NoOwner {
		args = append(args, "-O")
	}

	if opts.SingleTxn {
		args = append(args, "-1")
	}

	if opts.Jobs > 1 {
		args = append(args, "-j", fmt.Sprintf("%d", opts.Jobs))
	}

	args = append(args, opts.InputFile)

	env := map[string]string{
		"PGPASSWORD": opts.Password,
	}

	result, err := shell.RunWithEnv(env, pgRestore, args...)
	if err != nil {
		// pg_restore often returns non-zero even on success (e.g., for "already exists" errors)
		// Only fail if there are actual error messages
		if result.Stderr != "" && result.ExitCode != 0 {
			return fmt.Errorf("pg_restore failed: %s", result.Stderr)
		}
	}

	return nil
}

// restoreSQL restores from a plain SQL file using psql.
func restoreSQL(opts RestoreOptions) error {
	psql, err := findPGTool("psql")
	if err != nil {
		return err
	}

	allowedAuthTables := map[string]bool{}
	allowedAllTables := map[string]bool{}

	if opts.CopyAllInsertableTables {
		allowedAllTables, err = resolveAllInsertableCopyTables(opts)
		if err != nil {
			return fmt.Errorf("failed to resolve insertable table scope: %w", err)
		}
	} else {
		allowedAuthTables, err = resolveAllowedAuthCopyTables(opts)
		if err != nil {
			return fmt.Errorf("failed to resolve auth copy tables: %w", err)
		}
	}

	// Preprocess the backup file to:
	// 1. Remove Supabase guard metacommands (\restrict/\unrestrict)
	// 2. Keep only safe data sync statements (COPY + sequence setval) for:
	//    - public.*
	//    - auth.* tables with INSERT privileges on target
	//    - supabase_migrations.schema_migrations
	// 3. Add session_replication_role = replica to bypass trigger/constraint side effects
	processedFile, err := preprocessBackupFileWithScope(opts.InputFile, allowedAuthTables, allowedAllTables)
	if err != nil {
		return fmt.Errorf("failed to preprocess backup: %w", err)
	}
	defer os.Remove(processedFile) // Clean up temp file

	args := []string{
		"-h", opts.Host,
		"-p", fmt.Sprintf("%d", opts.Port),
		"-U", opts.User,
		"-d", opts.Database,
		"-v", "ON_ERROR_STOP=1",
		"-f", processedFile,
	}

	if opts.SingleTxn {
		args = append(args, "-1")
	}

	env := map[string]string{
		"PGPASSWORD": opts.Password,
	}

	result, err := shell.RunWithEnv(env, psql, args...)
	if err != nil || result.ExitCode != 0 {
		errMsg := result.Stderr
		if errMsg == "" {
			if err != nil {
				errMsg = err.Error()
			} else {
				errMsg = fmt.Sprintf("psql exited with code %d", result.ExitCode)
			}
		}
		return fmt.Errorf("psql restore failed: %s", errMsg)
	}

	return nil
}

// preprocessBackupFile creates a modified copy of the backup file that:
// 1. Removes \restrict/\unrestrict lines (Supabase psql guard metacommands)
// 2. Keeps only safe data sync statements from full backups:
//   - COPY public.*
//   - COPY auth.* (limited to insertable tables on target)
//   - COPY supabase_migrations.schema_migrations
//   - SELECT pg_catalog.setval(...) for public/supabase_migrations sequences
//
// 3. Inserts TRUNCATE ... CASCADE for each copied table before first COPY
// 4. Wraps content with SET session_replication_role = replica
func preprocessBackupFile(inputFile string, allowedAuthCopyTables map[string]bool) (string, error) {
	return preprocessBackupFileWithScope(inputFile, allowedAuthCopyTables, nil)
}

func preprocessBackupFileWithScope(inputFile string, allowedAuthCopyTables, allowedAllTables map[string]bool) (string, error) {
	input, err := os.Open(inputFile)
	if err != nil {
		return "", err
	}
	defer input.Close()

	// Create temp file for processed output
	tempFile, err := os.CreateTemp("", "drift-restore-*.sql")
	if err != nil {
		return "", err
	}

	// Write header to disable triggers during restore
	// This prevents trigger-related errors (e.g., handle_new_user trigger on auth.users)
	tempFile.WriteString("-- Drift: Disable triggers during restore\n")
	tempFile.WriteString("SET session_replication_role = replica;\n\n")

	scanner := bufio.NewScanner(input)
	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // 10MB max line
	inCopyBlock := false
	keepCopyBlock := false
	truncatedTables := map[string]bool{}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		// Skip Supabase restore guard metacommands that can block COPY data.
		if strings.HasPrefix(trimmed, "\\restrict") || strings.HasPrefix(trimmed, "\\unrestrict") {
			continue
		}

		if inCopyBlock {
			if keepCopyBlock {
				tempFile.WriteString(line + "\n")
			}
			if trimmed == "\\." {
				inCopyBlock = false
				keepCopyBlock = false
			}
			continue
		}

		if strings.HasPrefix(upper, "COPY ") {
			table := copyTargetTable(trimmed)
			if isAllowedCopyTable(table, allowedAuthCopyTables, allowedAllTables) {
				if !truncatedTables[table] {
					tempFile.WriteString(fmt.Sprintf("TRUNCATE TABLE %s CASCADE;\n", table))
					truncatedTables[table] = true
				}
				tempFile.WriteString(line + "\n")
				inCopyBlock = true
				keepCopyBlock = true
			} else {
				inCopyBlock = true
				keepCopyBlock = false
			}
			continue
		}

		if isAllowedSetvalStatement(trimmed, allowedAuthCopyTables, allowedAllTables) {
			tempFile.WriteString(line + "\n")
		}
	}

	if err := scanner.Err(); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("error reading backup file: %w", err)
	}

	// Write footer to restore normal trigger behavior
	tempFile.WriteString("\n-- Drift: Restore normal trigger behavior\n")
	tempFile.WriteString("SET session_replication_role = DEFAULT;\n")

	tempFile.Close()
	return tempFile.Name(), nil
}

// ResolveAllowedAuthCopyTables returns the auth.* tables that Drift will copy
// during plain SQL restore preprocessing.
func ResolveAllowedAuthCopyTables(opts RestoreOptions) ([]string, error) {
	tables, err := resolveAllowedAuthCopyTables(opts)
	if err != nil {
		return nil, err
	}

	list := make([]string, 0, len(tables))
	for table := range tables {
		list = append(list, table)
	}
	sort.Strings(list)
	return list, nil
}

func resolveAllowedAuthCopyTables(opts RestoreOptions) (map[string]bool, error) {
	tables := map[string]bool{}

	// Explicit override (useful for tests or custom callers).
	if len(opts.AuthCopyTables) > 0 {
		for _, table := range opts.AuthCopyTables {
			normalized := normalizeQualifiedName(table)
			if strings.HasPrefix(normalized, "auth.") {
				tables[normalized] = true
			}
		}
		return tables, nil
	}

	psql, err := findPGTool("psql")
	if err != nil {
		return nil, err
	}

	query := `
SELECT format('%I.%I', n.nspname, c.relname)
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'auth'
  AND c.relkind IN ('r', 'p')
  AND has_table_privilege(current_user, format('%I.%I', n.nspname, c.relname), 'INSERT')
ORDER BY c.relname;`

	args := []string{
		"-h", opts.Host,
		"-p", fmt.Sprintf("%d", opts.Port),
		"-U", opts.User,
		"-d", opts.Database,
		"-t", "-A", "-c", query,
	}

	env := map[string]string{
		"PGPASSWORD": opts.Password,
	}

	result, runErr := shell.RunWithEnv(env, psql, args...)
	if runErr != nil || result.ExitCode != 0 {
		// Conservative fallback if the privilege query can't run.
		tables["auth.users"] = true
		return tables, nil
	}

	for _, line := range strings.Split(result.Stdout, "\n") {
		normalized := normalizeQualifiedName(line)
		if strings.HasPrefix(normalized, "auth.") {
			tables[normalized] = true
		}
	}

	// Always keep users as a fallback safety net.
	if len(tables) == 0 {
		tables["auth.users"] = true
	}

	return tables, nil
}

func resolveAllInsertableCopyTables(opts RestoreOptions) (map[string]bool, error) {
	tables := map[string]bool{}

	psql, err := findPGTool("psql")
	if err != nil {
		return nil, err
	}

	query := `
SELECT format('%I.%I', n.nspname, c.relname)
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind IN ('r', 'p')
  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
  AND n.nspname NOT LIKE 'pg_toast%'
  AND n.nspname NOT LIKE 'pg_temp_%'
  AND has_table_privilege(current_user, format('%I.%I', n.nspname, c.relname), 'INSERT')
ORDER BY n.nspname, c.relname;`

	args := []string{
		"-h", opts.Host,
		"-p", fmt.Sprintf("%d", opts.Port),
		"-U", opts.User,
		"-d", opts.Database,
		"-t", "-A", "-c", query,
	}

	env := map[string]string{
		"PGPASSWORD": opts.Password,
	}

	result, runErr := shell.RunWithEnv(env, psql, args...)
	if runErr != nil || result.ExitCode != 0 {
		errMsg := ""
		if result != nil {
			errMsg = result.Stderr
		}
		if errMsg == "" && runErr != nil {
			errMsg = runErr.Error()
		}
		if errMsg == "" {
			errMsg = "failed to query insertable tables"
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	for _, line := range strings.Split(result.Stdout, "\n") {
		normalized := normalizeQualifiedName(line)
		if normalized != "" {
			tables[normalized] = true
		}
	}

	if len(tables) == 0 {
		return nil, fmt.Errorf("no insertable tables discovered for current role")
	}

	return tables, nil
}

func copyTargetTable(copyLine string) string {
	trimmed := strings.TrimSpace(copyLine)
	upper := strings.ToUpper(trimmed)
	if !strings.HasPrefix(upper, "COPY ") {
		return ""
	}

	rest := strings.TrimSpace(trimmed[5:])
	if rest == "" {
		return ""
	}

	if idx := strings.IndexAny(rest, " \t("); idx != -1 {
		rest = rest[:idx]
	}

	return normalizeQualifiedName(rest)
}

func normalizeQualifiedName(name string) string {
	n := strings.TrimSpace(name)
	n = strings.TrimSuffix(n, ";")
	n = strings.ReplaceAll(n, `"`, "")
	return strings.ToLower(n)
}

func isAllowedCopyTable(table string, allowedAuthCopyTables, allowedAllTables map[string]bool) bool {
	if len(allowedAllTables) > 0 {
		return allowedAllTables[table]
	}

	if strings.HasPrefix(table, "public.") {
		return true
	}
	if strings.HasPrefix(table, "auth.") {
		return allowedAuthCopyTables[table]
	}
	return table == "supabase_migrations.schema_migrations"
}

func isAllowedSetvalStatement(line string, allowedAuthCopyTables, allowedAllTables map[string]bool) bool {
	trimmed := strings.TrimSpace(line)
	upper := strings.ToUpper(trimmed)
	if !strings.HasPrefix(upper, "SELECT PG_CATALOG.SETVAL(") {
		return false
	}

	lower := strings.ToLower(trimmed)
	if len(allowedAllTables) > 0 {
		return !strings.Contains(lower, "'pg_catalog.") &&
			!strings.Contains(lower, "'information_schema.") &&
			!strings.Contains(lower, "'pg_toast")
	}

	if strings.Contains(lower, "'public.") || strings.Contains(lower, "'supabase_migrations.") {
		return true
	}

	if strings.Contains(lower, "'auth.") && len(allowedAuthCopyTables) > 0 {
		return true
	}

	return false
}

// TestConnection tests the database connection.
func TestConnection(opts RestoreOptions) error {
	psql, err := findPGTool("psql")
	if err != nil {
		return err
	}

	env := map[string]string{
		"PGPASSWORD": opts.Password,
	}

	args := []string{
		"-h", opts.Host,
		"-p", fmt.Sprintf("%d", opts.Port),
		"-U", opts.User,
		"-d", opts.Database,
		"-c", "SELECT 1;",
	}

	result, err := shell.RunWithEnv(env, psql, args...)
	if err != nil {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("connection test failed: %s", errMsg)
	}

	return nil
}

// ExecuteSQL executes a SQL statement.
func ExecuteSQL(opts RestoreOptions, sql string) (*shell.Result, error) {
	psql, err := findPGTool("psql")
	if err != nil {
		return nil, err
	}

	env := map[string]string{
		"PGPASSWORD": opts.Password,
	}

	args := []string{
		"-h", opts.Host,
		"-p", fmt.Sprintf("%d", opts.Port),
		"-U", opts.User,
		"-d", opts.Database,
		"-c", sql,
	}

	return shell.RunWithEnv(env, psql, args...)
}
