package database

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/undrift/drift/pkg/shell"
)

// RestoreOptions holds options for database restore.
type RestoreOptions struct {
	PGBin       string
	Host        string
	Port        int
	Database    string
	User        string
	Password    string
	InputFile   string
	CleanFirst  bool
	NoOwner     bool
	SingleTxn   bool
	Jobs        int // Number of parallel jobs
}

// DefaultRestoreOptions returns default restore options.
func DefaultRestoreOptions() RestoreOptions {
	return RestoreOptions{
		PGBin:      "/opt/homebrew/opt/postgresql@16/bin",
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

	// Use pg_restore for custom format, psql for plain SQL
	ext := filepath.Ext(opts.InputFile)
	if ext == ".sql" {
		return restoreSQL(opts)
	}

	return restoreCustom(opts)
}

// restoreCustom restores from a custom format backup using pg_restore.
func restoreCustom(opts RestoreOptions) error {
	pgRestore := filepath.Join(opts.PGBin, "pg_restore")

	// Check if pg_restore exists
	if _, err := os.Stat(pgRestore); os.IsNotExist(err) {
		return fmt.Errorf("pg_restore not found at %s", pgRestore)
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
	psql := filepath.Join(opts.PGBin, "psql")

	if _, err := os.Stat(psql); os.IsNotExist(err) {
		return fmt.Errorf("psql not found at %s", psql)
	}

	args := []string{
		"-h", opts.Host,
		"-p", fmt.Sprintf("%d", opts.Port),
		"-U", opts.User,
		"-d", opts.Database,
		"-f", opts.InputFile,
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

// TestConnection tests the database connection.
func TestConnection(opts RestoreOptions) error {
	psql := filepath.Join(opts.PGBin, "psql")

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
	psql := filepath.Join(opts.PGBin, "psql")

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

