package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	printMigrationStatus(statusVerboseFlag, info)

	// === EDGE FUNCTIONS ===
	ui.NewLine()
	ui.SubHeader("Edge Functions")
	printFunctionsStatus(cfg)

	// === SERVICE HEALTH ===
	// Use the main project ref from config for health checks
	// Branch-specific refs don't have health endpoints
	mainProjectRef := cfg.Supabase.ProjectRef
	if mainProjectRef != "" {
		ui.NewLine()
		ui.SubHeader("Service Health")
		printServiceHealth(mainProjectRef)
	}

	// === WORKTREES ===
	if statusVerboseFlag {
		ui.NewLine()
		ui.SubHeader("Worktrees")
		printWorktreeStatus()
	}

	// === QUICK LINKS ===
	if info != nil {
		ui.NewLine()
		ui.SubHeader("Quick Links")
		ui.List(fmt.Sprintf("Dashboard: %s", GetProjectDashboardURL(info.ProjectRef)))
		ui.List(fmt.Sprintf("Functions: %s", GetFunctionsURL(info.ProjectRef)))
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

func printMigrationStatus(verbose bool, info *supabase.BranchInfo) {
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
	if info == nil {
		return
	}

	// Get DB URL from experimental API for non-production
	dbURL := getDbURLForBranch(info)
	if dbURL == "" {
		ui.KeyValue("Remote Status", ui.Dim("Could not check (no connection)"))
		return
	}

	result, err := shell.Run("supabase", "migration", "list", "--db-url", dbURL)
	if err == nil && result.Stdout != "" {
		// Check if any migration has empty REMOTE column (pending)
		hasPending := false
		lines := strings.Split(result.Stdout, "\n")
		for _, line := range lines {
			if strings.Contains(line, "|") && !strings.HasPrefix(strings.TrimSpace(line), "Local") && !strings.HasPrefix(strings.TrimSpace(line), "-") {
				parts := strings.Split(line, "|")
				if len(parts) >= 2 {
					remote := strings.TrimSpace(parts[1])
					if remote == "" || remote == " " {
						hasPending = true
						break
					}
				}
			}
		}
		if hasPending {
			ui.KeyValue("Remote Status", ui.Yellow("Some migrations may be pending"))
			ui.Infof("Run 'drift migrate history' for details")
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

func printServiceHealth(projectRef string) {
	// Check API health by hitting the REST endpoint
	apiURL := fmt.Sprintf("https://%s.supabase.co/rest/v1/", projectRef)

	if statusVerboseFlag {
		ui.Infof("Checking API health: %s", apiURL)
	}

	// Try to check if API is responding with curl's built-in timeout
	// Use --connect-timeout for connection phase and --max-time for total time
	result, err := shell.RunWithTimeout(15*time.Second, "curl", "-s", "-o", "/dev/null",
		"--connect-timeout", "5", "--max-time", "10",
		"-w", "%{http_code}", apiURL)
	if err != nil {
		ui.KeyValue("API", ui.Yellow("Could not check"))
		if statusVerboseFlag {
			ui.Infof("  Error: %v", err)
		}
	} else {
		code := strings.TrimSpace(result.Stdout)
		if statusVerboseFlag {
			ui.Infof("  Response code: %s", code)
		}
		if code == "200" || code == "401" { // 401 is expected without auth
			ui.KeyValue("API", ui.Green("✓ Healthy"))
		} else if code == "000" {
			ui.KeyValue("API", ui.Yellow("Could not connect"))
		} else {
			ui.KeyValue("API", ui.Red(fmt.Sprintf("✗ Status %s", code)))
		}
	}

	// Check database via Supabase Management API (project status)
	if statusVerboseFlag {
		ui.Infof("Checking database status via Management API...")
	}

	client := supabase.NewClient()
	status, err := client.GetProjectStatus(projectRef)
	if err != nil {
		ui.KeyValue("Database", ui.Yellow("Could not check"))
		if statusVerboseFlag {
			ui.Infof("  Error: %v", err)
			ui.Infof("  Hint: Ensure SUPABASE_ACCESS_TOKEN is set for API access")
		}
	} else {
		if statusVerboseFlag {
			ui.Infof("  Project status: %s", status)
		}
		switch strings.ToUpper(status) {
		case "ACTIVE_HEALTHY":
			ui.KeyValue("Database", ui.Green("✓ Healthy"))
		case "ACTIVE_UNHEALTHY":
			ui.KeyValue("Database", ui.Red("✗ Unhealthy"))
		case "COMING_UP", "GOING_DOWN", "RESTORING":
			ui.KeyValue("Database", ui.Yellow(fmt.Sprintf("⚠ %s", status)))
		case "INACTIVE":
			ui.KeyValue("Database", ui.Dim("Paused"))
		default:
			ui.KeyValue("Database", ui.Yellow(status))
		}
	}

	// Check functions status
	if statusVerboseFlag {
		ui.Infof("Checking Edge Functions...")
	}

	funcResult, err := shell.Run("supabase", "functions", "list", "--project-ref", projectRef)
	if err != nil {
		ui.KeyValue("Functions", ui.Yellow("Could not check"))
		if statusVerboseFlag {
			ui.Infof("  Error: %v", err)
		}
	} else if funcResult.ExitCode == 0 {
		// Count deployed functions from output
		lines := strings.Split(funcResult.Stdout, "\n")
		deployedCount := 0
		for _, line := range lines {
			if strings.Contains(line, "ACTIVE") {
				deployedCount++
			}
		}
		if statusVerboseFlag && funcResult.Stdout != "" {
			ui.Infof("  Raw output: %s", strings.TrimSpace(funcResult.Stdout))
		}
		if deployedCount > 0 {
			ui.KeyValue("Functions", ui.Green(fmt.Sprintf("✓ %d deployed", deployedCount)))
		} else {
			ui.KeyValue("Functions", ui.Dim("No functions deployed"))
		}
	} else if statusVerboseFlag {
		ui.Infof("  Exit code: %d", funcResult.ExitCode)
		if funcResult.Stderr != "" {
			ui.Infof("  Stderr: %s", funcResult.Stderr)
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

// getDbURLForBranch gets the database URL for a branch.
// For non-production, it gets from the experimental API.
// For production, it builds from env vars.
func getDbURLForBranch(info *supabase.BranchInfo) string {
	if info == nil {
		return ""
	}

	// For production, require explicit password and build URL
	if info.Environment == supabase.EnvProduction {
		cfg := config.LoadOrDefault()
		poolerHost := cfg.Database.GetPoolerHostForBranch(info.SupabaseBranch.GitBranch)
		poolerPort := cfg.Database.GetPoolerPort()

		// Prefer host/port from branch connection info so regional projects connect correctly.
		client := supabase.NewClient()
		if connInfo, err := client.GetBranchConnectionInfo(info.SupabaseBranch.GitBranch); err == nil && connInfo != nil {
			if connInfo.PoolerHost != "" {
				poolerHost = connInfo.PoolerHost
			}
			if connInfo.PoolerPort != 0 {
				poolerPort = connInfo.PoolerPort
			}
		}

		pw := os.Getenv("PROD_PASSWORD")
		if pw == "" {
			return ""
		}
		return fmt.Sprintf("postgresql://postgres.%s:%s@%s:%d/postgres", info.ProjectRef, pw, poolerHost, poolerPort)
	}

	// For non-production, try experimental API
	client := supabase.NewClient()
	connInfo, err := client.GetBranchConnectionInfo(info.SupabaseBranch.GitBranch)
	if err != nil {
		return ""
	}

	if connInfo.PostgresURL == "" {
		return ""
	}

	// Use session mode (port 5432) instead of transaction mode (port 6543)
	// Transaction mode doesn't support prepared statements which supabase CLI uses
	url := strings.Replace(connInfo.PostgresURL, ":6543/", ":5432/", 1)

	return url
}
