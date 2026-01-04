package cmd

import (
	"fmt"

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

	// Resolve Supabase branch
	client := supabase.NewClient()

	sp := ui.NewSpinner("Resolving Supabase branch")
	sp.Start()

	info, err := client.GetBranchInfo(targetBranch)
	if err != nil {
		sp.Fail("Failed to resolve branch")
		return err
	}
	sp.Stop()

	ui.KeyValue("Git Branch", ui.Cyan(targetBranch))
	ui.KeyValue("Environment", envColorString(string(info.Environment)))
	ui.KeyValue("Supabase Branch", ui.Cyan(info.SupabaseBranch.Name))
	ui.KeyValue("Project Ref", ui.Cyan(info.ProjectRef))

	if info.IsFallback {
		ui.Warning("Using fallback branch")
	}

	// Confirm for production
	if info.Environment == supabase.EnvProduction && !IsYes() {
		ui.NewLine()
		ui.Warning("You are about to push migrations to PRODUCTION!")
		confirmed, err := ui.PromptYesNo("Are you absolutely sure?", false)
		if err != nil || !confirmed {
			ui.Info("Cancelled")
			return nil
		}
	}

	ui.NewLine()

	if migrateDryRunFlag {
		ui.Info("Dry run - would push migrations to:")
		ui.KeyValue("Branch", info.SupabaseBranch.Name)
		ui.KeyValue("Project Ref", info.ProjectRef)
		return nil
	}

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
	ui.Success("Migrations pushed successfully")

	return nil
}

func runMigrateStatus(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	ui.Header("Migration Status")

	// Get current git branch
	gitBranch, err := git.CurrentBranch()
	if err != nil {
		return err
	}

	client := supabase.NewClient()
	info, err := client.GetBranchInfo(gitBranch)
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not resolve Supabase branch: %v", err))
	} else {
		ui.KeyValue("Git Branch", ui.Cyan(gitBranch))
		ui.KeyValue("Environment", envColorString(string(info.Environment)))
		ui.KeyValue("Supabase Branch", ui.Cyan(info.SupabaseBranch.Name))
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

