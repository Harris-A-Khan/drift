package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/internal/git"
	"github.com/undrift/drift/internal/supabase"
	"github.com/undrift/drift/internal/ui"
	"github.com/undrift/drift/internal/web"
	"github.com/undrift/drift/internal/xcode"
	"github.com/undrift/drift/pkg/shell"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show project status overview",
	Long: `Display a comprehensive overview of your drift project status.

This includes:
  - Current git branch and Supabase environment
  - Config file status (.env.local or Config.xcconfig)
  - Migration status
  - Edge functions count`,
	RunE: runStatus,
}

var statusVerboseFlag bool

func init() {
	statusCmd.Flags().BoolVarP(&statusVerboseFlag, "verbose", "v", false, "Show detailed status")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()

	// Get current git branch
	gitBranch, err := git.CurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current git branch: %w", err)
	}

	// Resolve Supabase branch
	client := supabase.NewClient()
	overrideBranch := cfg.Supabase.OverrideBranch
	info, err := client.GetBranchInfoWithOverride(gitBranch, overrideBranch)

	// === HEADER ===
	ui.Header("Drift Status")

	// === GIT & ENVIRONMENT ===
	ui.KeyValue("Git Branch", ui.Cyan(gitBranch))

	if err != nil {
		ui.Warning(fmt.Sprintf("Could not resolve Supabase branch: %v", err))
	} else {
		ui.KeyValue("Environment", envColorString(string(info.Environment)))

		branchDisplay := info.SupabaseBranch.Name
		if info.IsOverride {
			branchDisplay = fmt.Sprintf("%s (override from %s)", info.SupabaseBranch.Name, info.OverrideFrom)
		} else if info.IsFallback {
			branchDisplay = fmt.Sprintf("%s (fallback)", info.SupabaseBranch.Name)
		}
		ui.KeyValue("Supabase Branch", ui.Cyan(branchDisplay))
		ui.KeyValue("Project Ref", ui.Cyan(info.ProjectRef))
	}

	// === CONFIG FILE STATUS ===
	ui.NewLine()
	ui.SubHeader("Config File")

	configStatus := checkConfigFileStatus(cfg, info)
	printConfigStatus(configStatus)

	// === MIGRATIONS ===
	ui.NewLine()
	ui.SubHeader("Migrations")
	printMigrationStatus(statusVerboseFlag)

	// === EDGE FUNCTIONS ===
	ui.NewLine()
	ui.SubHeader("Edge Functions")
	printFunctionsStatus(cfg)

	// === WORKTREES ===
	if statusVerboseFlag {
		ui.NewLine()
		ui.SubHeader("Worktrees")
		printWorktreeStatus()
	}

	return nil
}

type configFileStatus struct {
	exists       bool
	path         string
	isUpToDate   bool
	currentEnv   string
	expectedEnv  string
	errorMessage string
}

func checkConfigFileStatus(cfg *config.Config, info *supabase.BranchInfo) configFileStatus {
	status := configFileStatus{}

	if cfg.Project.IsWebPlatform() {
		status.path = cfg.GetEnvLocalPath()
		status.exists = web.EnvLocalExists(status.path)

		if status.exists {
			currentEnv, err := web.GetCurrentEnvironment(status.path)
			if err != nil {
				status.errorMessage = err.Error()
			} else {
				status.currentEnv = currentEnv
				if info != nil {
					status.expectedEnv = string(info.Environment)
					status.isUpToDate = status.currentEnv == status.expectedEnv
				}
			}
		}
	} else {
		status.path = cfg.GetXcconfigPath()
		status.exists = xcode.XcconfigExists(status.path)

		if status.exists {
			currentEnv, err := xcode.GetCurrentEnvironment(status.path)
			if err != nil {
				status.errorMessage = err.Error()
			} else {
				status.currentEnv = currentEnv
				if info != nil {
					status.expectedEnv = string(info.Environment)
					status.isUpToDate = status.currentEnv == status.expectedEnv
				}
			}
		}
	}

	return status
}

