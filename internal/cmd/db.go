package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/internal/database"
	"github.com/undrift/drift/internal/git"
	"github.com/undrift/drift/internal/supabase"
	"github.com/undrift/drift/internal/ui"
	"github.com/undrift/drift/pkg/shell"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database operations",
	Long: `Manage database dumps and restores.

The db command helps you dump and restore databases between 
different Supabase environments (production, development, feature branches).`,
}

var dbDumpCmd = &cobra.Command{
	Use:   "dump <env> [filename]",
	Short: "Dump database (prod|dev)",
	Long: `Dump a database from the specified environment.

This creates a complete database dump including:
  - All schemas (public, auth, storage, etc.)
  - All data including auth.users (user accounts)
  - All migrations (supabase_migrations schema)

The dump is in plain SQL format and can be restored with 'drift db push'.

Examples:
  drift db dump prod              # Dump to backups/prod_YYYYMMDD_HHMMSS.backup (default)
  drift db dump dev               # Dump to backups/dev_YYYYMMDD_HHMMSS.backup (default)
  drift db dump prod mybackup     # Dump to mybackup.backup
  drift db dump prod -o custom.sql  # Dump to custom.sql`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runDbDump,
}

var dbPushCmd = &cobra.Command{
	Use:   "push [target]",
	Short: "Push backup to target branch",
	Long: `Restore a database backup to a target environment.

If no target is specified, shows an interactive branch picker.

Examples:
  drift db push           # Interactive: select from all branches
  drift db push dev       # Push prod backup to development
  drift db push feature   # Push dev backup to current feature branch
  drift db push feature -i prod_20260215_143000.backup`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDbPush,
}

var dbSeedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Generate seed.sql for new branches",
	Long: `Generate a seed.sql file that will be used to populate new Supabase branches.

This dumps auth.users and selected public tables to supabase/seed.sql.
New preview branches will automatically have this data.

Examples:
  drift db seed                    # Generate seed.sql from dev
  drift db seed --source prod      # Generate from production
  drift db seed --tables users,profiles  # Only specific tables`,
	RunE: runDbSeed,
}

var dbListCmd = &cobra.Command{
	Use:   "list [filter]",
	Short: "List local backups",
	Long: `List available local database backups.

Optional filters:
  prod / production     List production backups (prod*.backup)
  dev / development     List development backups (dev*.backup)
  <name>.backup         List an exact backup file name`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDbList,
}

var (
	dbOutputFlag   string
	dbInputFlag    string
	dbPasswordFlag string
	dbSeedSource   string
	dbSeedTables   string
)

func init() {
	dbDumpCmd.Flags().StringVarP(&dbOutputFlag, "output", "o", "", "Output file path")
	dbPushCmd.Flags().StringVarP(&dbInputFlag, "input", "i", "", "Input backup file")
	dbPushCmd.Flags().StringVar(&dbPasswordFlag, "password", "", "Target database password (or use env var)")
	dbSeedCmd.Flags().StringVar(&dbSeedSource, "source", "dev", "Source environment (prod|dev)")
	dbSeedCmd.Flags().StringVar(&dbSeedTables, "tables", "", "Comma-separated list of public tables to include")

	dbCmd.AddCommand(dbDumpCmd)
	dbCmd.AddCommand(dbPushCmd)
	dbCmd.AddCommand(dbListCmd)
	dbCmd.AddCommand(dbSeedCmd)
	rootCmd.AddCommand(dbCmd)
}

func getDbPassword(env string) string {
	// Check flag first
	if dbPasswordFlag != "" {
		return dbPasswordFlag
	}

	// Check environment variables
	switch env {
	case "prod", "production":
		if pw := os.Getenv("PROD_PASSWORD"); pw != "" {
			return pw
		}
	case "dev", "development":
		if pw := os.Getenv("DEV_PASSWORD"); pw != "" {
			return pw
		}
	}

	// Check generic password
	if pw := os.Getenv("DB_PASSWORD"); pw != "" {
		return pw
	}

	return ""
}

func getProjectRef(env string) string {
	switch env {
	case "prod", "production":
		if ref := os.Getenv("PROD_PROJECT_REF"); ref != "" {
			return ref
		}
	case "dev", "development":
		if ref := os.Getenv("DEV_PROJECT_REF"); ref != "" {
			return ref
		}
	}
	return ""
}

