// Package database provides utilities for database operations.
package database

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/undrift/drift/pkg/shell"
)

// DumpOptions holds options for database dump.
type DumpOptions struct {
	Host         string
	Port         int
	Database     string
	User         string
	Password     string
	OutputFile   string
	Format       string // "custom", "plain", "directory", "tar"
	SchemaOnly   bool
	DataOnly     bool
	NoOwner      bool
	NoPrivileges bool
	CleanFirst   bool
	IfExists     bool
}

// DefaultDumpOptions returns default dump options.
// Uses plain SQL format by default (like the original push-db-to-branch.sh script)
// which dumps everything including auth.users and can be restored with psql -f.
func DefaultDumpOptions() DumpOptions {
	return DumpOptions{
		Database:     "postgres",
		User:         "postgres",
		Port:         5432,
		Format:       "plain", // Plain SQL format - includes everything and works with psql -f
		NoOwner:      true,
		NoPrivileges: true,
		CleanFirst:   true, // Include DROP statements before CREATE
		IfExists:     true, // Use IF EXISTS with DROP statements
	}
}

// findPGTool looks for a PostgreSQL tool in PATH.
func findPGTool(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf(`%s not found in PATH

Install the latest version of PostgreSQL:
  brew install postgresql

Then add to your shell profile (~/.zshrc or ~/.bashrc):
  export PATH="$(brew --prefix postgresql)/bin:$PATH"

Or run with PATH prefix:
  PATH="$(brew --prefix postgresql)/bin:$PATH" drift db dump prod`, name)
	}
	return path, nil
}

// FindPGDump returns the path to pg_dump.
func FindPGDump() (string, error) {
	return findPGTool("pg_dump")
}

// Dump performs a database dump using pg_dump.
func Dump(opts DumpOptions) error {
	pgDump, err := findPGTool("pg_dump")
	if err != nil {
		return err
	}

	args := []string{
		"-h", opts.Host,
		"-p", fmt.Sprintf("%d", opts.Port),
		"-U", opts.User,
		"-d", opts.Database,
		"-F", opts.Format[0:1], // c, p, d, or t
		"-f", opts.OutputFile,
		// Dump entire database - don't filter schemas to avoid missing anything
		// (supabase_migrations, extensions, etc.)
	}

	if opts.SchemaOnly {
		args = append(args, "-s")
	}

	if opts.DataOnly {
		args = append(args, "-a")
	}

	if opts.NoOwner {
		args = append(args, "-O")
	}

	if opts.NoPrivileges {
		args = append(args, "-x")
	}

	if opts.CleanFirst {
		args = append(args, "-c")
	}

	if opts.IfExists {
		args = append(args, "--if-exists")
	}

	// Set password via environment
	env := map[string]string{
		"PGPASSWORD": opts.Password,
	}

	result, err := shell.RunWithEnv(env, pgDump, args...)
	if err != nil {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		// Clean up any partial file
		os.Remove(opts.OutputFile)

		// Check for specific error types
		stderrLower := strings.ToLower(errMsg)
		if strings.Contains(stderrLower, "password authentication failed") ||
			strings.Contains(stderrLower, "authentication failed") {
			return fmt.Errorf("authentication failed - incorrect password\n\nCheck your database password and try again")
		}
		if strings.Contains(stderrLower, "could not connect") ||
			strings.Contains(stderrLower, "connection refused") {
			return fmt.Errorf("could not connect to database\n\nCheck:\n  1. Host is correct: %s:%d\n  2. Your network can reach the pooler\n  3. Supabase project is active", opts.Host, opts.Port)
		}
		if strings.Contains(stderrLower, "timeout") {
			return fmt.Errorf("connection timed out\n\nThe database server at %s:%d is not responding", opts.Host, opts.Port)
		}
		if strings.Contains(stderrLower, "version mismatch") || strings.Contains(stderrLower, "aborting") {
			return fmt.Errorf("pg_dump version mismatch: %s\n\nYour pg_dump must be >= the server version.\nInstall the latest: brew install postgresql", errMsg)
		}

		return fmt.Errorf("pg_dump failed: %s", errMsg)
	}

	// Check for warnings in stderr even on success (e.g., version mismatch)
	if result.Stderr != "" {
		stderr := strings.ToLower(result.Stderr)
		if strings.Contains(stderr, "version mismatch") || strings.Contains(stderr, "aborting") {
			os.Remove(opts.OutputFile)
			return fmt.Errorf("pg_dump version mismatch: %s\n\nYour pg_dump must be >= the server version.\nInstall the latest: brew install postgresql", result.Stderr)
		}
	}

	// Validate output file exists and is not empty
	info, err := os.Stat(opts.OutputFile)
	if err != nil {
		return fmt.Errorf("pg_dump did not create output file: %w", err)
	}

	if info.Size() == 0 {
		os.Remove(opts.OutputFile)
		return fmt.Errorf("pg_dump created an empty file - connection may have failed silently. Check:\n  1. Password is correct\n  2. Pooler host is reachable: %s:%d\n  3. pg_dump version >= server version (brew install postgresql)", opts.Host, opts.Port)
	}

	// Minimum size sanity check - a valid dump should be at least 1KB
	if info.Size() < 1024 {
		return fmt.Errorf("pg_dump created a suspiciously small file (%d bytes) - verify connection succeeded", info.Size())
	}

	return nil
}

// DumpToFile dumps a database to a file with automatic naming.
func DumpToFile(opts DumpOptions, prefix string) (string, error) {
	if opts.OutputFile == "" {
		timestamp := time.Now().Format("20060102-150405")
		opts.OutputFile = fmt.Sprintf("%s-%s.backup", prefix, timestamp)
	}

	if err := Dump(opts); err != nil {
		return "", err
	}

	return opts.OutputFile, nil
}

// GetDumpConnectionString returns a connection string for the dump.
func GetDumpConnectionString(opts DumpOptions) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		opts.User, opts.Password, opts.Host, opts.Port, opts.Database)
}

// EstimateDatabaseSize returns an approximate size of the database.
func EstimateDatabaseSize(opts DumpOptions) (string, error) {
	psql, err := findPGTool("psql")
	if err != nil {
		return "", err
	}

	query := "SELECT pg_size_pretty(pg_database_size(current_database()));"

	env := map[string]string{
		"PGPASSWORD": opts.Password,
	}

	args := []string{
		"-h", opts.Host,
		"-p", fmt.Sprintf("%d", opts.Port),
		"-U", opts.User,
		"-d", opts.Database,
		"-t", "-c", query,
	}

	result, err := shell.RunWithEnv(env, psql, args...)
	if err != nil {
		return "", fmt.Errorf("failed to get database size: %w", err)
	}

	return result.Stdout, nil
}

