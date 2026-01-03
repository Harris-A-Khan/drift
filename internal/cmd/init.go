package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/internal/git"
	"github.com/undrift/drift/internal/ui"
	"github.com/undrift/drift/pkg/shell"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize drift in a project",
	Long: `Initialize drift configuration in the current project.

This command creates a .drift.yaml configuration file with 
detected settings and sensible defaults.`,
	RunE: runInit,
}

var initForceFlag bool

func init() {
	initCmd.Flags().BoolVarP(&initForceFlag, "force", "f", false, "Overwrite existing .drift.yaml")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	ui.Header("Initialize Drift")

	// Check if already initialized
	if config.Exists() && !initForceFlag {
		configPath, _ := config.FindConfigFile()
		ui.Warningf("Already initialized at %s", configPath)
		ui.Info("Use --force to overwrite")
		return nil
	}

	// Detect project settings
	projectName := detectProjectName()
	projectType := detectProjectType()

	ui.SubHeader("Detected Settings")
	ui.KeyValue("Project Name", projectName)
	ui.KeyValue("Project Type", projectType)

	// Interactive configuration
	ui.NewLine()
	ui.SubHeader("Configuration")

	// Ask for project name
	name, err := ui.PromptString("Project name", projectName)
	if err != nil {
		return err
	}

	// Ask for project type
	typeOptions := []string{"ios", "macos", "multiplatform"}
	idx, _, err := ui.PromptSelectWithIndex("Project type", typeOptions)
	if err != nil {
		return err
	}
	pType := typeOptions[idx]

	// Detect APNs settings
	teamID := detectTeamID()
	bundleID := detectBundleID()

	if teamID == "" {
		teamID, _ = ui.PromptString("APNs Team ID (optional)", "")
	}
	if bundleID == "" {
		bundleID, _ = ui.PromptString("Bundle ID (optional)", "")
	}

	// Create configuration
	configContent := generateConfig(name, pType, teamID, bundleID)

	// Write configuration file
	configPath := ".drift.yaml"
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	ui.NewLine()
	ui.Success(fmt.Sprintf("Created %s", configPath))
	ui.NewLine()

	// Show next steps
	ui.SubHeader("Next Steps")
	ui.NumberedList(1, "Run 'drift doctor' to check dependencies")
	ui.NumberedList(2, "Run 'drift env setup' to generate Config.xcconfig")
	ui.NumberedList(3, "Add Config.xcconfig to your Xcode project")

	return nil
}

func detectProjectName() string {
	// Try to get from git remote
	result, err := shell.Run("git", "remote", "get-url", "origin")
	if err == nil && result.Stdout != "" {
		url := result.Stdout
		// Extract repo name from URL
		// https://github.com/user/repo.git or git@github.com:user/repo.git
		parts := strings.Split(url, "/")
		if len(parts) > 0 {
			name := parts[len(parts)-1]
			name = strings.TrimSuffix(name, ".git")
			return name
		}
	}

	// Try to get from directory name
	cwd, err := os.Getwd()
	if err == nil {
		return filepath.Base(cwd)
	}

	return "my-project"
}

func detectProjectType() string {
	// Check for .xcodeproj or .xcworkspace
	matches, _ := filepath.Glob("*.xcodeproj")
	if len(matches) > 0 {
		return "ios"
	}

	matches, _ = filepath.Glob("*.xcworkspace")
	if len(matches) > 0 {
		return "ios"
	}

	// Check for Package.swift (could be macOS or multiplatform)
	if _, err := os.Stat("Package.swift"); err == nil {
		return "multiplatform"
	}

	return "ios"
}

func detectTeamID() string {
	// Try to read from existing xcconfig or pbxproj
	matches, _ := filepath.Glob("*.xcodeproj/project.pbxproj")
	if len(matches) > 0 {
		data, err := os.ReadFile(matches[0])
		if err == nil {
			content := string(data)
			// Look for DEVELOPMENT_TEAM
			if idx := strings.Index(content, "DEVELOPMENT_TEAM = "); idx != -1 {
				end := strings.Index(content[idx:], ";")
				if end != -1 {
					teamID := content[idx+len("DEVELOPMENT_TEAM = ") : idx+end]
					teamID = strings.Trim(teamID, " \"")
					if teamID != "" && teamID != "$(inherited)" {
						return teamID
					}
				}
			}
		}
	}

	return ""
}