func runDbDump(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	env := args[0]
	cfg := config.LoadOrDefault()

	// Validate environment
	if env != "prod" && env != "production" && env != "dev" && env != "development" {
		return fmt.Errorf("invalid environment: %s (use prod or dev)", env)
	}

	isProd := env == "prod" || env == "production"
	envName := "Production"
	if !isProd {
		envName = "Development"
	}

	ui.Header(fmt.Sprintf("Database Dump - %s", envName))

	// Get branch info from Supabase
	client := supabase.NewClient()
	var branch *supabase.Branch
	var err error

	if isProd {
		branch, err = client.GetProductionBranch()
	} else {
		branch, err = client.GetDevelopmentBranch()
	}

	if err != nil {
		return fmt.Errorf("failed to get %s branch: %w", envName, err)
	}

	projectRef := branch.ProjectRef
	gitBranch := branch.GitBranch

	// Get connection info using experimental API (includes correct pooler host)
	connInfo, err := client.GetBranchConnectionInfo(gitBranch)
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not get connection info via API: %v", err))
	}

	// Get password - for non-production, API has the real password (no env var needed)
	var password string
	if !isProd && connInfo != nil && connInfo.PostgresURL != "" {
		// Non-production: API returns actual password
		password = supabase.ExtractPasswordFromURL(connInfo.PostgresURL)
	}
	// Fallback to env vars (required for production, optional for others)
	if password == "" {
		password = getDbPassword(env)
	}
	// Last resort: prompt
	if password == "" {
		password, err = ui.PromptPassword("Database password")
		if err != nil {
			return err
		}
	}

	// Determine pooler host - prefer from API, fallback to config
	var poolerHost string
	if connInfo != nil && connInfo.PoolerHost != "" {
		poolerHost = connInfo.PoolerHost
	} else {
		// Fallback to config (shouldn't happen if API works)
		poolerHost = cfg.Database.PoolerHost
	}

	// Use session mode (port 5432) for pg_dump
	poolerPort := 5432
	poolerUser := fmt.Sprintf("postgres.%s", projectRef)

	ui.KeyValue("Environment", envColorString(envName))
	ui.KeyValue("Project Ref", ui.Cyan(projectRef))
	ui.KeyValue("Pooler", fmt.Sprintf("%s:%d", poolerHost, poolerPort))

	// Confirm for production
	if isProd && !IsYes() {
		ui.NewLine()
		ui.Warning("You are about to dump PRODUCTION database")
		confirmed, err := ui.PromptYesNo("Continue?", true)
		if err != nil || !confirmed {
			ui.Info("Cancelled")
			return nil
		}
	}

	// Set up dump options using pooler connection
	opts := database.DefaultDumpOptions()
	opts.Host = poolerHost
	opts.Port = poolerPort
	opts.User = poolerUser
	opts.Password = password

	// Determine output filename: -o flag > positional arg > default
	if dbOutputFlag != "" {
		opts.OutputFile = dbOutputFlag
	} else if len(args) > 1 {
		// User provided filename as second argument
		filename := args[1]
		// Add .backup extension if not present
		if !strings.HasSuffix(filename, ".backup") && !strings.HasSuffix(filename, ".sql") {
			filename = filename + ".backup"
		}
		opts.OutputFile = filename
	} else {
		// Default based on environment
		prefix := "prod"
		if !isProd {
			prefix = "dev"
		}
		backupDir := cfg.GetBackupPath()
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			return fmt.Errorf("failed to create backup directory %s: %w", backupDir, err)
		}
		opts.OutputFile = filepath.Join(backupDir, timestampedBackupFilename(prefix, time.Now()))
	}

	ui.NewLine()

	// Perform dump (includes validation for empty/small files)
	sp := ui.NewSpinner(fmt.Sprintf("Dumping database to %s", opts.OutputFile))
	sp.Start()

	if err := database.Dump(opts); err != nil {
		sp.Fail("Dump failed")
		return err
	}

	sp.Success(fmt.Sprintf("Database dumped to %s", opts.OutputFile))

	// Show file size
	if info, err := os.Stat(opts.OutputFile); err == nil {
		sizeMB := float64(info.Size()) / 1024 / 1024
		ui.KeyValue("File Size", fmt.Sprintf("%.2f MB", sizeMB))
	}

	// Next steps
	ui.NewLine()
	ui.SubHeader("What's included")
	ui.List("All schemas (public, auth, storage, extensions, etc.)")
	ui.List("All data including auth.users (user accounts)")
	ui.List("All migrations (supabase_migrations)")

	ui.NewLine()
	ui.SubHeader("Next Steps")
	ui.List("drift db push <target>    - Push this backup to another branch")
	ui.List("drift db list             - View all local backups")

	return nil
}

