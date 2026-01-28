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

var envValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate environment configuration",
	Long: `Perform comprehensive validation of your environment configuration.

Validation checks:
1. Config file exists and is valid YAML
2. Required Supabase credentials are set (SUPABASE_URL, SUPABASE_ANON_KEY)
3. Drift markers are intact (=== DRIFT MANAGED ===)
4. Configured Xcode schemes exist (if applicable)
5. DB_SCHEMA_VERSION matches latest migration (optional)`,
	RunE: runEnvValidate,
}

var envDiffCmd = &cobra.Command{
	Use:   "diff <branch1> <branch2>",
	Short: "Compare environments between branches",
	Long: `Compare the environment configuration between two branches.

Shows differences in:
- Supabase URL and Project Ref
- API keys (masked)
- Custom variables

Examples:
  drift env diff main dev
  drift env diff main feat/new-feature`,
	Args: cobra.ExactArgs(2),
	RunE: runEnvDiff,
}

var (
	envBranchFlag         string
	envBuildServerFlag    bool
	envCopyCustomFromFlag string
	envCopyEnvFlag        bool
	envSchemeFlag         string
	envCIFlag             bool
)

func init() {
	envSetupCmd.Flags().StringVarP(&envBranchFlag, "branch", "b", "", "Override Supabase branch selection")
	envSetupCmd.Flags().BoolVar(&envBuildServerFlag, "build-server", false, "Also generate buildServer.json for sourcekit-lsp")
	envSetupCmd.Flags().StringVar(&envCopyCustomFromFlag, "copy-custom-from", "", "Copy custom variables from a specific .env.local file path")
	envSetupCmd.Flags().BoolVar(&envCopyEnvFlag, "copy-env", false, "Copy custom variables from another worktree (interactive picker)")
	envSetupCmd.Flags().StringVar(&envSchemeFlag, "scheme", "", "Xcode scheme to use for buildServer.json (requires --build-server)")
	envSetupCmd.Flags().BoolVar(&envCIFlag, "ci", false, "CI mode: read SUPABASE_URL and SUPABASE_ANON_KEY from environment variables")

	envCmd.AddCommand(envShowCmd)
	envCmd.AddCommand(envSetupCmd)
	envCmd.AddCommand(envSwitchCmd)
	envCmd.AddCommand(envValidateCmd)
	envCmd.AddCommand(envDiffCmd)
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

	// CI mode: read from environment variables
	if envCIFlag {
		return runEnvSetupCI(cfg)
	}

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

		// Copy custom variables from another worktree (interactive picker)
		if envCopyEnvFlag {
			sourcePath, err := selectWorktreeConfigFile(cfg, ".env.local")
			if err != nil {
				ui.Warning(fmt.Sprintf("Could not select worktree: %v", err))
			} else if sourcePath != "" {
				envCopyCustomFromFlag = sourcePath
			}
		}

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

		// Copy custom variables from another worktree (interactive picker)
		if envCopyEnvFlag {
			xcconfigName := filepath.Base(cfg.GetXcconfigPath())
			sourcePath, err := selectWorktreeConfigFile(cfg, xcconfigName)
			if err != nil {
				ui.Warning(fmt.Sprintf("Could not select worktree: %v", err))
			} else if sourcePath != "" {
				if err := copyXcconfigCustomVariables(sourcePath, outputPath); err != nil {
					ui.Warning(fmt.Sprintf("Could not copy custom variables: %v", err))
				}
			}
		}

		// Generate buildServer.json if requested (only for Apple platforms)
		if envBuildServerFlag {
			if err := generateBuildServer(cfg, info, envSchemeFlag); err != nil {
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

// runEnvSetupCI generates Config.xcconfig or .env.local from environment variables.
// This is used in CI pipelines where Supabase CLI is not available or configured.
func runEnvSetupCI(cfg *config.Config) error {
	ui.Info("CI mode: reading credentials from environment variables")

	// Read required environment variables
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseAnonKey := os.Getenv("SUPABASE_ANON_KEY")

	if supabaseURL == "" {
		return fmt.Errorf("SUPABASE_URL environment variable is not set")
	}
	if supabaseAnonKey == "" {
		return fmt.Errorf("SUPABASE_ANON_KEY environment variable is not set")
	}

	// Get git branch (optional in CI, may not have full git context)
	gitBranch := "ci"
	if branch, err := git.CurrentBranch(); err == nil {
		gitBranch = branch
	}

	// Generate config file based on project type
	var outputPath string

	if cfg.Project.IsWebPlatform() {
		outputPath = cfg.GetEnvLocalPath()
		generator := web.NewEnvLocalGenerator(outputPath)

		// Create minimal BranchInfo for CI
		info := &supabase.BranchInfo{
			GitBranch:   gitBranch,
			Environment: supabase.EnvFeature, // Default to feature for CI
			APIURL:      supabaseURL,
			ProjectRef:  extractProjectRef(supabaseURL),
			SupabaseBranch: &supabase.Branch{
				Name: "ci",
			},
		}

		webSecrets := &web.BranchSecretsInput{
			AnonKey: supabaseAnonKey,
		}

		if err := generator.GenerateFromBranchInfo(info, webSecrets); err != nil {
			return fmt.Errorf("failed to generate .env.local: %w", err)
		}

		ui.Success(".env.local generated from environment variables")
	} else {
		outputPath = cfg.GetXcconfigPath()
		generator := xcode.NewXcconfigGenerator(outputPath)

		// Create minimal BranchInfo for CI
		info := &supabase.BranchInfo{
			GitBranch:   gitBranch,
			Environment: supabase.EnvFeature, // Default to feature for CI
			APIURL:      supabaseURL,
			ProjectRef:  extractProjectRef(supabaseURL),
			SupabaseBranch: &supabase.Branch{
				Name: "ci",
			},
		}

		if err := generator.GenerateFromBranchInfo(info, supabaseAnonKey); err != nil {
			return fmt.Errorf("failed to generate Config.xcconfig: %w", err)
		}

		ui.Success("Config.xcconfig generated from environment variables")
	}

	// Display summary
	ui.NewLine()
	ui.KeyValue("Mode", ui.Cyan("CI"))
	ui.KeyValue("Supabase URL", ui.Cyan(supabaseURL))
	ui.KeyValue("Anon Key", ui.Cyan(maskValue(supabaseAnonKey)))
	ui.KeyValue("Output", outputPath)

	return nil
}

// extractProjectRef attempts to extract the project ref from a Supabase URL.
// Example: https://abcdefgh.supabase.co -> abcdefgh
func extractProjectRef(url string) string {
	// Remove protocol
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Extract subdomain
	parts := strings.Split(url, ".")
	if len(parts) > 0 {
		return parts[0]
	}
	return "unknown"
}

// generateBuildServer generates buildServer.json for sourcekit-lsp support.
// This enables better IDE integration for Swift projects in VS Code and other editors.
// If schemeOverride is provided, it will be used instead of auto-detection.
func generateBuildServer(cfg *config.Config, info *supabase.BranchInfo, schemeOverride string) error {
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
			return generateBuildServerWithWorkspace(cfg, info, matches[0], schemeOverride)
		}
		return fmt.Errorf("no .xcodeproj or .xcworkspace found")
	}

	// Determine scheme - use override if provided, otherwise auto-detect
	scheme := schemeOverride
	if scheme == "" {
		scheme = getSchemeForEnvironment(cfg, info)
	}
	if scheme == "" {
		return fmt.Errorf("could not determine Xcode scheme. Use --scheme to specify one")
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
// If schemeOverride is provided, it will be used instead of auto-detection.
func generateBuildServerWithWorkspace(cfg *config.Config, info *supabase.BranchInfo, workspace string, schemeOverride string) error {
	// Determine scheme - use override if provided, otherwise auto-detect
	scheme := schemeOverride
	if scheme == "" {
		scheme = getSchemeForEnvironment(cfg, info)
	}
	if scheme == "" {
		return fmt.Errorf("could not determine Xcode scheme. Use --scheme to specify one")
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
	// Check if Supabase CLI is available
	if !shell.CommandExists("supabase") {
		return fmt.Errorf("Supabase CLI not found. Install with: brew install supabase/tap/supabase")
	}

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
		// Some other error (auth, network, etc.) - report it instead of silently ignoring
		if strings.Contains(errMsg, "not logged in") || strings.Contains(errMsg, "Access token") {
			return fmt.Errorf("Supabase CLI not authenticated. Run 'supabase login' first")
		}
		// For other errors, warn but continue (might be a network blip)
		if errMsg != "" {
			ui.Warning(fmt.Sprintf("Could not check Supabase link status: %s", strings.TrimSpace(errMsg)))
		}
		return nil
	}

	// Not linked - check if we have project_ref in config
	projectRef := cfg.Supabase.ProjectRef
	if projectRef == "" {
		// No project_ref in config - ask user interactively
		ui.Warning("Supabase is not linked to this directory")
		ui.NewLine()

		// Prompt for project ref
		input, err := ui.PromptString("Enter Supabase project ref (or leave empty to skip)", "")
		if err != nil || input == "" {
			ui.Info("Skipping Supabase linking. Run 'supabase link' manually when ready.")
			return nil
		}
		projectRef = strings.TrimSpace(input)
	}

	// Ask user for confirmation before linking
	ui.Infof("Will link to Supabase project: %s", ui.Cyan(projectRef))
	proceed, err := ui.PromptYesNo("Proceed with linking?", true)
	if err != nil || !proceed {
		ui.Info("Skipping Supabase linking")
		return nil
	}

	sp := ui.NewSpinner("Linking Supabase project")
	sp.Start()

	result, err = shell.Run("supabase", "link", "--project-ref", projectRef)
	if err != nil || (result != nil && result.ExitCode != 0) {
		sp.Fail("Failed to link Supabase")
		errMsg := ""
		if result != nil {
			errMsg = strings.TrimSpace(result.Stderr + result.Stdout)
		}
		return fmt.Errorf("failed to link Supabase: %s", errMsg)
	}

	// Verify linking succeeded
	verifyResult, verifyErr := shell.Run("supabase", "branches", "list", "--output", "json")
	if verifyErr != nil || (verifyResult != nil && verifyResult.ExitCode != 0) {
		sp.Fail("Linking verification failed")
		return fmt.Errorf("Supabase linking appeared to succeed but verification failed. Try running 'supabase link' manually")
	}

	sp.Success("Supabase linked successfully")
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

// selectWorktreeConfigFile shows an interactive picker of worktrees that have the specified
// config file and returns the selected path.
func selectWorktreeConfigFile(cfg *config.Config, filename string) (string, error) {
	worktrees, err := git.ListWorktrees()
	if err != nil {
		return "", err
	}

	// Filter worktrees that have the config file (exclude current)
	var options []string
	var paths []string

	for _, wt := range worktrees {
		if wt.IsCurrent {
			continue
		}

		configPath := filepath.Join(wt.Path, filename)
		if _, err := os.Stat(configPath); err == nil {
			display := fmt.Sprintf("%s (%s)", wt.Branch, wt.Path)
			options = append(options, display)
			paths = append(paths, configPath)
		}
	}

	if len(options) == 0 {
		ui.Infof("No other worktrees with %s files found", filename)
		return "", nil
	}

	idx, _, err := ui.PromptSelectWithIndex("Copy custom variables from", options)
	if err != nil {
		return "", err
	}

	return paths[idx], nil
}

// copyXcconfigCustomVariables copies custom (non-drift-managed) variables from a source
// xcconfig file to a destination file.
func copyXcconfigCustomVariables(sourcePath, destPath string) error {
	// Read the source file
	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("could not read source file: %w", err)
	}

	// Extract custom variables from source (everything after drift-managed section)
	customContent := xcode.ExtractUserContent(string(sourceData))
	if customContent == "" {
		ui.Info("No custom variables found in source file")
		return nil
	}

	// Read the destination file
	destData, err := os.ReadFile(destPath)
	if err != nil {
		return fmt.Errorf("could not read destination file: %w", err)
	}

	// Append custom content to destination
	destContent := string(destData)
	newContent := strings.TrimRight(destContent, "\n") + "\n" + customContent

	if err := os.WriteFile(destPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("could not write destination file: %w", err)
	}

	ui.Success(fmt.Sprintf("Copied custom variables from %s", sourcePath))
	return nil
}

func runEnvValidate(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()

	ui.Header("Environment Validation")

	hasErrors := false
	validCount := 0
	totalChecks := 0

	// Check 1: Config file validity
	totalChecks++
	ui.SubHeader("Config File")
	configPath, _ := config.FindConfigFile()
	if configPath != "" {
		ui.Success(fmt.Sprintf("Config file: %s", configPath))
		validCount++
	} else {
		ui.Error("Config file not found")
		hasErrors = true
	}

	// Check 2: Environment config file exists
	totalChecks++
	ui.NewLine()
	ui.SubHeader("Environment File")

	var envFilePath string
	var envFileContent string
	if cfg.Project.IsWebPlatform() {
		envFilePath = cfg.GetEnvLocalPath()
	} else {
		envFilePath = cfg.GetXcconfigPath()
	}

	if _, err := os.Stat(envFilePath); err == nil {
		data, readErr := os.ReadFile(envFilePath)
		if readErr == nil {
			envFileContent = string(data)
			ui.Success(fmt.Sprintf("Environment file: %s", envFilePath))
			validCount++
		} else {
			ui.Error(fmt.Sprintf("Cannot read %s: %v", envFilePath, readErr))
			hasErrors = true
		}
	} else {
		ui.Warning(fmt.Sprintf("Environment file not found: %s", envFilePath))
		ui.Info("Run 'drift env setup' to generate it")
		hasErrors = true
	}

	// Check 3: Required variables present
	if envFileContent != "" {
		totalChecks++
		ui.NewLine()
		ui.SubHeader("Required Variables")

		if cfg.Project.IsWebPlatform() {
			// Check for NEXT_PUBLIC_SUPABASE_URL and NEXT_PUBLIC_SUPABASE_ANON_KEY
			requiredVars := []string{"NEXT_PUBLIC_SUPABASE_URL", "NEXT_PUBLIC_SUPABASE_ANON_KEY"}
			allPresent := true
			for _, v := range requiredVars {
				if strings.Contains(envFileContent, v+"=") {
					// Check if value is not empty
					for _, line := range strings.Split(envFileContent, "\n") {
						if strings.HasPrefix(line, v+"=") {
							value := strings.TrimPrefix(line, v+"=")
							value = strings.Trim(value, "\"' ")
							if value != "" {
								fmt.Printf("  %s %s\n", ui.Green("✓"), v)
							} else {
								fmt.Printf("  %s %s %s\n", ui.Red("✗"), v, ui.Red("(empty)"))
								allPresent = false
							}
							break
						}
					}
				} else {
					fmt.Printf("  %s %s %s\n", ui.Red("✗"), v, ui.Red("(missing)"))
					allPresent = false
				}
			}
			if allPresent {
				validCount++
			} else {
				hasErrors = true
			}
		} else {
			// Check for SUPABASE_URL and SUPABASE_ANON_KEY in xcconfig
			requiredVars := []string{"SUPABASE_URL", "SUPABASE_ANON_KEY"}
			allPresent := true
			for _, v := range requiredVars {
				if strings.Contains(envFileContent, v+" = ") || strings.Contains(envFileContent, v+"=") {
					fmt.Printf("  %s %s\n", ui.Green("✓"), v)
				} else {
					fmt.Printf("  %s %s %s\n", ui.Red("✗"), v, ui.Red("(missing)"))
					allPresent = false
				}
			}
			if allPresent {
				validCount++
			} else {
				hasErrors = true
			}
		}
	}

	// Check 4: Drift markers intact
	if envFileContent != "" {
		totalChecks++
		ui.NewLine()
		ui.SubHeader("Drift Markers")

		hasStartMarker := strings.Contains(envFileContent, "DRIFT MANAGED")
		if hasStartMarker {
			ui.Success("Drift markers are intact")
			validCount++
		} else {
			ui.Warning("Drift markers not found - file may have been manually edited")
			ui.Info("Run 'drift env setup' to regenerate with markers")
		}
	}

	// Check 5: Xcode schemes (for Apple platforms)
	if !cfg.Project.IsWebPlatform() && cfg.Xcode.Schemes != nil && len(cfg.Xcode.Schemes) > 0 {
		totalChecks++
		ui.NewLine()
		ui.SubHeader("Xcode Schemes")

		allValid := true
		for env, scheme := range cfg.Xcode.Schemes {
			if scheme == "" {
				continue
			}
			if xcode.SchemeExists(scheme) {
				fmt.Printf("  %s %s: %s\n", ui.Green("✓"), env, scheme)
			} else {
				fmt.Printf("  %s %s: %s %s\n", ui.Red("✗"), env, scheme, ui.Red("(not found)"))
				allValid = false
			}
		}
		if allValid {
			validCount++
		} else {
			hasErrors = true
		}
	}

	// Summary
	ui.NewLine()
	if hasErrors {
		ui.Warningf("Validation complete: %d/%d checks passed", validCount, totalChecks)
		return fmt.Errorf("validation failed")
	}

	ui.Successf("All %d validation checks passed", totalChecks)
	return nil
}

func runEnvDiff(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()

	branch1 := args[0]
	branch2 := args[1]

	ui.Header("Environment Diff")
	ui.Infof("Comparing: %s vs %s", ui.Cyan(branch1), ui.Cyan(branch2))
	ui.NewLine()

	// Find worktrees for each branch
	wt1, err := git.GetWorktree(branch1)
	if err != nil {
		return fmt.Errorf("branch '%s' not found as worktree: %w", branch1, err)
	}

	wt2, err := git.GetWorktree(branch2)
	if err != nil {
		return fmt.Errorf("branch '%s' not found as worktree: %w", branch2, err)
	}

	// Determine the config file name
	var configFileName string
	if cfg.Project.IsWebPlatform() {
		configFileName = filepath.Base(cfg.GetEnvLocalPath())
	} else {
		configFileName = filepath.Base(cfg.GetXcconfigPath())
	}

	// Read config files from each worktree
	path1 := filepath.Join(wt1.Path, configFileName)
	path2 := filepath.Join(wt2.Path, configFileName)

	data1, err1 := os.ReadFile(path1)
	data2, err2 := os.ReadFile(path2)

	if err1 != nil && err2 != nil {
		return fmt.Errorf("neither worktree has %s", configFileName)
	}

	// Parse variables from each file
	vars1 := parseEnvVariables(string(data1))
	vars2 := parseEnvVariables(string(data2))

	// Compare and display differences
	ui.SubHeader("Supabase Configuration")

	// List of variables to compare (with masking for sensitive ones)
	compareVars := []struct {
		name   string
		masked bool
	}{
		{"SUPABASE_URL", false},
		{"NEXT_PUBLIC_SUPABASE_URL", false},
		{"SUPABASE_ANON_KEY", true},
		{"NEXT_PUBLIC_SUPABASE_ANON_KEY", true},
		{"SUPABASE_PROJECT_REF", false},
		{"DRIFT_ENVIRONMENT", false},
		{"DRIFT_SUPABASE_BRANCH", false},
	}

	hasDiff := false
	for _, v := range compareVars {
		val1 := vars1[v.name]
		val2 := vars2[v.name]

		if val1 == "" && val2 == "" {
			continue
		}

		if v.masked {
			val1 = maskValue(val1)
			val2 = maskValue(val2)
		}

		if val1 == val2 {
			fmt.Printf("  %s = %s (same)\n", v.name, ui.Dim(truncateValue(val1, 40)))
		} else {
			hasDiff = true
			fmt.Printf("  %s:\n", ui.Yellow(v.name))
			fmt.Printf("    %s: %s\n", ui.Cyan(branch1), truncateValue(val1, 50))
			fmt.Printf("    %s: %s\n", ui.Cyan(branch2), truncateValue(val2, 50))
		}
	}

	// Show custom variables diff
	ui.NewLine()
	ui.SubHeader("Custom Variables")

	// Find all unique variable names
	allVars := make(map[string]bool)
	for k := range vars1 {
		allVars[k] = true
	}
	for k := range vars2 {
		allVars[k] = true
	}

	// Filter out the compared ones
	comparedNames := make(map[string]bool)
	for _, v := range compareVars {
		comparedNames[v.name] = true
	}

	customDiff := false
	for name := range allVars {
		if comparedNames[name] {
			continue
		}
		// Skip comments and empty lines
		if strings.HasPrefix(name, "#") || name == "" {
			continue
		}

		val1 := vars1[name]
		val2 := vars2[name]

		if val1 == "" && val2 != "" {
			customDiff = true
			fmt.Printf("  %s: only in %s\n", ui.Yellow(name), ui.Cyan(branch2))
		} else if val2 == "" && val1 != "" {
			customDiff = true
			fmt.Printf("  %s: only in %s\n", ui.Yellow(name), ui.Cyan(branch1))
		} else if val1 != val2 {
			customDiff = true
			fmt.Printf("  %s: different values\n", ui.Yellow(name))
		}
	}

	if !customDiff {
		ui.Info("No differences in custom variables")
	}

	ui.NewLine()
	if hasDiff || customDiff {
		ui.Warningf("Environments differ between %s and %s", branch1, branch2)
	} else {
		ui.Success("Environments are identical")
	}

	return nil
}

// parseEnvVariables parses environment variables from a config file.
func parseEnvVariables(content string) map[string]string {
	vars := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		// Handle both = and : = formats (xcconfig vs env)
		var key, value string
		if idx := strings.Index(line, " = "); idx != -1 {
			key = strings.TrimSpace(line[:idx])
			value = strings.TrimSpace(line[idx+3:])
		} else if idx := strings.Index(line, "="); idx != -1 {
			key = strings.TrimSpace(line[:idx])
			value = strings.TrimSpace(line[idx+1:])
		} else {
			continue
		}

		// Remove quotes
		value = strings.Trim(value, "\"'")
		vars[key] = value
	}
	return vars
}

// maskValue masks a sensitive value, showing only the first and last few characters.
func maskValue(value string) string {
	if len(value) <= 8 {
		return "****"
	}
	return value[:4] + "****" + value[len(value)-4:]
}

// truncateValue truncates a value to the specified length.
func truncateValue(value string, maxLen int) string {
	if len(value) <= maxLen {
		return value
	}
	return value[:maxLen-3] + "..."
}

