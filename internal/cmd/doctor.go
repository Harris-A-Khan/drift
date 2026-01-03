package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/internal/ui"
	"github.com/undrift/drift/pkg/shell"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system dependencies and configuration",
	Long: `Check that all required tools are installed and configured correctly.

This command verifies:
  - Required CLI tools (git, supabase, pg_dump, etc.)
  - Configuration file presence and validity
  - Environment variables
  - Supabase project linking`,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

type checkResult struct {
	name    string
	status  string // "ok", "warning", "error"
	message string
	version string
}

func runDoctor(cmd *cobra.Command, args []string) error {
	ui.Header("Drift Doctor")
	
	results := []checkResult{}
	hasErrors := false

	// System info
	ui.SubHeader("System Information")
	ui.KeyValue("OS", runtime.GOOS)
	ui.KeyValue("Arch", runtime.GOARCH)
	ui.KeyValue("Drift", version)
	ui.NewLine()

	// Check required tools
	ui.SubHeader("Required Tools")

	// Git
	results = append(results, checkGit())
	
	// Supabase CLI
	results = append(results, checkSupabase())
	
	// PostgreSQL tools
	results = append(results, checkPgTools())
	
	// Optional tools
	ui.SubHeader("Optional Tools")
	
	// fzf (for interactive selection)
	results = append(results, checkOptionalTool("fzf", "fzf --version", "interactive selection"))
	
	// VS Code
	results = append(results, checkOptionalTool("code", "code --version", "opening worktrees"))

	// Print tool results
	for _, r := range results {
		printCheckResult(r)
		if r.status == "error" {
			hasErrors = true
		}
	}

	ui.NewLine()

	// Configuration
	ui.SubHeader("Configuration")
	checkConfig()

	// Supabase project
	ui.SubHeader("Supabase Project")
	checkSupabaseProject()

	ui.NewLine()

	if hasErrors {
		ui.Error("Some checks failed. Please install missing dependencies.")
		return fmt.Errorf("doctor checks failed")
	}

	ui.Success("All checks passed!")
	return nil
}

func checkGit() checkResult {
	result, err := shell.Run("git", "--version")
	if err != nil {
		return checkResult{
			name:    "git",
			status:  "error",
			message: "Git is not installed",
		}
	}

	// Parse version from "git version 2.x.x"
	version := strings.TrimPrefix(result.Stdout, "git version ")
	version = strings.Split(version, " ")[0]

	return checkResult{
		name:    "git",
		status:  "ok",
		version: version,
	}
}

func checkSupabase() checkResult {
	result, err := shell.Run("supabase", "--version")
	if err != nil {
		return checkResult{
			name:    "supabase",
			status:  "error",
			message: "Supabase CLI is not installed. Install with: brew install supabase/tap/supabase",
		}
	}

	// Parse version
	version := strings.TrimSpace(result.Stdout)

	return checkResult{
		name:    "supabase",
		status:  "ok",
		version: version,
	}
}

func checkPgTools() checkResult {
	// Try common locations
	pgBinPaths := []string{
		"/opt/homebrew/opt/postgresql@16/bin",
		"/opt/homebrew/opt/postgresql@15/bin",
		"/opt/homebrew/opt/postgresql@14/bin",
		"/opt/homebrew/bin",
		"/usr/local/opt/postgresql@16/bin",
		"/usr/local/opt/postgresql@15/bin",
		"/usr/local/bin",
		"/usr/bin",
	}

	// Check environment variable first
	if envPath := os.Getenv("PG_BIN"); envPath != "" {
		pgBinPaths = append([]string{envPath}, pgBinPaths...)
	}

	// Also check config if available
	cfg := config.LoadOrDefault()
	if cfg.Database.PGBin != "" {
		pgBinPaths = append([]string{cfg.Database.PGBin}, pgBinPaths...)
	}

	for _, binPath := range pgBinPaths {
		pgDump := filepath.Join(binPath, "pg_dump")
		if _, err := os.Stat(pgDump); err == nil {
			// Found pg_dump, get version
			result, err := shell.Run(pgDump, "--version")
			if err == nil {
				version := strings.TrimSpace(result.Stdout)
				// Extract just the version number
				parts := strings.Split(version, " ")
				if len(parts) >= 3 {
					version = parts[len(parts)-1]
				}
				return checkResult{
					name:    "pg_dump",
					status:  "ok",
					version: version,
					message: fmt.Sprintf("Found at %s", binPath),
				}
			}
		}
	}

	return checkResult{
		name:    "pg_dump",
		status:  "error",
		message: "PostgreSQL tools not found. Install with: brew install postgresql@16",
	}
}

func checkOptionalTool(name, versionCmd, purpose string) checkResult {
	parts := strings.Split(versionCmd, " ")
	result, err := shell.Run(parts[0], parts[1:]...)
	if err != nil {
		return checkResult{
			name:    name,
			status:  "warning",
			message: fmt.Sprintf("Not installed (used for %s)", purpose),
		}
	}

	// Get first line of version output
	version := strings.Split(result.Stdout, "\n")[0]
	version = strings.TrimSpace(version)

	return checkResult{
		name:    name,
		status:  "ok",
		version: version,
	}
}

func printCheckResult(r checkResult) {
	switch r.status {
	case "ok":
		version := ""
		if r.version != "" {
			version = ui.Dim(fmt.Sprintf(" (%s)", r.version))
		}
		ui.Success(fmt.Sprintf("%s%s", r.name, version))
		if r.message != "" {
			ui.Info(fmt.Sprintf("  %s", r.message))
		}
	case "warning":
		ui.Warning(fmt.Sprintf("%s - %s", r.name, r.message))
	case "error":
		ui.Error(fmt.Sprintf("%s - %s", r.name, r.message))
	}
}

func checkConfig() {
	configPath, err := config.FindConfigFile()
	if err != nil {
		ui.Warning("No .drift.yaml found (using defaults)")
		ui.Info("  Run 'drift init' to create one")
		return
	}

	cfg, err := config.LoadFromPath(configPath)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to parse config: %v", err))
		return
	}

	ui.Success(fmt.Sprintf("Found .drift.yaml at %s", configPath))
	
	// Show key config values
	if cfg.Project.Name != "" {
		ui.KeyValue("Project", cfg.Project.Name)
	}
	ui.KeyValue("Project Type", cfg.Project.Type)
	ui.KeyValue("Functions Dir", cfg.Supabase.FunctionsDir)
	ui.KeyValue("Migrations Dir", cfg.Supabase.MigrationsDir)
}

func checkSupabaseProject() {
	// Check if we're in a Supabase project
	result, err := shell.Run("supabase", "status")
	if err != nil {
		if strings.Contains(result.Stderr, "not linked") || result.ExitCode != 0 {
			ui.Warning("Supabase project not linked")
			ui.Info("  Run 'supabase link' to connect to your project")
			return
		}
		ui.Warning("Could not determine Supabase status")
		return
	}

	ui.Success("Supabase project is linked")

	// Try to get project info
	if strings.Contains(result.Stdout, "API URL") {
		lines := strings.Split(result.Stdout, "\n")
		for _, line := range lines {
			if strings.Contains(line, "API URL") {
				ui.KeyValue("API URL", strings.TrimSpace(strings.Split(line, ":")[1]))
				break
			}
		}
	}
}