func runDbPush(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}

	client := supabase.NewClient()

	// Validate target
	var sourceFile string
	var targetEnv string
	var targetBranch *supabase.Branch
	var targetGitBranch string

	// If no target specified, show interactive branch picker
	if len(args) == 0 {
		branches, err := client.GetBranches()
		if err != nil {
			return fmt.Errorf("failed to get branches: %w", err)
		}

		// Filter out production branch (can't push to prod)
		var selectableBranches []supabase.Branch
		for _, b := range branches {
			if !b.IsDefault { // Exclude production
				selectableBranches = append(selectableBranches, b)
			}
		}

		if len(selectableBranches) == 0 {
			return fmt.Errorf("no non-production branches available to push to")
		}

		// Build options for picker
		options := make([]string, len(selectableBranches))
		for i, b := range selectableBranches {
			envType := "Feature"
			if b.Persistent {
				envType = "Development"
			}
			options[i] = fmt.Sprintf("%s (%s) → %s", b.GitBranch, envType, b.ProjectRef)
		}

		ui.Header("Select Target Branch")
		selected, err := ui.PromptSelect("Push backup to", options)
		if err != nil {
			return fmt.Errorf("branch selection cancelled: %w", err)
		}

		// Find the selected branch
		for i, opt := range options {
			if opt == selected {
				targetBranch = &selectableBranches[i]
				targetGitBranch = targetBranch.GitBranch
				if targetBranch.Persistent {
					targetEnv = "Development"
					sourceFile = "prod.backup"
				} else {
					targetEnv = "Feature"
					sourceFile = "dev.backup"
				}
				break
			}
		}

		if targetBranch == nil {
			return fmt.Errorf("no branch selected")
		}
	} else {
		target := args[0]

		switch target {
		case "dev", "development":
			sourceFile = "prod.backup"
			targetEnv = "Development"
			branch, err := client.GetDevelopmentBranch()
			if err != nil {
				return fmt.Errorf("failed to get development branch: %w", err)
			}
			targetBranch = branch
			targetGitBranch = branch.GitBranch
		case "feature":
			sourceFile = "dev.backup"
			targetEnv = "Feature"
			// Get project ref from current branch
			gitBranch, err := git.CurrentBranch()
			if err != nil {
				return err
			}
			branch, _, err := client.ResolveBranch(gitBranch)
			if err != nil {
				return err
			}
			if branch == nil {
				return fmt.Errorf("no Supabase branch found for '%s'", gitBranch)
			}
			targetBranch = branch
			targetGitBranch = gitBranch
		default:
			// Try to find branch by name
			branch, err := client.GetBranch(target)
			if err != nil {
				return fmt.Errorf("invalid target '%s': not a known target (dev, feature) or branch name", target)
			}
			if branch.IsDefault {
				return fmt.Errorf("cannot push to production branch")
			}
			targetBranch = branch
			targetGitBranch = branch.GitBranch
			if branch.Persistent {
				targetEnv = "Development"
				sourceFile = "prod.backup"
			} else {
				targetEnv = "Feature"
				sourceFile = "dev.backup"
			}
		}
	}

	targetProjectRef := targetBranch.ProjectRef
	cfg := config.LoadOrDefault()
	backups, err := discoverLocalBackups(cfg)
	if err != nil {
		return err
	}
	expectedSourceFile := sourceFile
	sourcePrefix := "dev"
	if strings.HasPrefix(strings.ToLower(expectedSourceFile), "prod") {
		sourcePrefix = "prod"
	}

	// Override source file if specified via flag
	if dbInputFlag != "" {
		sourceFile, err = resolveBackupInputPath(dbInputFlag, backups)
		if err != nil {
			if len(backups) == 0 {
				return fmt.Errorf("%w\nNo backup files found in %s or %s", err, cfg.GetBackupPath(), cfg.ProjectRoot())
			}
			return fmt.Errorf("%w\nRun 'drift db list' to view available backups", err)
		}
	} else {
		if exact := findLocalBackupByName(backups, expectedSourceFile); exact != nil {
			sourceFile = exact.Path
		} else {
			if len(backups) == 0 {
				return fmt.Errorf("backup file not found: %s\nNo backup files found in %s or %s\nRun 'drift db dump %s' first", expectedSourceFile, cfg.GetBackupPath(), cfg.ProjectRoot(), sourcePrefix)
			}

			suggested := suggestLocalBackup(backups, expectedSourceFile, sourcePrefix)
			if suggested == nil {
				return fmt.Errorf("no backup files found")
			}

			if IsYes() {
				ui.Warningf("Expected backup %s not found", expectedSourceFile)
				ui.Infof("Using suggested backup: %s", backupDisplayPath(suggested.Path, cfg.ProjectRoot()))
				sourceFile = suggested.Path
			} else {
				ui.Warningf("Expected backup %s not found", expectedSourceFile)

				options := make([]string, len(backups))
				lookup := make(map[string]localBackupFile, len(backups))
				for i, backup := range backups {
					sizeMB := float64(backup.SizeBytes) / 1024 / 1024
					marker := ""
					if backup.Path == suggested.Path {
						marker = " (suggested)"
					}
					option := fmt.Sprintf("%s  %.2f MB  %s%s", backupDisplayPath(backup.Path, cfg.ProjectRoot()), sizeMB, formatBackupAge(backup.ModTime), marker)
					options[i] = option
					lookup[option] = backup
				}

				ui.Header("Select Backup File")
				selected, err := ui.PromptSelect("Use backup", options)
				if err != nil {
					return fmt.Errorf("backup selection cancelled: %w", err)
				}

				sourceFile = lookup[selected].Path
			}
		}
	}

	// Check source file exists
	if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s\nRun 'drift db dump' first", sourceFile)
	}

	// Check backup freshness
	if info, err := os.Stat(sourceFile); err == nil {
		age := time.Since(info.ModTime())
		if age > 24*time.Hour {
			ui.Warningf("Backup is %.0f hours old", age.Hours())
			if !IsYes() {
				refresh, _ := ui.PromptYesNo("Refresh backup first?", true)
				if refresh {
					// Determine source env based on target type
					dumpEnv := "prod"
					if targetEnv == "Feature" {
						dumpEnv = "dev"
					}
					if err := runDbDump(cmd, []string{dumpEnv}); err != nil {
						return err
					}

					refreshedBackups, discoverErr := discoverLocalBackups(cfg)
					if discoverErr == nil {
						if refreshed := suggestLocalBackup(refreshedBackups, "", sourcePrefix); refreshed != nil {
							sourceFile = refreshed.Path
						}
					}
				}
			}
		}
	}

	ui.NewLine()
	ui.KeyValue("Selected Backup", ui.Cyan(backupDisplayPath(sourceFile, cfg.ProjectRoot())))
	if !IsYes() {
		confirmed, err := ui.PromptYesNo("Use selected backup?", true)
		if err != nil || !confirmed {
			ui.Info("Cancelled")
			return nil
		}
	}

	ui.Header(fmt.Sprintf("Database Push - %s", targetEnv))

	// Get connection info using experimental API (includes correct pooler host)
	connInfo, err := client.GetBranchConnectionInfo(targetGitBranch)
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not get connection info via API: %v", err))
	}

	// Get password - for non-production, API has the real password (no need for env vars)
	var password string
	if connInfo != nil && connInfo.PostgresURL != "" {
		// Non-production branches: API returns the actual password
		password = supabase.ExtractPasswordFromURL(connInfo.PostgresURL)
	}
	// Fallback to env vars if API didn't return password
	if password == "" {
		password = getDbPassword("dev")
	}
	// Last resort: prompt
	if password == "" {
		password, err = ui.PromptPassword("Target database password")
		if err != nil {
			return err
		}
	}

	// Determine pooler host - prefer from API, fallback to config
	var poolerHost string
	if connInfo != nil && connInfo.PoolerHost != "" {
		poolerHost = connInfo.PoolerHost
	} else {
		// Fallback to config (shouldn't happen if API works)
		poolerHost = cfg.Database.PoolerHost
	}

	// Use transaction mode (port 6543) for psql restore - matches push-db-to-branch.sh
	poolerPort := 6543
	poolerUser := fmt.Sprintf("postgres.%s", targetProjectRef)

	ui.KeyValue("Source", sourceFile)
	ui.KeyValue("Target", envColorString(targetEnv))
	ui.KeyValue("Project Ref", ui.Cyan(targetProjectRef))
	ui.KeyValue("Pooler", fmt.Sprintf("%s:%d", poolerHost, poolerPort))

	// Confirm - stricter for development (permanent branch) vs feature (preview)
	if targetEnv == "Development" {
		// Development is a persistent branch, require stricter confirmation
		confirmed, err := ConfirmDestructiveOperation("REPLACE the development database with this backup")
		if err != nil || !confirmed {
			return nil
		}
	} else if !IsYes() {
		ui.NewLine()
		ui.Warning("This will REPLACE the target database!")
		confirmed, err := ui.PromptYesNo("Continue?", false)
		if err != nil || !confirmed {
			ui.Info("Cancelled")
			return nil
		}
	}

	ui.NewLine()

	// Set up restore options using pooler connection
	opts := database.DefaultRestoreOptions()
	opts.Host = poolerHost
	opts.Port = poolerPort
	opts.User = poolerUser
	opts.Password = password
	opts.InputFile = sourceFile

	// Perform restore
	sp := ui.NewSpinner(fmt.Sprintf("Restoring database from %s", sourceFile))
	sp.Start()

	if err := database.Restore(opts); err != nil {
		sp.Fail("Restore failed")
		return err
	}

	sp.Success("Database restored successfully")

	// Show next steps
	ui.NewLine()
	ui.SubHeader("Next Steps")
	ui.List("drift migrate history    - Check if migrations need to be applied")
	ui.List("drift migrate push       - Apply any pending migrations")
	ui.List("drift deploy functions   - Deploy edge functions")
	ui.List("drift secrets copy       - Copy secrets from dev to this branch")
	ui.List("drift status             - Check service health")

	return nil
}

