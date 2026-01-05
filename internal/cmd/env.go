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

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Environment management",
	Long: `Manage Supabase environment configuration.

The env command helps you manage which Supabase environment your project
is configured to use. It auto-detects your git branch and maps it to the
appropriate Supabase branch (production, development, or feature).

For web projects, it generates .env.local with all Supabase credentials.
For iOS/macOS projects, it generates Config.xcconfig.`,
}

var envShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current environment info",
	Long:  `Display the current environment configuration, including git branch, Supabase branch, and project details.`,
	RunE:  runEnvShow,
}

var envSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Generate environment config for current branch",
	Long: `Generate environment configuration with Supabase credentials for the current git branch.

This command:
1. Detects your current git branch
2. Finds the matching Supabase branch (or falls back to development)
3. Fetches the API keys for that branch
4. Generates the appropriate config file:
   - .env.local for web projects
   - Config.xcconfig for iOS/macOS projects

For web projects, you can copy custom variables from another .env.local file:
  drift env setup --copy-custom-from /path/to/other/.env.local`,
	RunE: runEnvSetup,
}

var envSwitchCmd = &cobra.Command{
	Use:   "switch <branch>",
	Short: "Setup environment for a different Supabase branch",
	Long:  `Generate environment config for a specific Supabase branch, regardless of the current git branch.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runEnvSwitch,
}

var (
	envBranchFlag        string
	envBuildServerFlag   bool
	envCopyCustomFromFlag string
)

func init() {
	envSetupCmd.Flags().StringVarP(&envBranchFlag, "branch", "b", "", "Override Supabase branch selection")
	envSetupCmd.Flags().BoolVar(&envBuildServerFlag, "build-server", false, "Also generate buildServer.json for sourcekit-lsp")
	envSetupCmd.Flags().StringVar(&envCopyCustomFromFlag, "copy-custom-from", "", "Copy custom variables from another .env.local file")

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

	// Check config file status based on project type
	ui.NewLine()

	if cfg.Project.IsWebPlatform() {
		ui.SubHeader("Environment File Status")
		envLocalPath := cfg.GetEnvLocalPath()
		if web.EnvLocalExists(envLocalPath) {
			ui.KeyValue("Config File", envLocalPath)

			currentEnv, err := web.GetCurrentEnvironment(envLocalPath)
			if err != nil {
				ui.Warning(fmt.Sprintf("Could not read environment: %v", err))
				ui.Infof("Run 'drift env setup' to regenerate")
			} else {
				ui.KeyValue("Configured Env", envColorString(currentEnv))

				if currentEnv != string(info.Environment) {
					ui.NewLine()
					ui.Warning("Environment file doesn't match current branch!")
					ui.Infof("Run 'drift env setup' to update")
				}
			}
		} else {
			ui.Warning(".env.local not found")
			ui.Infof("Run 'drift env setup' to generate it")
		}
	} else {
		ui.SubHeader("Xcconfig Status")
		xcconfigPath := cfg.GetXcconfigPath()
		if xcode.XcconfigExists(xcconfigPath) {
			ui.KeyValue("Config File", xcconfigPath)

			currentEnv, err := xcode.GetCurrentEnvironment(xcconfigPath)
			if err != nil {
				ui.Warning(fmt.Sprintf("Could not read environment: %v", err))
				ui.Infof("Run 'drift env setup' to regenerate")
			} else {
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

	// Ensure Supabase is linked (uses project_ref from config)
	if err := ensureSupabaseLinked(cfg); err != nil {
		return err
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

	// Fetch API keys and secrets
	sp = ui.NewSpinner("Fetching API keys")
	sp.Start()

	var anonKey, serviceRoleKey string
	var webSecrets *web.BranchSecretsInput

	// For non-production branches, we can get all secrets via branches get
	if info.Environment != supabase.EnvProduction {
		secrets, err := client.GetBranchSecrets(info.SupabaseBranch.Name)
		if err == nil {
			anonKey = secrets.SupabaseAnonKey
			serviceRoleKey = secrets.SupabaseServiceRoleKey
			if cfg.Project.IsWebPlatform() {
				webSecrets = &web.BranchSecretsInput{
					AnonKey:           secrets.SupabaseAnonKey,
					ServiceRoleKey:    secrets.SupabaseServiceRoleKey,
					DatabasePassword:  supabase.ExtractPasswordFromURL(secrets.PostgresURLNonPooling),
					DirectDatabaseURL: secrets.PostgresURLNonPooling,
					PoolerDatabaseURL: secrets.PostgresURL,
				}
			}
		} else {
			// Fallback to API keys method
			anonKey, err = client.GetAnonKey(info.ProjectRef)
			if err != nil {
				sp.Fail("Failed to fetch API keys")
				return fmt.Errorf("failed to get anon key: %w", err)
			}
			if cfg.Project.IsWebPlatform() {
				serviceRoleKey, _ = client.GetServiceKey(info.ProjectRef)
				webSecrets = &web.BranchSecretsInput{
					AnonKey:        anonKey,
					ServiceRoleKey: serviceRoleKey,
				}
			}
		}
	} else {
		// Production - use API keys method
		anonKey, err = client.GetAnonKey(info.ProjectRef)
		if err != nil {
			sp.Fail("Failed to fetch API keys")
			return fmt.Errorf("failed to get anon key: %w", err)
		}
		if cfg.Project.IsWebPlatform() {
			serviceRoleKey, _ = client.GetServiceKey(info.ProjectRef)
			webSecrets = &web.BranchSecretsInput{
				AnonKey:        anonKey,
				ServiceRoleKey: serviceRoleKey,
			}
		}
	}
	sp.Stop()

	// Generate config file based on project type
	var outputPath string

	if cfg.Project.IsWebPlatform() {
		sp = ui.NewSpinner("Generating .env.local")
		sp.Start()

		outputPath = cfg.GetEnvLocalPath()
		generator := web.NewEnvLocalGenerator(outputPath)

		if err := generator.GenerateFromBranchInfo(info, webSecrets); err != nil {
			sp.Fail("Failed to generate .env.local")
			return err
		}

		sp.Success(".env.local generated")

		// Copy custom variables from another file if requested
		if envCopyCustomFromFlag != "" {
			if err := copyCustomVariables(envCopyCustomFromFlag, outputPath); err != nil {
				ui.Warning(fmt.Sprintf("Could not copy custom variables: %v", err))
			}
		}
	} else {
		sp = ui.NewSpinner("Generating Config.xcconfig")
		sp.Start()

		outputPath = cfg.GetXcconfigPath()
		generator := xcode.NewXcconfigGenerator(outputPath)

		if err := generator.GenerateFromBranchInfo(info, anonKey); err != nil {
			sp.Fail("Failed to generate xcconfig")
			return err
		}

		sp.Success("Config.xcconfig generated")

		// Generate buildServer.json if requested (only for Apple platforms)
		if envBuildServerFlag {
			if err := generateBuildServer(cfg, info); err != nil {
				ui.Warning(fmt.Sprintf("Could not generate buildServer.json: %v", err))
			}
		}
	}

	// Display summary
	ui.NewLine()
	ui.KeyValue("Environment", envColorString(string(info.Environment)))
	ui.KeyValue("Supabase Branch", ui.Cyan(info.SupabaseBranch.Name))
	ui.KeyValue("Project Ref", ui.Cyan(info.ProjectRef))
	ui.KeyValue("Output", outputPath)

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

// ensureSupabaseLinked checks if Supabase is linked in the current directory,
// and links it using the project_ref from config if not.
func ensureSupabaseLinked(cfg *config.Config) error {
	// Check if already linked by trying to list branches
	result, err := shell.Run("supabase", "branches", "list", "--output", "json")
	if err == nil && result.ExitCode == 0 {
		return nil // Already linked
	}

	// Check if error is about not being linked
	errMsg := ""
	if result != nil {
		errMsg = result.Stderr + result.Stdout
	}
	if !strings.Contains(errMsg, "Have you run supabase link") {
		// Some other error
		return nil
	}

	// Not linked - try to link using project_ref from config
	projectRef := cfg.Supabase.ProjectRef
	if projectRef == "" {
		return fmt.Errorf("Supabase not linked and no project_ref in config. Run 'supabase link' first")
	}

	sp := ui.NewSpinner("Linking Supabase project")
	sp.Start()

	result, err = shell.Run("supabase", "link", "--project-ref", projectRef)
	if err != nil {
		sp.Fail("Failed to link Supabase")
		errMsg := ""
		if result != nil {
			errMsg = result.Stderr
		}
		return fmt.Errorf("failed to link Supabase: %s", errMsg)
	}

	sp.Success("Supabase linked")
	return nil
}

// copyCustomVariables copies custom (non-drift-managed) variables from a source
// .env.local file to a destination file.
func copyCustomVariables(sourcePath, destPath string) error {
	// Read the source file
	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("could not read source file: %w", err)
	}

	// Extract custom variables from source
	customContent := web.ExtractUserContent(string(sourceData))
	if customContent == "" {
		ui.Info("No custom variables found in source file")
		return nil
	}

	// Read the destination file
	destData, err := os.ReadFile(destPath)
	if err != nil {
		return fmt.Errorf("could not read destination file: %w", err)
	}

	// Check if destination already has the DRIFT MANAGED END marker
	destContent := string(destData)
	endMarker := web.DriftSectionEnd

	// Find the end marker and append custom content after it
	if idx := strings.Index(destContent, endMarker); idx != -1 {
		// Find the end of the header section after the marker
		afterMarker := destContent[idx+len(endMarker):]

		// Find where the "CUSTOM VARIABLES" header ends
		headerEnd := strings.Index(afterMarker, "# =============================================================================")
		if headerEnd != -1 {
			// Skip past the header line
			nextNewline := strings.Index(afterMarker[headerEnd:], "\n")
			if nextNewline != -1 {
				headerEnd += nextNewline + 1
			}
		}

		// Preserve the header and replace everything after with the new custom content
		beforeCustom := destContent[:idx+len(endMarker)] + afterMarker[:headerEnd]
		newContent := beforeCustom + customContent

		if err := os.WriteFile(destPath, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("could not write destination file: %w", err)
		}

		ui.Success(fmt.Sprintf("Copied custom variables from %s", sourcePath))
	}

	return nil
}

