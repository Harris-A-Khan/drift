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

	// Confirm for production (stricter)
	if info.Environment == supabase.EnvProduction && !IsYes() {
		ui.Warning("You are about to push migrations to PRODUCTION!")
		confirmed, err := ui.PromptYesNo("Are you absolutely sure?", false)
		if err != nil || !confirmed {
			ui.Info("Cancelled")
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

	pushArgs := []string{"db", "push", "--project-ref", info.ProjectRef}

	result, err := shell.Run("supabase", pushArgs...)
	if err != nil {
		sp.Fail("Migration push failed")
		if result.Stderr != "" {
			ui.Error(result.Stderr)
		}
		return fmt.Errorf("failed to push migrations: %w", err)
	}
	sp.Stop()

	// Show output
	if result.Stdout != "" {
		fmt.Println(result.Stdout)
	}

	ui.NewLine()
	ui.Success(fmt.Sprintf("Pushed %d migration(s) successfully", len(pendingMigrations)))

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
func getAppliedMigrations(projectRef string) (map[string]bool, error) {
	// Run supabase migration list to get applied migrations
	result, err := shell.Run("supabase", "migration", "list", "--project-ref", projectRef)
	if err != nil {
		return nil, err
	}

	applied := make(map[string]bool)

	// Parse output - format is typically:
	// LOCAL | REMOTE | TIME
	// 20240101000000 | 20240101000000 | 2024-01-01 00:00:00
	lines := strings.Split(result.Stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "LOCAL") || strings.HasPrefix(line, "-") {
			continue
		}

		// Split by | and check if REMOTE column has a value
		parts := strings.Split(line, "|")
		if len(parts) >= 2 {
			remote := strings.TrimSpace(parts[1])
			if remote != "" && remote != " " {
				// This migration is applied remotely
				local := strings.TrimSpace(parts[0])
				if local != "" {
					applied[local] = true
				}
			}
		}
	}

	return applied, nil
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
	ui.SubHeader("Local Migrations")

	// List local migrations
	result, err := shell.Run("supabase", "migration", "list")
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
func getMigrationDetails(projectRef string) (map[string]string, error) {
	result, err := shell.Run("supabase", "migration", "list", "--project-ref", projectRef)
	if err != nil {
		return nil, err
	}

	details := make(map[string]string)

	// Parse output - format is typically:
	// LOCAL | REMOTE | TIME (UTC)
	// 20240101000000 | 20240101000000 | 2024-01-01 00:00:00
	lines := strings.Split(result.Stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "LOCAL") || strings.HasPrefix(line, "-") {
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

