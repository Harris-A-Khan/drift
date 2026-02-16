package cmd

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/internal/database"
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

type migrationListRow struct {
	Local     string
	Remote    string
	AppliedAt string
}

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
	fmt.Printf("  %-14s  %s\n", "VERSION", "FILE")
	fmt.Printf("  %-14s  %s\n", strings.Repeat("-", 14), strings.Repeat("-", 4))
	for _, m := range pendingMigrations {
		fmt.Printf("  %-14s  %s\n", migrationTimestampFromFilename(m), m)
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

	// Some migrations alter the supabase_realtime publication directly.
	// Ensure it exists before push so these migrations don't fail on branches
	// where Supabase hasn't created it yet.
	dbURL, urlErr := getDbURLForProject(info.ProjectRef)
	if urlErr == nil && dbURL != "" {
		if err := ensureSupabaseRealtimePublication(dbURL); err != nil {
			ui.Warning(fmt.Sprintf("Could not ensure realtime publication: %v", err))
		}
	}

	// Push migrations
	sp = ui.NewSpinner("Pushing migrations")
	sp.Start()

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
			timestamp := migrationTimestampFromFilename(m)

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
		cfg := config.LoadOrDefault()
		poolerHost := cfg.Database.GetPoolerHostForBranch(gitBranch)
		poolerPort := cfg.Database.GetPoolerPort()

		// Use experimental API to discover the correct regional pooler host.
		if connInfo, err := client.GetBranchConnectionInfo(gitBranch); err == nil && connInfo != nil {
			if connInfo.PoolerHost != "" {
				poolerHost = connInfo.PoolerHost
			}
			if connInfo.PoolerPort != 0 {
				poolerPort = connInfo.PoolerPort
			}
		}

		pw := os.Getenv("PROD_PASSWORD")
		if pw == "" {
			if IsYes() {
				return "", fmt.Errorf("production requires PROD_PASSWORD environment variable in non-interactive mode")
			}

			var err error
			pw, err = ui.PromptPassword("Enter production database password")
			if err != nil {
				return "", fmt.Errorf("could not read production database password: %w", err)
			}

			pw = strings.TrimSpace(pw)
			if pw == "" {
				return "", fmt.Errorf("production database password is required")
			}

			// Cache for subsequent production DB lookups in this process.
			_ = os.Setenv("PROD_PASSWORD", pw)
		}
		// Build production connection URL
		return fmt.Sprintf("postgresql://postgres.%s:%s@%s:%d/postgres", projectRef, pw, poolerHost, poolerPort), nil
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

func ensureSupabaseRealtimePublication(dbURL string) error {
	opts, err := restoreOptionsFromDBURL(dbURL)
	if err != nil {
		return err
	}

	sql := `
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_publication WHERE pubname = 'supabase_realtime'
    ) THEN
        CREATE PUBLICATION supabase_realtime;
    END IF;
END $$;`

	result, err := database.ExecuteSQL(opts, sql)
	if err != nil {
		return err
	}
	if result == nil {
		return fmt.Errorf("empty result from publication check")
	}
	if result.ExitCode != 0 {
		if result.Stderr != "" {
			return fmt.Errorf(strings.TrimSpace(result.Stderr))
		}
		return fmt.Errorf("psql exited with code %d while ensuring publication", result.ExitCode)
	}

	return nil
}

func restoreOptionsFromDBURL(dbURL string) (database.RestoreOptions, error) {
	opts := database.DefaultRestoreOptions()

	parsed, err := url.Parse(dbURL)
	if err != nil {
		return opts, fmt.Errorf("invalid db url: %w", err)
	}
	if parsed.Hostname() == "" {
		return opts, fmt.Errorf("invalid db url: missing host")
	}

	opts.Host = parsed.Hostname()
	opts.Database = strings.TrimPrefix(parsed.Path, "/")
	if opts.Database == "" {
		opts.Database = "postgres"
	}

	if parsed.Port() != "" {
		port, err := strconv.Atoi(parsed.Port())
		if err != nil {
			return opts, fmt.Errorf("invalid db url port %q: %w", parsed.Port(), err)
		}
		opts.Port = port
	}

	if parsed.User != nil {
		if username := parsed.User.Username(); username != "" {
			opts.User = username
		}
		if password, ok := parsed.User.Password(); ok {
			opts.Password = password
		}
	}

	return opts, nil
}

// findPendingMigrations returns migrations that exist locally but aren't applied remotely.
func findPendingMigrations(local []string, applied map[string]bool) []string {
	var pending []string

	for _, migration := range local {
		// Extract timestamp from filename (e.g., "20240101000000_create_users.sql" -> "20240101000000")
		timestamp := migrationTimestampFromFilename(migration)

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

	localMigrations, localErr := getLocalMigrations(cfg)
	if localErr != nil {
		ui.Warning(fmt.Sprintf("Could not list local migrations: %v", localErr))
		localMigrations = nil
	}

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
		if !renderMigrationListWithFiles(result.Stdout, localMigrations) {
			fmt.Println(result.Stdout)
		}
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
		timestamp := migrationTimestampFromFilename(m)
		if _, ok := migrationInfo[timestamp]; ok {
			appliedCount++
		} else {
			pendingCount++
		}
	}

	ui.Infof("Total: %d migrations (%d applied, %d pending)", len(localMigrations), appliedCount, pendingCount)
	ui.NewLine()

	// Show table header
	fmt.Printf("  %-6s  %-14s  %-20s  %s\n", "STATUS", "VERSION", "APPLIED AT", "FILE")
	fmt.Printf("  %-6s  %-14s  %-20s  %s\n", "------", "-------", "----------", "----")

	for _, m := range localMigrations {
		timestamp := migrationTimestampFromFilename(m)

		if appliedAt, ok := migrationInfo[timestamp]; ok {
			// Applied
			fmt.Printf("  %s  %-14s  %-20s  %s\n", ui.Green("✓ "), timestamp, appliedAt, m)
		} else {
			// Pending
			fmt.Printf("  %s  %-14s  %-20s  %s\n", ui.Yellow("○ "), timestamp, "pending", m)
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
	for _, row := range parseMigrationListRows(result.Stdout) {
		if row.Remote == "" || row.Local == "" {
			continue
		}
		appliedAt := row.AppliedAt
		if len(appliedAt) > 16 {
			appliedAt = appliedAt[:16] // Trim to "2024-01-01 00:00"
		}
		details[row.Local] = appliedAt
	}

	return details, nil
}

func parseMigrationListRows(output string) []migrationListRow {
	lines := strings.Split(output, "\n")
	rows := make([]migrationListRow, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		lowerTrimmed := strings.ToLower(trimmed)
		if strings.HasPrefix(lowerTrimmed, "local") || strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "─") {
			continue
		}

		if !strings.Contains(line, "|") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}

		row := migrationListRow{
			Local:     strings.TrimSpace(parts[0]),
			Remote:    strings.TrimSpace(parts[1]),
			AppliedAt: strings.TrimSpace(parts[2]),
		}

		if row.Local == "" && row.Remote == "" {
			continue
		}

		rows = append(rows, row)
	}

	return rows
}

func migrationTimestampFromFilename(filename string) string {
	name := strings.TrimSuffix(filename, ".sql")
	parts := strings.SplitN(name, "_", 2)
	return strings.TrimSpace(parts[0])
}

func buildMigrationFilenameIndex(localMigrations []string) map[string]string {
	index := make(map[string]string, len(localMigrations))
	for _, migration := range localMigrations {
		timestamp := migrationTimestampFromFilename(migration)
		if timestamp != "" {
			index[timestamp] = migration
		}
	}
	return index
}

func migrationFileForRow(row migrationListRow, filenameByTimestamp map[string]string) string {
	timestamp := row.Local
	if timestamp == "" {
		timestamp = row.Remote
	}
	if timestamp == "" {
		return "-"
	}

	if filename, ok := filenameByTimestamp[timestamp]; ok {
		return filename
	}

	return "-"
}

func renderMigrationListWithFiles(rawOutput string, localMigrations []string) bool {
	rows := parseMigrationListRows(rawOutput)
	if len(rows) == 0 {
		return false
	}

	filenameByTimestamp := buildMigrationFilenameIndex(localMigrations)

	fmt.Printf("  %-14s | %-14s | %-19s | %s\n", "Local", "Remote", "Time (UTC)", "File")
	fmt.Printf("  %-14s-|-%-14s-|-%-19s-|-%s\n", strings.Repeat("-", 14), strings.Repeat("-", 14), strings.Repeat("-", 19), strings.Repeat("-", 4))

	for _, row := range rows {
		fmt.Printf(
			"  %-14s | %-14s | %-19s | %s\n",
			row.Local,
			row.Remote,
			row.AppliedAt,
			migrationFileForRow(row, filenameByTimestamp),
		)
	}

	return true
}
