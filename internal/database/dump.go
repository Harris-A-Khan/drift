// Package database provides utilities for database operations.
package database

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/undrift/drift/pkg/shell"
)

// DumpOptions holds options for database dump.
type DumpOptions struct {
	PGBin       string
	Host        string
	Port        int
	Database    string
	User        string
	Password    string
	OutputFile  string
	Format      string // "custom", "plain", "directory", "tar"
	SchemaOnly  bool
	DataOnly    bool
	NoOwner     bool
	CleanFirst  bool
}

// DefaultDumpOptions returns default dump options.
func DefaultDumpOptions() DumpOptions {
	return DumpOptions{
		PGBin:    "/opt/homebrew/opt/postgresql@16/bin",
		Database: "postgres",
		User:     "postgres",
		Port:     5432,
		Format:   "custom",
		NoOwner:  true,
	}
}

// Dump performs a database dump using pg_dump.
func Dump(opts DumpOptions) error {
	pgDump := filepath.Join(opts.PGBin, "pg_dump")

	// Check if pg_dump exists
	if _, err := os.Stat(pgDump); os.IsNotExist(err) {
		return fmt.Errorf("pg_dump not found at %s", pgDump)
	}

	args := []string{
		"-h", opts.Host,
		"-p", fmt.Sprintf("%d", opts.Port),
		"-U", opts.User,
		"-d", opts.Database,
		"-F", opts.Format[0:1], // c, p, d, or t
		"-f", opts.OutputFile,
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

	if opts.CleanFirst {
		args = append(args, "-c")
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
		return fmt.Errorf("pg_dump failed: %s", errMsg)
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
	psql := filepath.Join(opts.PGBin, "psql")

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