func runDbList(cmd *cobra.Command, args []string) error {
	ui.Header("Local Database Backups")
	cfg := config.LoadOrDefault()
	backups, err := discoverLocalBackups(cfg)
	if err != nil {
		return err
	}
	if len(args) > 0 {
		backups = filterLocalBackups(backups, args[0])
	}

	if len(backups) == 0 {
		ui.Info("No backup files found")
		return nil
	}

	for _, backup := range backups {
		sizeMB := float64(backup.SizeBytes) / 1024 / 1024
		fmt.Printf("  %s  %s  %s\n",
			ui.Cyan(backupDisplayPath(backup.Path, cfg.ProjectRoot())),
			ui.Dim(fmt.Sprintf("%.2f MB", sizeMB)),
			ui.Dim(formatBackupAge(backup.ModTime)),
		)
	}

	return nil
}

func runDbSeed(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()

	ui.Header("Generate Seed Data")

	// Interactive source selection if not specified via flag
	source := dbSeedSource
	if !cmd.Flags().Changed("source") {
		options := []string{
			"prod - Production database",
			"dev - Development database",
		}
		selected, err := ui.PromptSelect("Select source database", options)
		if err != nil {
			return err
		}
		source = strings.Split(selected, " ")[0]
	}

	if source != "prod" && source != "production" && source != "dev" && source != "development" {
		return fmt.Errorf("invalid source: %s (use prod or dev)", source)
	}

	isProd := source == "prod" || source == "production"
	sourceName := "Development"
	if isProd {
		sourceName = "Production"
	}

	ui.NewLine()
	ui.KeyValue("Source", envColorString(sourceName))

	// Get branch info
	client := supabase.NewClient()
	var branch *supabase.Branch
	var err error

	if isProd {
		branch, err = client.GetProductionBranch()
	} else {
		branch, err = client.GetDevelopmentBranch()
	}

	if err != nil {
		return fmt.Errorf("failed to get %s branch: %w", sourceName, err)
	}

	projectRef := branch.ProjectRef
	gitBranch := branch.GitBranch

	// Get connection info
	connInfo, err := client.GetBranchConnectionInfo(gitBranch)
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not get connection info via API: %v", err))
	}

	// Get password - for non-production, API has the real password
	var password string
	if !isProd && connInfo != nil && connInfo.PostgresURL != "" {
		password = supabase.ExtractPasswordFromURL(connInfo.PostgresURL)
	}
	if password == "" {
		password = getDbPassword(source)
	}
	if password == "" {
		password, err = ui.PromptPassword("Database password")
		if err != nil {
			return err
		}
	}

	// Determine pooler host
	var poolerHost string
	if connInfo != nil && connInfo.PoolerHost != "" {
		poolerHost = connInfo.PoolerHost
	} else {
		poolerHost = cfg.Database.PoolerHost
	}

	poolerPort := 5432
	poolerUser := fmt.Sprintf("postgres.%s", projectRef)

	// Query for public tables if not specified via flag
	var selectedTables []string
	if !cmd.Flags().Changed("tables") {
		ui.NewLine()
		sp := ui.NewSpinner("Fetching public tables")
		sp.Start()

		publicTables, err := getPublicTables(poolerHost, poolerPort, poolerUser, password)
		sp.Stop()

		if err != nil {
			ui.Warning(fmt.Sprintf("Could not fetch tables: %v", err))
			ui.Info("Using default: profiles")
			selectedTables = []string{"profiles"}
		} else if len(publicTables) == 0 {
			ui.Info("No public tables found")
		} else {
			// Add auth.users as an option (but not selected by default due to permission issues)
			allTables := append([]string{"auth.users (requires special permissions)"}, publicTables...)
			defaultSelected := []string{}
			// Pre-select profiles if it exists
			for _, t := range publicTables {
				if t == "profiles" {
					defaultSelected = append(defaultSelected, t)
					break
				}
			}

			ui.NewLine()
			ui.Info("Select tables to include in seed.sql")
			ui.Warning("auth.users may fail due to Supabase permissions")
			ui.NewLine()

			// Show multi-select for tables
			selected, err := ui.PromptMultiSelect("Select tables to seed", allTables, defaultSelected)
			if err != nil {
				return err
			}

			// Clean up auth.users selection (remove the note)
			for _, t := range selected {
				if strings.HasPrefix(t, "auth.users") {
					selectedTables = append(selectedTables, "auth.users")
				} else {
					selectedTables = append(selectedTables, t)
				}
			}
		}
	} else {
		// Parse from flag
		for _, t := range strings.Split(dbSeedTables, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				selectedTables = append(selectedTables, t)
			}
		}
	}

	ui.NewLine()
	ui.KeyValue("Project Ref", ui.Cyan(projectRef))
	ui.KeyValue("Pooler", fmt.Sprintf("%s:%d", poolerHost, poolerPort))

	// Determine output path
	outputPath := filepath.Join(cfg.Supabase.MigrationsDir, "..", "seed.sql")
	outputPath = filepath.Clean(outputPath)

	ui.KeyValue("Output", outputPath)

	// Build tables to dump
	var tables []string
	for _, t := range selectedTables {
		if t == "auth.users" {
			tables = append(tables, "auth.users")
		} else if !strings.Contains(t, ".") {
			tables = append(tables, "public."+t)
		} else {
			tables = append(tables, t)
		}
	}

	ui.NewLine()
	ui.Info(fmt.Sprintf("Tables to seed: %s", strings.Join(tables, ", ")))
	ui.NewLine()

	// Confirm
	if !IsYes() {
		confirmed, err := ui.PromptYesNo("Generate seed.sql with these tables?", true)
		if err != nil || !confirmed {
			ui.Info("Cancelled")
			return nil
		}
	}

	ui.NewLine()

	// Build pg_dump args for seed data
	opts := database.DumpOptions{
		Host:         poolerHost,
		Port:         poolerPort,
		User:         poolerUser,
		Password:     password,
		Database:     "postgres",
		OutputFile:   outputPath,
		Format:       "plain",
		DataOnly:     true,
		NoOwner:      true,
		NoPrivileges: true,
	}

	sp := ui.NewSpinner("Generating seed.sql")
	sp.Start()

	if err := dumpTablesToSeed(opts, tables); err != nil {
		sp.Fail("Failed to generate seed")
		return err
	}

	sp.Success("Seed file generated")

	// Show file info
	if info, err := os.Stat(outputPath); err == nil {
		sizeKB := float64(info.Size()) / 1024
		ui.KeyValue("File Size", fmt.Sprintf("%.2f KB", sizeKB))
	}

	ui.NewLine()
	ui.Success("Seed file created at: " + outputPath)
	ui.Info("New Supabase branches will be seeded with this data")

	return nil
}