func detectBundleID() string {
	// Try to read from Info.plist or xcconfig
	matches, _ := filepath.Glob("**/Info.plist")
	for _, match := range matches {
		// Skip Pods
		if strings.Contains(match, "Pods/") {
			continue
		}

		result, err := shell.Run("plutil", "-extract", "CFBundleIdentifier", "raw", match)
		if err == nil && result.Stdout != "" && !strings.HasPrefix(result.Stdout, "$(") {
			return result.Stdout
		}
	}

	return ""
}

func generateConfig(name, projectType, teamID, bundleID string) string {
	// Detect existing paths
	functionsDir := "supabase/functions"
	migrationsDir := "supabase/migrations"

	if _, err := os.Stat("supabase/functions"); os.IsNotExist(err) {
		functionsDir = "supabase/functions"
	}

	// Detect xcconfig path
	xcconfigPath := "Config.xcconfig"
	if matches, _ := filepath.Glob("*.xcconfig"); len(matches) > 0 {
		// Use existing xcconfig location as hint
		xcconfigPath = filepath.Dir(matches[0]) + "/Config.xcconfig"
		if xcconfigPath == "./Config.xcconfig" {
			xcconfigPath = "Config.xcconfig"
		}
	}

	// Detect PG bin
	pgBin := "/opt/homebrew/opt/postgresql@16/bin"
	if _, err := os.Stat("/opt/homebrew/opt/postgresql@16/bin/pg_dump"); os.IsNotExist(err) {
		if _, err := os.Stat("/opt/homebrew/opt/postgresql@15/bin/pg_dump"); err == nil {
			pgBin = "/opt/homebrew/opt/postgresql@15/bin"
		}
	}

	// Get worktree naming pattern
	wtPattern := fmt.Sprintf("%s-{branch}", name)

	config := fmt.Sprintf(`# .drift.yaml - Project configuration for drift CLI
# Generated by drift init

project:
  name: %s
  type: %s

supabase:
  project_ref_file: .supabase-project-ref
  functions_dir: %s
  migrations_dir: %s
  protected_branches:
    - main
    - master

`, name, projectType, functionsDir, migrationsDir)

	// Add APNs config if we have values
	if teamID != "" || bundleID != "" {
		config += fmt.Sprintf(`apns:
  team_id: "%s"
  bundle_id: "%s"
  key_pattern: "AuthKey_*.p8"
  environment: development

`, teamID, bundleID)
	} else {
		config += `# apns:
#   team_id: "YOUR_TEAM_ID"
#   bundle_id: "com.yourcompany.yourapp"
#   key_pattern: "AuthKey_*.p8"
#   environment: development

`
	}

	config += fmt.Sprintf(`xcode:
  xcconfig_output: %s
  version_file: Version.xcconfig
  # schemes:
  #   production: "App (Production)"
  #   development: "App (Development)"
  #   feature: "App (Feature)"

database:
  pg_bin: %s
  pooler_host: aws-0-us-east-1.pooler.supabase.com
  pooler_port: 6543
  direct_port: 5432

backup:
  provider: supabase
  bucket: database-backups
  retention_days: 30

worktree:
  naming_pattern: "%s"
  copy_on_create:
    - .env
    - "*.p8"
  auto_setup_xcconfig: true
`, xcconfigPath, pgBin, wtPattern)

	return config
}

// initIfNeeded checks if initialization is needed and suggests running init.
func initIfNeeded() bool {
	if !config.Exists() {
		if !git.IsGitRepository() {
			ui.Error("Not in a git repository")
			return false
		}
		ui.Info("No .drift.yaml found")
		ui.Infof("Run 'drift init' to create one")
		return false
	}
	return true
}

