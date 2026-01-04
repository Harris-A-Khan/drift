package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/internal/git"
	"github.com/undrift/drift/internal/supabase"
	"github.com/undrift/drift/internal/ui"
	"github.com/undrift/drift/internal/xcode"
	"github.com/undrift/drift/pkg/shell"
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
	envBranchFlag      string
	envBuildServerFlag bool
)

func init() {
	envSetupCmd.Flags().StringVarP(&envBranchFlag, "branch", "b", "", "Override Supabase branch selection")
	envSetupCmd.Flags().BoolVar(&envBuildServerFlag, "build-server", false, "Also generate buildServer.json for sourcekit-lsp")

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

	// Get Supabase branch info (with override support)
	client := supabase.NewClient()
	overrideBranch := cfg.Supabase.OverrideBranch
	info, err := client.GetBranchInfoWithOverride(gitBranch, overrideBranch)
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

	if info.IsOverride {
		ui.NewLine()
		ui.Infof("Override: using %s instead of %s", ui.Cyan(info.SupabaseBranch.Name), ui.Cyan(info.OverrideFrom))
	}

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

	// Apply override from config (if no flag override)
	overrideBranch := ""
	if envBranchFlag == "" && cfg.Supabase.OverrideBranch != "" {
		overrideBranch = cfg.Supabase.OverrideBranch
	}

	// Resolve Supabase branch
	client := supabase.NewClient()

	sp := ui.NewSpinner("Resolving Supabase branch")
	sp.Start()

	info, err := client.GetBranchInfoWithOverride(targetBranch, overrideBranch)
	if err != nil {
		sp.Fail("Failed to resolve Supabase branch")
		return err
	}
	sp.Stop()

	if info.IsOverride {
		ui.Infof("Override: using %s instead of %s", ui.Cyan(info.SupabaseBranch.Name), ui.Cyan(info.OverrideFrom))
	}

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

	// Generate buildServer.json if requested
	if envBuildServerFlag {
		if err := generateBuildServer(cfg, info); err != nil {
			ui.Warning(fmt.Sprintf("Could not generate buildServer.json: %v", err))
		}
	}

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

// generateBuildServer generates buildServer.json for sourcekit-lsp support.
// This enables better IDE integration for Swift projects in VS Code and other editors.
func generateBuildServer(cfg *config.Config, info *supabase.BranchInfo) error {
	// Check if xcode-build-server is installed
	if !shell.CommandExists("xcode-build-server") {
		ui.Warning("xcode-build-server not found")
		ui.Info("Install with: brew install xcode-build-server")
		return nil
	}

	// Find the Xcode project file
	projectFile := ""
	matches, _ := filepath.Glob("*.xcodeproj")
	if len(matches) > 0 {
		projectFile = matches[0]
	} else {
		// Try workspace
		matches, _ = filepath.Glob("*.xcworkspace")
		if len(matches) > 0 {
			// Use workspace with -workspace flag instead
			return generateBuildServerWithWorkspace(cfg, info, matches[0])
		}
		return fmt.Errorf("no .xcodeproj or .xcworkspace found")
	}

	// Determine scheme based on environment
	scheme := getSchemeForEnvironment(cfg, info)
	if scheme == "" {
		return fmt.Errorf("could not determine Xcode scheme")
	}

	sp := ui.NewSpinner("Generating buildServer.json")
	sp.Start()

	// Run xcode-build-server config
	result, err := shell.Run("xcode-build-server", "config", "-project", projectFile, "-scheme", scheme)
	if err != nil {
		sp.Fail("Failed to generate buildServer.json")
		if result != nil && result.Stderr != "" {
			return fmt.Errorf("%s", result.Stderr)
		}
		return err
	}

	sp.Success(fmt.Sprintf("buildServer.json generated for scheme: %s", scheme))
	return nil
}

// generateBuildServerWithWorkspace generates buildServer.json using a workspace.
func generateBuildServerWithWorkspace(cfg *config.Config, info *supabase.BranchInfo, workspace string) error {
	scheme := getSchemeForEnvironment(cfg, info)
	if scheme == "" {
		return fmt.Errorf("could not determine Xcode scheme")
	}

	sp := ui.NewSpinner("Generating buildServer.json")
	sp.Start()

	result, err := shell.Run("xcode-build-server", "config", "-workspace", workspace, "-scheme", scheme)
	if err != nil {
		sp.Fail("Failed to generate buildServer.json")
		if result != nil && result.Stderr != "" {
			return fmt.Errorf("%s", result.Stderr)
		}
		return err
	}

	sp.Success(fmt.Sprintf("buildServer.json generated for scheme: %s", scheme))
	return nil
}

// getSchemeForEnvironment determines the Xcode scheme based on environment.
func getSchemeForEnvironment(cfg *config.Config, info *supabase.BranchInfo) string {
	// First check config for explicit scheme mappings
	if cfg.Xcode.Schemes != nil {
		switch info.Environment {
		case supabase.EnvProduction:
			if scheme, ok := cfg.Xcode.Schemes["production"]; ok {
				return scheme
			}
		case supabase.EnvDevelopment:
			if scheme, ok := cfg.Xcode.Schemes["development"]; ok {
				return scheme
			}
		default:
			if scheme, ok := cfg.Xcode.Schemes["feature"]; ok {
				return scheme
			}
		}
	}

	// Try to find schemes automatically
	projectName := cfg.Project.Name
	if projectName == "" {
		// Try to get from xcodeproj name
		matches, _ := filepath.Glob("*.xcodeproj")
		if len(matches) > 0 {
			projectName = matches[0][:len(matches[0])-len(".xcodeproj")]
		}
	}

	// Try common naming patterns
	patterns := []string{
		fmt.Sprintf("%s (%s)", projectName, info.Environment),
		fmt.Sprintf("%s-%s", projectName, info.Environment),
		projectName,
	}

	// Check if any scheme exists
	for _, pattern := range patterns {
		if schemeExists(pattern) {
			return pattern
		}
	}

	// Return the project name as a fallback
	return projectName
}

// schemeExists checks if a scheme exists in the project.
func schemeExists(scheme string) bool {
	// Check in xcuserdata or xcshareddata
	patterns := []string{
		fmt.Sprintf("*.xcodeproj/xcshareddata/xcschemes/%s.xcscheme", scheme),
		fmt.Sprintf("*.xcodeproj/xcuserdata/*/xcschemes/%s.xcscheme", scheme),
	}

	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		if len(matches) > 0 {
			return true
		}
	}

	// Also check if we can list schemes
	result, err := shell.Run("xcodebuild", "-list", "-json")
	if err != nil {
		return false
	}

	// Simple string check (not full JSON parsing for simplicity)
	return len(result.Stdout) > 0 && (filepath.Base(scheme) != "" || os.Getenv("DRIFT_DEBUG") != "")
}