// getPublicTables queries the database for all tables in the public schema.
func getPublicTables(host string, port int, user, password string) ([]string, error) {
	psql, err := exec.LookPath("psql")
	if err != nil {
		return nil, fmt.Errorf("psql not found: %w", err)
	}

	query := `SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename;`

	args := []string{
		"-h", host,
		"-p", fmt.Sprintf("%d", port),
		"-U", user,
		"-d", "postgres",
		"-t", "-A", "-c", query,
	}

	env := map[string]string{
		"PGPASSWORD": password,
	}

	result, err := shell.RunWithEnv(env, psql, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	var tables []string
	for _, line := range strings.Split(result.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			tables = append(tables, line)
		}
	}

	return tables, nil
}

// dumpTablesToSeed dumps specific tables to a seed file.
// Produces clean SQL compatible with Supabase's seed runner.
func dumpTablesToSeed(opts database.DumpOptions, tables []string) error {
	pgDump, err := database.FindPGDump()
	if err != nil {
		return err
	}

	// Create temp file for raw dump
	tempFile := opts.OutputFile + ".tmp"

	args := []string{
		"-h", opts.Host,
		"-p", fmt.Sprintf("%d", opts.Port),
		"-U", opts.User,
		"-d", opts.Database,
		"-f", tempFile,
		"--data-only",
		"--column-inserts", // Full column names for clarity
		"--no-owner",
		"--no-privileges",
		"--no-comments",
		// Note: Don't use --disable-triggers as it requires table ownership
	}

	// Add each table
	for _, t := range tables {
		args = append(args, "-t", t)
	}

	env := map[string]string{
		"PGPASSWORD": opts.Password,
	}

	result, err := shell.RunWithEnv(env, pgDump, args...)
	if err != nil {
		os.Remove(tempFile)
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = err.Error()
		}
		// Check if it's just "table not found" - that's okay
		if strings.Contains(strings.ToLower(errMsg), "no matching tables") ||
			strings.Contains(strings.ToLower(errMsg), "did not find any relation") {
			// Create empty seed file
			return os.WriteFile(opts.OutputFile, []byte("-- No matching tables found\n"), 0644)
		}
		return fmt.Errorf("pg_dump failed: %s", errMsg)
	}

	// Post-process: remove psql-specific commands that break Supabase seeding
	if err := cleanSeedFile(tempFile, opts.OutputFile); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to clean seed file: %w", err)
	}

	os.Remove(tempFile)
	return nil
}