func printConfigStatus(status configFileStatus) {
	if !status.exists {
		ui.KeyValue("Status", ui.Red("Not found"))
		ui.KeyValue("Path", status.path)
		ui.Infof("Run 'drift env setup' to generate")
		return
	}

	// Show path
	ui.KeyValue("Path", status.path)

	if status.errorMessage != "" {
		ui.Warning(fmt.Sprintf("Could not read config: %s", status.errorMessage))
		ui.Infof("Run 'drift env setup' to regenerate")
		return
	}

	ui.KeyValue("Configured Env", envColorString(status.currentEnv))

	if status.isUpToDate {
		ui.KeyValue("Status", ui.Green("✓ Up to date"))
	} else if status.expectedEnv != "" {
		ui.KeyValue("Status", ui.Yellow("⚠ Out of sync"))
		ui.Infof("Expected: %s, Got: %s", status.expectedEnv, status.currentEnv)
		ui.Infof("Run 'drift env setup' to update")
	}
}

func printMigrationStatus(verbose bool) {
	// Check if migrations directory exists
	migrationsPath := "supabase/migrations"
	if _, err := os.Stat(migrationsPath); os.IsNotExist(err) {
		ui.KeyValue("Status", ui.Dim("No migrations directory"))
		return
	}

	// Count local migrations
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not read migrations: %v", err))
		return
	}

	migrationCount := 0
	var migrations []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			migrationCount++
			migrations = append(migrations, entry.Name())
		}
	}

	ui.KeyValue("Local Migrations", fmt.Sprintf("%d", migrationCount))

	if verbose && len(migrations) > 0 {
		// Show last 5 migrations
		start := 0
		if len(migrations) > 5 {
			start = len(migrations) - 5
			ui.Infof("Showing last 5 of %d:", migrationCount)
		}
		for _, m := range migrations[start:] {
			ui.List(m)
		}
	}

	// Try to get remote migration status
	result, err := shell.Run("supabase", "migration", "list", "--output", "json")
	if err == nil && result.Stdout != "" {
		// Parse output to check for pending
		if strings.Contains(result.Stdout, "pending") || strings.Contains(result.Stdout, "Not applied") {
			ui.KeyValue("Remote Status", ui.Yellow("Some migrations may be pending"))
			ui.Infof("Run 'drift migrate status' for details")
		} else {
			ui.KeyValue("Remote Status", ui.Green("✓ Up to date"))
		}
	}
}

func printFunctionsStatus(cfg *config.Config) {
	functionsPath := cfg.GetFunctionsPath()

	// Check if functions directory exists
	if _, err := os.Stat(functionsPath); os.IsNotExist(err) {
		ui.KeyValue("Status", ui.Dim("No functions directory"))
		return
	}

	functions, err := supabase.ListFunctions(functionsPath)
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not list functions: %v", err))
		return
	}

	ui.KeyValue("Functions", fmt.Sprintf("%d", len(functions)))

	if statusVerboseFlag && len(functions) > 0 {
		for _, fn := range functions {
			ui.List(fn.Name)
		}
	}
}

func printWorktreeStatus() {
	worktrees, err := git.ListWorktrees()
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not list worktrees: %v", err))
		return
	}

	ui.KeyValue("Worktrees", fmt.Sprintf("%d", len(worktrees)))

	for _, wt := range worktrees {
		branchDisplay := wt.Branch
		if branchDisplay == "" {
			branchDisplay = "(bare)"
		}

		current := ""
		if wt.IsCurrent {
			current = " ← current"
		}

		// Color based on branch type
		switch {
		case branchDisplay == "main" || branchDisplay == "master":
			branchDisplay = ui.Red(branchDisplay)
		case branchDisplay == "development":
			branchDisplay = ui.Yellow(branchDisplay)
		default:
			branchDisplay = ui.Cyan(branchDisplay)
		}

		ui.List(fmt.Sprintf("%s%s", branchDisplay, current))
	}
}

