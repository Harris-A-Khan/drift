package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/internal/database"
	"github.com/undrift/drift/internal/git"
	"github.com/undrift/drift/internal/supabase"
	"github.com/undrift/drift/internal/ui"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database operations",
	Long: `Manage database dumps and restores.

The db command helps you dump and restore databases between 
different Supabase environments (production, development, feature branches).`,
}

var dbDumpCmd = &cobra.Command{
	Use:   "dump <env>",
	Short: "Dump database (prod|dev)",
	Long: `Dump a database from the specified environment.

Examples:
  drift db dump prod     # Dump production database
  drift db dump dev      # Dump development database`,
	Args: cobra.ExactArgs(1),
	RunE: runDbDump,
}

var dbPushCmd = &cobra.Command{
	Use:   "push <target>",
	Short: "Push backup to target (dev|feature)",
	Long: `Restore a database backup to a target environment.

Examples:
  drift db push dev      # Push prod backup to development
  drift db push feature  # Push dev backup to current feature branch`,
	Args: cobra.ExactArgs(1),
	RunE: runDbPush,
}

var dbListCmd = &cobra.Command{
	Use:   "list <env>",
	Short: "List local backups",
	Long:  `List available local database backups.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runDbList,
}

var (
	dbOutputFlag   string
	dbInputFlag    string
	dbPasswordFlag string
)

func init() {
	dbDumpCmd.Flags().StringVarP(&dbOutputFlag, "output", "o", "", "Output file path")
	dbPushCmd.Flags().StringVarP(&dbInputFlag, "input", "i", "", "Input backup file")
	dbPushCmd.Flags().StringVar(&dbPasswordFlag, "password", "", "Target database password (or use env var)")

	dbCmd.AddCommand(dbDumpCmd)
	dbCmd.AddCommand(dbPushCmd)
	dbCmd.AddCommand(dbListCmd)
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
	_ = config.LoadOrDefault() // Load config for consistency

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
	connInfo, err := client.GetBranchConnectionInfo(gitBranch, projectRef)
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not get connection info via API: %v", err))
		ui.Info("Falling back to manual password entry")
	}

	// Get password - prefer from env vars, then API, then prompt
	password := getDbPassword(env)
	if password == "" && connInfo != nil && connInfo.PostgresURL != "" {
		// Extract password from POSTGRES_URL if available (preview branches only)
		password = supabase.ExtractPasswordFromURL(connInfo.PostgresURL)
	}
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
		cfg := config.LoadOrDefault()
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

	if dbOutputFlag != "" {
		opts.OutputFile = dbOutputFlag
	} else {
		prefix := "prod"
		if !isProd {
			prefix = "dev"
		}
		opts.OutputFile = fmt.Sprintf("%s.backup", prefix)
	}

	ui.NewLine()

	// Perform dump
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

	return nil
}

func runDbPush(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	target := args[0]

	// Validate target
	var sourceFile string
	var targetEnv string
	var targetBranch *supabase.Branch
	var targetGitBranch string

	client := supabase.NewClient()

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
		return fmt.Errorf("invalid target: %s (use dev or feature)", target)
	}

	targetProjectRef := targetBranch.ProjectRef

	// Override source file if specified
	if dbInputFlag != "" {
		sourceFile = dbInputFlag
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
					dumpEnv := "prod"
					if target == "feature" {
						dumpEnv = "dev"
					}
					if err := runDbDump(cmd, []string{dumpEnv}); err != nil {
						return err
					}
				}
			}
		}
	}

	ui.Header(fmt.Sprintf("Database Push - %s", targetEnv))

	// Get connection info using experimental API (includes correct pooler host)
	connInfo, err := client.GetBranchConnectionInfo(targetGitBranch, targetProjectRef)
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not get connection info via API: %v", err))
	}

	// Get password - prefer from env vars, then API, then prompt
	password := getDbPassword(target)
	if password == "" && target == "feature" {
		password = getDbPassword("dev") // Feature branches use dev password
	}
	if password == "" && connInfo != nil && connInfo.PostgresURL != "" {
		// Extract password from POSTGRES_URL if available (preview branches only)
		password = supabase.ExtractPasswordFromURL(connInfo.PostgresURL)
	}
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
		cfg := config.LoadOrDefault()
		poolerHost = cfg.Database.PoolerHost
	}

	// Use session mode (port 5432) for pg_restore
	poolerPort := 5432
	poolerUser := fmt.Sprintf("postgres.%s", targetProjectRef)

	ui.KeyValue("Source", sourceFile)
	ui.KeyValue("Target", envColorString(targetEnv))
	ui.KeyValue("Project Ref", ui.Cyan(targetProjectRef))
	ui.KeyValue("Pooler", fmt.Sprintf("%s:%d", poolerHost, poolerPort))

	// Confirm
	if !IsYes() {
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

	return nil
}

func runDbList(cmd *cobra.Command, args []string) error {
	ui.Header("Local Database Backups")

	pattern := "*.backup"
	if len(args) > 0 {
		pattern = args[0] + ".backup"
	}

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	if len(matches) == 0 {
		ui.Info("No backup files found")
		return nil
	}

	for _, file := range matches {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		sizeMB := float64(info.Size()) / 1024 / 1024
		age := time.Since(info.ModTime())
		ageStr := fmt.Sprintf("%.0f hours ago", age.Hours())
		if age < time.Hour {
			ageStr = fmt.Sprintf("%.0f minutes ago", age.Minutes())
		} else if age > 24*time.Hour {
			ageStr = fmt.Sprintf("%.1f days ago", age.Hours()/24)
		}

		fmt.Printf("  %s  %s  %s\n",
			ui.Cyan(file),
			ui.Dim(fmt.Sprintf("%.2f MB", sizeMB)),
			ui.Dim(ageStr),
		)
	}

	return nil
}