// cleanSeedFile removes psql-specific commands from a dump file.
// Supabase seed runner doesn't support psql metacommands like \connect, \.
func cleanSeedFile(inputPath, outputPath string) error {
	input, err := os.ReadFile(inputPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(input), "\n")
	var cleanLines []string

	// Check if we have auth.users data
	hasAuthUsers := strings.Contains(string(input), "auth.users")

	// Add header
	cleanLines = append(cleanLines, "-- Seed file generated by drift")
	cleanLines = append(cleanLines, "-- This file is auto-applied when creating new Supabase branches")
	cleanLines = append(cleanLines, "")

	// If we have auth.users, we need special handling
	if hasAuthUsers {
		cleanLines = append(cleanLines, "-- Disable triggers for auth.users seeding (allows INSERT without ownership)")
		cleanLines = append(cleanLines, "SET session_replication_role = 'replica';")
		cleanLines = append(cleanLines, "")
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip psql metacommands (start with \)
		if strings.HasPrefix(trimmed, "\\") {
			continue
		}

		// Skip problematic SET commands
		if strings.HasPrefix(trimmed, "SET ") {
			// Keep some safe SET commands, skip others
			lower := strings.ToLower(trimmed)
			if strings.Contains(lower, "search_path") ||
				strings.Contains(lower, "client_encoding") ||
				strings.Contains(lower, "standard_conforming_strings") {
				// These are generally safe
				cleanLines = append(cleanLines, line)
			}
			// Skip other SET commands that might cause issues
			continue
		}

		// Skip SELECT pg_catalog.set_config
		if strings.Contains(strings.ToLower(trimmed), "pg_catalog.set_config") {
			continue
		}

		cleanLines = append(cleanLines, line)
	}

	// Restore session_replication_role if we changed it
	if hasAuthUsers {
		cleanLines = append(cleanLines, "")
		cleanLines = append(cleanLines, "-- Restore normal trigger behavior")
		cleanLines = append(cleanLines, "SET session_replication_role = 'origin';")
	}

	output := strings.Join(cleanLines, "\n")
	return os.WriteFile(outputPath, []byte(output), 0644)
}