// GetProjectDashboardURL returns the URL for the Supabase project dashboard.
func GetProjectDashboardURL(projectRef string) string {
	return fmt.Sprintf("https://supabase.com/dashboard/project/%s", projectRef)
}

// GetTableEditorURL returns the URL for the Supabase table editor.
func GetTableEditorURL(projectRef string) string {
	return fmt.Sprintf("https://supabase.com/dashboard/project/%s/editor", projectRef)
}

// GetSQLEditorURL returns the URL for the Supabase SQL editor.
func GetSQLEditorURL(projectRef string) string {
	return fmt.Sprintf("https://supabase.com/dashboard/project/%s/sql", projectRef)
}

// GetFunctionsURL returns the URL for Edge Functions.
func GetFunctionsURL(projectRef string) string {
	return fmt.Sprintf("https://supabase.com/dashboard/project/%s/functions", projectRef)
}

// GetAuthURL returns the URL for Auth settings.
func GetAuthURL(projectRef string) string {
	return fmt.Sprintf("https://supabase.com/dashboard/project/%s/auth/users", projectRef)
}

// GetStorageURL returns the URL for Storage.
func GetStorageURL(projectRef string) string {
	return fmt.Sprintf("https://supabase.com/dashboard/project/%s/storage/buckets", projectRef)
}

// GetLogsURL returns the URL for Logs Explorer.
func GetLogsURL(projectRef string) string {
	return fmt.Sprintf("https://supabase.com/dashboard/project/%s/logs/explorer", projectRef)
}

// GetSettingsURL returns the URL for project settings.
func GetSettingsURL(projectRef string) string {
	return fmt.Sprintf("https://supabase.com/dashboard/project/%s/settings/general", projectRef)
}

// GetAPISettingsURL returns the URL for API settings (keys).
func GetAPISettingsURL(projectRef string) string {
	return fmt.Sprintf("https://supabase.com/dashboard/project/%s/settings/api", projectRef)
}

// GetBranchesURL returns the URL for Supabase branches.
func GetBranchesURL(projectRef string) string {
	// Branches are at the organization level, need parent project
	return fmt.Sprintf("https://supabase.com/dashboard/project/%s/branches", projectRef)
}

// GetRepoURL returns the GitHub repo URL from git remote.
func GetRepoURL(cfg *config.Config) string {
	// Try to get from git remote
	result, err := shell.Run("git", "remote", "get-url", "origin")
	if err == nil && result.Stdout != "" {
		url := strings.TrimSpace(result.Stdout)
		// Convert git@github.com:user/repo.git to https://github.com/user/repo
		if strings.HasPrefix(url, "git@github.com:") {
			url = strings.TrimPrefix(url, "git@github.com:")
			url = strings.TrimSuffix(url, ".git")
			return "https://github.com/" + url
		}
		// Already https
		return strings.TrimSuffix(url, ".git")
	}

	return ""
}

// GetDeployURL returns the deployment URL (not currently configured).
func GetDeployURL(cfg *config.Config, env supabase.Environment) string {
	// Deployment URLs need to be configured in the future
	// For now, return empty - users can open Vercel dashboard instead
	return ""
}

// GetAppStoreConnectURL returns App Store Connect URL.
func GetAppStoreConnectURL() string {
	return "https://appstoreconnect.apple.com"
}

// GetVercelURL returns Vercel dashboard URL.
func GetVercelURL(cfg *config.Config) string {
	// For now, just return the main dashboard
	// Project-specific URLs could be configured in the future
	return "https://vercel.com/dashboard"
}

// GetGitHubActionsURL returns GitHub Actions URL.
func GetGitHubActionsURL(cfg *config.Config) string {
	repoURL := GetRepoURL(cfg)
	if repoURL != "" {
		return repoURL + "/actions"
	}
	return ""
}

// ResolveFilePath resolves a relative path from project root.
func ResolveFilePath(cfg *config.Config, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(cfg.ProjectRoot(), path)
}
