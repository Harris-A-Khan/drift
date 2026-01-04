package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/internal/git"
	"github.com/undrift/drift/internal/supabase"
	"github.com/undrift/drift/internal/ui"
	"github.com/undrift/drift/internal/xcode"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Environment management",
	Long: `Manage Supabase environment configuration and xcconfig generation.

The env command helps you manage which Supabase environment your Xcode 
project is configured to use. It auto-detects your git branch and maps 
it to the appropriate Supabase branch (production, development, or feature).`,
}

var envShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current environment info",
	Long:  `Display the current environment configuration, including git branch, Supabase branch, and project details.`,
	RunE:  runEnvShow,
}

var envSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Generate Config.xcconfig for current branch",
	Long: `Generate Config.xcconfig with Supabase credentials for the current git branch.

This command:
1. Detects your current git branch
2. Finds the matching Supabase branch (or falls back to development)
3. Fetches the API keys for that branch
4. Generates Config.xcconfig with the credentials`,
	RunE: runEnvSetup,
}

var envSwitchCmd = &cobra.Command{
	Use:   "switch <branch>",
	Short: "Setup xcconfig for a different Supabase branch",
	Long:  `Generate Config.xcconfig for a specific Supabase branch, regardless of the current git branch.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runEnvSwitch,
}

var (
	envBranchFlag string
)

func init() {
	envSetupCmd.Flags().StringVarP(&envBranchFlag, "branch", "b", "", "Override Supabase branch selection")

	envCmd.AddCommand(envShowCmd)
	envCmd.AddCommand(envSetupCmd)
	envCmd.AddCommand(envSwitchCmd)
	rootCmd.AddCommand(envCmd)
}

func runEnvShow(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()

	// Get current git branch
	gitBranch, err := git.CurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current git branch: %w", err)
	}

	ui.Header("Environment Info")

	// Get Supabase branch info
	client := supabase.NewClient()
	info, err := client.GetBranchInfo(gitBranch)
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not resolve Supabase branch: %v", err))
		ui.KeyValue("Git Branch", ui.Cyan(gitBranch))
		return nil
	}

	// Display info
	ui.KeyValue("Git Branch", ui.Cyan(gitBranch))
	ui.KeyValue("Environment", envColorString(string(info.Environment)))
	ui.KeyValue("Supabase Branch", ui.Cyan(info.SupabaseBranch.Name))
	ui.KeyValue("Project Ref", ui.Cyan(info.ProjectRef))
	ui.KeyValue("API URL", info.APIURL)

	if info.IsFallback {
		ui.NewLine()
		ui.Warning("Using fallback: no Supabase branch exists for this git branch")
	}

	// Check xcconfig status
	ui.NewLine()
	ui.SubHeader("Xcconfig Status")

	xcconfigPath := cfg.GetXcconfigPath()
	if xcode.XcconfigExists(xcconfigPath) {
		currentEnv, err := xcode.GetCurrentEnvironment(xcconfigPath)
		if err == nil {
			ui.KeyValue("Config File", xcconfigPath)
			ui.KeyValue("Configured Env", envColorString(currentEnv))

			if currentEnv != string(info.Environment) {
				ui.NewLine()
				ui.Warning("Xcconfig environment doesn't match current branch!")
				ui.Infof("Run 'drift env setup' to update")
			}
		}
	} else {
		ui.Warning("Config.xcconfig not found")
		ui.Infof("Run 'drift env setup' to generate it")
	}

	return nil
}

func runEnvSetup(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()

	// Get current git branch
	gitBranch, err := git.CurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current git branch: %w", err)
	}

	targetBranch := gitBranch
	if envBranchFlag != "" {
		targetBranch = envBranchFlag
		ui.Infof("Using branch override: %s", envBranchFlag)
	}

	// Resolve Supabase branch
	client := supabase.NewClient()

	sp := ui.NewSpinner("Resolving Supabase branch")
	sp.Start()

	info, err := client.GetBranchInfo(targetBranch)
	if err != nil {
		sp.Fail("Failed to resolve Supabase branch")
		return err
	}
	sp.Stop()

	if info.IsFallback {
		ui.Warningf("No Supabase branch for '%s', using fallback to development", targetBranch)
	}

	// Fetch anon key
	sp = ui.NewSpinner("Fetching API keys")
	sp.Start()

	anonKey, err := client.GetAnonKey(info.ProjectRef)
	if err != nil {
		sp.Fail("Failed to fetch API keys")
		return fmt.Errorf("failed to get anon key: %w", err)
	}
	sp.Stop()

	// Generate xcconfig
	sp = ui.NewSpinner("Generating Config.xcconfig")
	sp.Start()

	xcconfigPath := cfg.GetXcconfigPath()
	generator := xcode.NewXcconfigGenerator(xcconfigPath)

	if err := generator.GenerateFromBranchInfo(info, anonKey); err != nil {
		sp.Fail("Failed to generate xcconfig")
		return err
	}

	sp.Success("Config.xcconfig generated")

	// Display summary
	ui.NewLine()
	ui.KeyValue("Environment", envColorString(string(info.Environment)))
	ui.KeyValue("Supabase Branch", ui.Cyan(info.SupabaseBranch.Name))
	ui.KeyValue("Project Ref", ui.Cyan(info.ProjectRef))
	ui.KeyValue("Output", xcconfigPath)

	return nil
}

func runEnvSwitch(cmd *cobra.Command, args []string) error {
	targetBranch := args[0]
	envBranchFlag = targetBranch
	return runEnvSetup(cmd, args)
}

// envColorString returns the colored environment string.
func envColorString(env string) string {
	switch env {
	case "Production":
		return ui.Red(env)
	case "Development":
		return ui.Yellow(env)
	default:
		return ui.Green(env)
	}
}

