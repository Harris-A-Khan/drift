package database

import (
	"bufio"
	"fmt"
	"os"
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

	// Preprocess the backup file to:
	// 1. Remove \restrict lines (Supabase security feature that breaks restore)
	// 2. Add session_replication_role = replica to disable triggers during restore
	processedFile, err := preprocessBackupFile(opts.InputFile)
	if err != nil {
		return fmt.Errorf("failed to preprocess backup: %w", err)
	}
	defer os.Remove(processedFile) // Clean up temp file

	args := []string{
		"-h", opts.Host,
		"-p", fmt.Sprintf("%d", opts.Port),
		"-U", opts.User,
		"-d", opts.Database,
		"-f", processedFile,
	}

	if opts.SingleTxn {
		args = append(args, "-1")
	}

	env := map[string]string{
		"PGPASSWORD": opts.Password,
	}

	result, err := shell.RunWithEnv(env, psql, args...)
	if err != nil {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("psql restore failed: %s", errMsg)
	}

	return nil
}

// preprocessBackupFile creates a modified copy of the backup file that:
// 1. Removes \restrict lines (Supabase security feature)
// 2. Wraps content with SET session_replication_role = replica to disable triggers
func preprocessBackupFile(inputFile string) (string, error) {
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

	for scanner.Scan() {
		line := scanner.Text()

		// Skip \restrict lines (Supabase security feature that blocks COPY data)
		if strings.HasPrefix(line, "\\restrict") {
			continue
		}

		tempFile.WriteString(line + "\n")
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

