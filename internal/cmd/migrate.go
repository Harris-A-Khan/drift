package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/internal/git"
	"github.com/undrift/drift/internal/supabase"
	"github.com/undrift/drift/internal/ui"
	"github.com/undrift/drift/pkg/shell"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Database migrations",
	Long:  `Push database migrations to Supabase branches.`,
}

var migratePushCmd = &cobra.Command{
	Use:   "push [branch]",
	Short: "Push migrations to branch",
	Long: `Push local migrations to a Supabase preview branch.

If no branch is specified, uses the current git branch.
Protected branches (main, master) are blocked by default.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runMigratePush,
}

var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show migration status",
	Long:  `Show the current migration status for the project.`,
	RunE:  runMigrateStatus,
}

var migrateNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new migration",
	Long:  `Create a new migration file with the given name.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runMigrateNew,
}

var migrateHistoryCmd = &cobra.Command{
	Use:   "history [branch]",
	Short: "Show migration history",
	Long: `Show which migrations have been applied to a branch.

Displays all migrations with their status (applied/pending) and timestamps.
If no branch is specified, uses the current git branch.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runMigrateHistory,
}

var (
	migrateDryRunFlag bool
	migrateForceFlag  bool
)

func init() {
	migratePushCmd.Flags().BoolVar(&migrateDryRunFlag, "dry-run", false, "Show what would be pushed without actually pushing")
	migratePushCmd.Flags().BoolVarP(&migrateForceFlag, "force", "f", false, "Force push to protected branches")

	migrateCmd.AddCommand(migratePushCmd)
	migrateCmd.AddCommand(migrateStatusCmd)
	migrateCmd.AddCommand(migrateNewCmd)
	migrateCmd.AddCommand(migrateHistoryCmd)
	rootCmd.AddCommand(migrateCmd)
}

func runMigratePush(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()

	// Determine target branch
	targetBranch := ""
	if len(args) > 0 {
		targetBranch = args[0]
	} else {
		var err error
		targetBranch, err = git.CurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
	}

	// Check for protected branches
	if cfg.IsProtectedBranch(targetBranch) && !migrateForceFlag {
		return fmt.Errorf("'%s' is a protected branch. Use --force to override", targetBranch)
	}

	ui.Header("Push Migrations")

	// Apply override from config
	overrideBranch := cfg.Supabase.OverrideBranch

	// Resolve Supabase branch
	client := supabase.NewClient()

	sp := ui.NewSpinner("Resolving Supabase branch")
	sp.Start()

	info, err := client.GetBranchInfoWithOverride(targetBranch, overrideBranch)
	if err != nil {
		sp.Fail("Failed to resolve branch")
		return err
	}
	sp.Stop()

	ui.KeyValue("Git Branch", ui.Cyan(targetBranch))
	ui.KeyValue("Environment", envColorString(string(info.Environment)))
	ui.KeyValue("Supabase Branch", ui.Cyan(info.SupabaseBranch.Name))
	ui.KeyValue("Project Ref", ui.Cyan(info.ProjectRef))

	if info.IsOverride {
		ui.Infof("Override: using %s instead of %s", ui.Cyan(info.SupabaseBranch.Name), ui.Cyan(info.OverrideFrom))
	}

	if info.IsFallback {
		ui.Warning("Using fallback branch")
	}

	ui.NewLine()

	// Get list of local migrations
	localMigrations, err := getLocalMigrations(cfg)
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not list local migrations: %v", err))
	}

	// Get list of applied migrations on remote
	appliedMigrations, err := getAppliedMigrations(info.ProjectRef)
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not check remote migrations: %v", err))
		// Continue anyway - we'll show all local migrations
	}

	// Find pending migrations
	pendingMigrations := findPendingMigrations(localMigrations, appliedMigrations)

	if len(pendingMigrations) == 0 {
		ui.Success("No pending migrations - database is up to date")
		return nil
	}

	// Show pending migrations
	ui.SubHeader(fmt.Sprintf("Pending Migrations (%d)", len(pendingMigrations)))
	for _, m := range pendingMigrations {
		ui.List(m)
	}

	ui.NewLine()

	if migrateDryRunFlag {
		ui.Info("Dry run - no changes made")
		return nil
	}

	// Confirm for production (stricter - requires typing "yes")
	if info.Environment == supabase.EnvProduction {
		confirmed, err := RequireProductionConfirmation(info.Environment, "push migrations")
		if err != nil || !confirmed {
			return nil
		}
	} else if !IsYes() {
		// Normal confirmation for non-production
		confirmed, err := ui.PromptYesNo(fmt.Sprintf("Push %d migration(s) to %s?", len(pendingMigrations), info.SupabaseBranch.Name), true)
		if err != nil || !confirmed {
			ui.Info("Cancelled")
			return nil
		}
	}

	ui.NewLine()

	// Push migrations
	sp = ui.NewSpinner("Pushing migrations")
	sp.Start()

	// Get database URL for this branch (same method used for listing migrations)
	dbURL, urlErr := getDbURLForProject(info.ProjectRef)

	var result *shell.Result
	if urlErr == nil && dbURL != "" {
		// Use --db-url for direct connection (works for preview branches)
		result, err = shell.Run("supabase", "db", "push", "--db-url", dbURL)
	} else {
		// Fallback to --project-ref (mainly for production)
		result, err = shell.Run("supabase", "db", "push", "--project-ref", info.ProjectRef)
	}

	if err != nil {
		sp.Fail("Migration push failed")
		if result != nil && result.Stderr != "" {
			ui.Error(result.Stderr)
		}
		return fmt.Errorf("failed to push migrations: %w", err)
	}

	// Check for actual errors in stderr (not just informational messages)
	if result.Stderr != "" {
		stderrLower := strings.ToLower(result.Stderr)
		// Check for actual SQL errors or migration failures
		if strings.Contains(stderrLower, "error:") || strings.Contains(stderrLower, "sqlstate") {
			sp.Fail("Migration push encountered errors")
			// Extract just the error lines from stderr
			lines := strings.Split(result.Stderr, "\n")
			for _, line := range lines {
				lineLower := strings.ToLower(line)
				if strings.Contains(lineLower, "error") || strings.Contains(lineLower, "sqlstate") || strings.Contains(lineLower, "at statement") {
					ui.Error(strings.TrimSpace(line))
				}
			}
			return fmt.Errorf("migration push failed - see errors above")
		}
	}

	sp.Stop()

	// Show output
	if result.Stdout != "" {
		fmt.Println(result.Stdout)
	}

	ui.NewLine()

	// Verify migrations were actually applied
	sp = ui.NewSpinner("Verifying migrations")
	sp.Start()

	newAppliedMigrations, verifyErr := getAppliedMigrations(info.ProjectRef)
	sp.Stop()

	if verifyErr != nil {
		ui.Warning(fmt.Sprintf("Could not verify migrations: %v", verifyErr))
		ui.Success(fmt.Sprintf("Pushed %d migration(s) - verify with 'drift migrate history'", len(pendingMigrations)))
	} else {
		// Check which migrations are now applied
		appliedCount := 0
		var failedMigrations []string

		for _, m := range pendingMigrations {
			name := strings.TrimSuffix(m, ".sql")
			parts := strings.SplitN(name, "_", 2)
			timestamp := parts[0]

			if newAppliedMigrations[timestamp] {
				appliedCount++
			} else {
				failedMigrations = append(failedMigrations, m)
			}
		}

		if len(failedMigrations) > 0 {
			ui.Warning(fmt.Sprintf("Some migrations may not have been applied:"))
			for _, m := range failedMigrations {
				ui.List(ui.Yellow(m))
			}
			ui.NewLine()
			ui.Infof("Successfully applied: %d/%d migrations", appliedCount, len(pendingMigrations))
			ui.Info("Run 'drift migrate history' for details or try pushing again")
		} else {
			ui.Success(fmt.Sprintf("All %d migration(s) applied successfully", appliedCount))
		}
	}

	// Next steps
	ui.NewLine()
	ui.SubHeader("Next Steps")
	ui.List("drift migrate history     - View migration status")
	ui.List("drift deploy functions    - Deploy edge functions")
	ui.List("drift status              - Check overall project status")

	return nil
}

// getLocalMigrations returns a sorted list of migration filenames from the migrations directory.
func getLocalMigrations(cfg *config.Config) ([]string, error) {
	migrationsDir := cfg.Supabase.MigrationsDir
	if migrationsDir == "" {
		migrationsDir = "supabase/migrations"
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("could not read migrations directory: %w", err)
	}

	var migrations []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".sql") {
			migrations = append(migrations, name)
		}
	}

	sort.Strings(migrations)
	return migrations, nil
}

// getAppliedMigrations queries the remote database for applied migrations.
// Uses getMigrationDetails internally to ensure consistent parsing.
func getAppliedMigrations(projectRef string) (map[string]bool, error) {
	// Reuse getMigrationDetails to ensure consistent parsing
	details, err := getMigrationDetails(projectRef)
	if err != nil {
		return nil, err
	}

	applied := make(map[string]bool)
	for timestamp := range details {
		applied[timestamp] = true
	}

	return applied, nil
}

// getDbURLForProject gets the database URL for a project ref.
func getDbURLForProject(projectRef string) (string, error) {
	client := supabase.NewClient()

	// Find the git branch for this project ref
	branches, err := client.GetBranches()
	if err != nil {
		return "", fmt.Errorf("failed to get branches: %w", err)
	}

	var gitBranch string
	var isProduction bool
	for _, b := range branches {
		if b.ProjectRef == projectRef {
			gitBranch = b.GitBranch
			isProduction = b.IsDefault
			break
		}
	}

	if gitBranch == "" {
		return "", fmt.Errorf("could not find git branch for project ref %s", projectRef)
	}

	// For production, require explicit password and build URL
	if isProduction {
		pw := os.Getenv("PROD_PASSWORD")
		if pw == "" {
			return "", fmt.Errorf("production requires PROD_PASSWORD environment variable")
		}
		// Build production connection URL
		return fmt.Sprintf("postgresql://postgres.%s:%s@aws-0-us-east-1.pooler.supabase.com:6543/postgres", projectRef, pw), nil
	}

	// For non-production, get URL from experimental API
	connInfo, err := client.GetBranchConnectionInfo(gitBranch)
	if err != nil {
		return "", fmt.Errorf("could not get connection info: %w", err)
	}

	if connInfo.PostgresURL == "" {
		return "", fmt.Errorf("could not get database URL from connection info")
	}

	// Use session mode (port 5432) instead of transaction mode (port 6543)
	// Transaction mode doesn't support prepared statements which supabase CLI uses
	url := strings.Replace(connInfo.PostgresURL, ":6543/", ":5432/", 1)

	return url, nil
}

// findPendingMigrations returns migrations that exist locally but aren't applied remotely.
func findPendingMigrations(local []string, applied map[string]bool) []string {
	var pending []string

	for _, migration := range local {
		// Extract timestamp from filename (e.g., "20240101000000_create_users.sql" -> "20240101000000")
		name := strings.TrimSuffix(migration, ".sql")
		parts := strings.SplitN(name, "_", 2)
		timestamp := parts[0]

		if !applied[timestamp] {
			pending = append(pending, migration)
		}
	}

	return pending
}

func runMigrateStatus(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()
	ui.Header("Migration Status")

	// Get current git branch
	gitBranch, err := git.CurrentBranch()
	if err != nil {
		return err
	}

	// Apply override from config
	overrideBranch := cfg.Supabase.OverrideBranch
	client := supabase.NewClient()
	info, err := client.GetBranchInfoWithOverride(gitBranch, overrideBranch)
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not resolve Supabase branch: %v", err))
	} else {
		ui.KeyValue("Git Branch", ui.Cyan(gitBranch))
		ui.KeyValue("Environment", envColorString(string(info.Environment)))
		ui.KeyValue("Supabase Branch", ui.Cyan(info.SupabaseBranch.Name))
		if info.IsOverride {
			ui.Infof("Override: using %s instead of %s", ui.Cyan(info.SupabaseBranch.Name), ui.Cyan(info.OverrideFrom))
		}
	}

	ui.NewLine()
	ui.SubHeader("Migrations")

	// Get DB URL for the project
	sp := ui.NewSpinner("Checking migration status")
	sp.Start()

	var result *shell.Result
	if info != nil {
		dbURL, urlErr := getDbURLForProject(info.ProjectRef)
		if urlErr == nil {
			result, err = shell.Run("supabase", "migration", "list", "--db-url", dbURL)
		} else {
			result, err = shell.Run("supabase", "migration", "list")
		}
	} else {
		result, err = shell.Run("supabase", "migration", "list")
	}

	sp.Stop()

	if err != nil {
		ui.Warning("Could not list migrations")
		return nil
	}

	if result.Stdout != "" {
		fmt.Println(result.Stdout)
	} else {
		ui.Info("No migrations found")
	}

	return nil
}

func runMigrateNew(cmd *cobra.Command, args []string) error {
	name := args[0]

	ui.Infof("Creating migration: %s", name)

	result, err := shell.Run("supabase", "migration", "new", name)
	if err != nil {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("failed to create migration: %s", errMsg)
	}

	if result.Stdout != "" {
		fmt.Println(result.Stdout)
	}

	ui.Success(fmt.Sprintf("Created migration: %s", name))
	return nil
}

func runMigrateHistory(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()

	// Determine target branch
	targetBranch := ""
	if len(args) > 0 {
		targetBranch = args[0]
	} else {
		var err error
		targetBranch, err = git.CurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
	}

	ui.Header("Migration History")

	// Apply override from config
	overrideBranch := cfg.Supabase.OverrideBranch

	// Resolve Supabase branch
	client := supabase.NewClient()
	info, err := client.GetBranchInfoWithOverride(targetBranch, overrideBranch)
	if err != nil {
		return fmt.Errorf("could not resolve Supabase branch: %w", err)
	}

	ui.KeyValue("Git Branch", ui.Cyan(targetBranch))
	ui.KeyValue("Environment", envColorString(string(info.Environment)))
	ui.KeyValue("Supabase Branch", ui.Cyan(info.SupabaseBranch.Name))
	ui.KeyValue("Project Ref", ui.Cyan(info.ProjectRef))

	ui.NewLine()

	// Get local migrations
	localMigrations, err := getLocalMigrations(cfg)
	if err != nil {
		return fmt.Errorf("could not read local migrations: %w", err)
	}

	// Get applied migrations with timestamps
	sp := ui.NewSpinner("Fetching migration history")
	sp.Start()

	migrationInfo, err := getMigrationDetails(info.ProjectRef)
	sp.Stop()

	if err != nil {
		ui.Warning(fmt.Sprintf("Could not fetch remote status: %v", err))
		// Show local only
		ui.SubHeader("Local Migrations")
		for _, m := range localMigrations {
			ui.List(m)
		}
		return nil
	}

	// Count applied and pending
	appliedCount := 0
	pendingCount := 0
	for _, m := range localMigrations {
		name := strings.TrimSuffix(m, ".sql")
		parts := strings.SplitN(name, "_", 2)
		timestamp := parts[0]
		if _, ok := migrationInfo[timestamp]; ok {
			appliedCount++
		} else {
			pendingCount++
		}
	}

	ui.Infof("Total: %d migrations (%d applied, %d pending)", len(localMigrations), appliedCount, pendingCount)
	ui.NewLine()

	// Show table header
	fmt.Printf("  %-6s  %-20s  %-40s\n", "STATUS", "APPLIED AT", "MIGRATION")
	fmt.Printf("  %-6s  %-20s  %-40s\n", "------", "----------", "---------")

	for _, m := range localMigrations {
		name := strings.TrimSuffix(m, ".sql")
		parts := strings.SplitN(name, "_", 2)
		timestamp := parts[0]

		migName := m
		if len(migName) > 50 {
			migName = migName[:47] + "..."
		}

		if appliedAt, ok := migrationInfo[timestamp]; ok {
			// Applied
			fmt.Printf("  %s  %-20s  %s\n", ui.Green("✓ "), appliedAt, migName)
		} else {
			// Pending
			fmt.Printf("  %s  %-20s  %s\n", ui.Yellow("○ "), "pending", migName)
		}
	}

	ui.NewLine()

	if pendingCount > 0 {
		ui.Infof("Run 'drift migrate push' to apply %d pending migration(s)", pendingCount)
	} else {
		ui.Success("All migrations are applied")
	}

	return nil
}

// getMigrationDetails returns a map of migration timestamps to their applied_at times.
// Uses supabase CLI with --db-url from experimental API.
func getMigrationDetails(projectRef string) (map[string]string, error) {
	// Get connection URL for this project
	dbURL, err := getDbURLForProject(projectRef)
	if err != nil {
		return nil, err
	}

	// Run supabase migration list with --db-url
	result, err := shell.Run("supabase", "migration", "list", "--db-url", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to list migrations: %w", err)
	}

	// Check for stderr errors even if exit code is 0
	if result.ExitCode != 0 && result.Stderr != "" {
		return nil, fmt.Errorf("migration list failed: %s", result.Stderr)
	}

	details := make(map[string]string)

	// Parse output - format is typically:
	// LOCAL | REMOTE | TIME (UTC)
	// 20240101000000 | 20240101000000 | 2024-01-01 00:00:00
	lines := strings.Split(result.Stdout, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "LOCAL") || strings.HasPrefix(line, "Local") || strings.HasPrefix(line, "-") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) >= 3 {
			local := strings.TrimSpace(parts[0])
			remote := strings.TrimSpace(parts[1])
			appliedAt := strings.TrimSpace(parts[2])

			if remote != "" && remote != " " && local != "" {
				// Format the timestamp nicely
				if len(appliedAt) > 16 {
					appliedAt = appliedAt[:16] // Trim to "2024-01-01 00:00"
				}
				details[local] = appliedAt
			}
		}
	}

	return details, nil
}

