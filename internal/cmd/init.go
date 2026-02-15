package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/internal/supabase"
	"github.com/undrift/drift/internal/ui"
	"github.com/undrift/drift/internal/xcode"
	"github.com/undrift/drift/pkg/shell"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize drift in a project",
	Long: `Initialize drift configuration in the current project.

This command creates a .drift.yaml configuration file with 
detected settings and sensible defaults.

Use --fallback-branch to seed supabase.fallback_branch in .drift.local.yaml.`,
	RunE: runInit,
}

var (
	initForceFlag        bool
	initSupabaseProject  string
	initSkipSupabaseLink bool
	initFallbackBranch   string
)

func init() {
	initCmd.Flags().BoolVarP(&initForceFlag, "force", "f", false, "Overwrite existing .drift.yaml")
	initCmd.Flags().StringVarP(&initSupabaseProject, "supabase-project", "s", "", "Supabase project name to link")
	initCmd.Flags().BoolVar(&initSkipSupabaseLink, "skip-link", false, "Skip Supabase project linking")
	initCmd.Flags().StringVar(&initFallbackBranch, "fallback-branch", "", "Set default Supabase fallback branch in .drift.local.yaml")
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

	// Ask for project type (put detected type first)
	typeOptions := []string{"ios", "macos", "multiplatform", "web"}
	// Reorder to put detected type first
	for i, opt := range typeOptions {
		if opt == projectType {
			typeOptions[0], typeOptions[i] = typeOptions[i], typeOptions[0]
			break
		}
	}
	idx, _, err := ui.PromptSelectWithIndex("Project type", typeOptions)
	if err != nil {
		return err
	}
	pType := typeOptions[idx]

	// Handle Supabase project linking
	var supabaseProjectRef, supabaseProjectName string
	if !initSkipSupabaseLink {
		ref, projName, err := resolveSupabaseProject()
		if err != nil {
			ui.Warning(fmt.Sprintf("Could not link Supabase project: %v", err))
			ui.Info("You can link later with 'supabase link'")
		} else {
			supabaseProjectRef = ref
			supabaseProjectName = projName
		}
	}

	// Detect APNs settings (only for Apple platforms)
	var teamID, bundleID string
	if pType != "web" {
		teamID = detectTeamID()
		bundleID = detectBundleID()

		if teamID == "" {
			teamID, _ = ui.PromptString("APNs Team ID (optional)", "")
		}
		if bundleID == "" {
			bundleID, _ = ui.PromptString("Bundle ID (optional)", "")
		}
	}

	// Create configuration
	configContent := generateConfig(name, pType, teamID, bundleID, supabaseProjectRef, supabaseProjectName)

	// Write configuration file
	configPath := ".drift.yaml"
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	ui.NewLine()
	ui.Success(fmt.Sprintf("Created %s", configPath))

	// Write local configuration file
	localConfigPath := config.LocalConfigFilename
	if err := config.WriteLocalConfig(localConfigPath); err != nil {
		ui.Warning(fmt.Sprintf("Could not create %s: %v", localConfigPath, err))
	} else {
		ui.Success(fmt.Sprintf("Created %s", localConfigPath))
		if initFallbackBranch != "" {
			if isProtectedBranchName(initFallbackBranch) {
				ui.Warning(fmt.Sprintf("Refusing to set production-like fallback branch '%s' in %s", initFallbackBranch, localConfigPath))
			} else if err := config.UpdateLocalSupabaseOverrides(localConfigPath, "", initFallbackBranch); err != nil {
				ui.Warning(fmt.Sprintf("Could not set fallback branch in %s: %v", localConfigPath, err))
			} else {
				ui.Success(fmt.Sprintf("Configured local fallback branch: %s", initFallbackBranch))
			}
		}
	}

	// Add .drift.local.yaml to .gitignore
	cwd, _ := os.Getwd()
	if err := config.AddToGitignore(cwd); err != nil {
		ui.Warning(fmt.Sprintf("Could not update .gitignore: %v", err))
	} else {
		ui.Success("Added .drift.local.yaml to .gitignore")
	}

	// Link Supabase project if we have a ref
	if supabaseProjectRef != "" {
		ui.NewLine()
		sp := ui.NewSpinner("Linking Supabase project")
		sp.Start()

		client := supabase.NewClient()
		if err := client.LinkProject(supabaseProjectRef); err != nil {
			sp.Fail("Failed to link Supabase project")
			ui.Warning(fmt.Sprintf("You can link manually: supabase link --project-ref %s", supabaseProjectRef))
		} else {
			sp.Success(fmt.Sprintf("Linked to Supabase project: %s", supabaseProjectName))
		}
	}

	ui.NewLine()

	// Show next steps
	ui.SubHeader("Next Steps")
	if supabaseProjectRef == "" {
		ui.NumberedList(1, "Link Supabase project: supabase link")
		if pType == "web" {
			ui.NumberedList(2, "Run 'drift env setup' to generate .env.local")
			ui.NumberedList(3, "Add .env.local to .gitignore")
		} else {
			ui.NumberedList(2, "Run 'drift env setup' to generate Config.xcconfig")
			ui.NumberedList(3, "Add Config.xcconfig to your Xcode project")
		}
	} else {
		if pType == "web" {
			ui.NumberedList(1, "Run 'drift env setup' to generate .env.local")
			ui.NumberedList(2, "Add .env.local to .gitignore")
		} else {
			ui.NumberedList(1, "Run 'drift env setup' to generate Config.xcconfig")
			ui.NumberedList(2, "Add Config.xcconfig to your Xcode project")
		}
	}

	return nil
}

// resolveSupabaseProject resolves the Supabase project to link.
// Returns (projectRef, projectName, error)
func resolveSupabaseProject() (string, string, error) {
	client := supabase.NewClient()

	// If project specified via flag, use that
	if initSupabaseProject != "" {
		return findSupabaseProjectByName(client, initSupabaseProject)
	}

	// Check if already linked
	if client.IsLinked() {
		ref, err := client.GetProjectRef()
		if err == nil && ref != "" {
			project, err := client.FindProjectByRef(ref)
			if err == nil {
				ui.Infof("Already linked to: %s", project.Name)
				return ref, project.Name, nil
			}
			return ref, "", nil
		}
	}

	// List all projects and let user select
	ui.NewLine()
	sp := ui.NewSpinner("Fetching Supabase projects")
	sp.Start()

	projects, err := client.ListProjects()
	sp.Stop()

	if err != nil {
		return "", "", fmt.Errorf("failed to list projects: %w", err)
	}

	if len(projects) == 0 {
		return "", "", fmt.Errorf("no Supabase projects found")
	}

	// Build options list
	options := make([]string, len(projects))
	for i, p := range projects {
		options[i] = p.FormatProjectDisplay()
	}

	idx, _, err := ui.PromptSelectWithIndex("Select Supabase project", options)
	if err != nil {
		return "", "", err
	}

	selected := projects[idx]
	return selected.Ref, selected.Name, nil
}

// findSupabaseProjectByName finds a project by name, prompting if multiple matches.
func findSupabaseProjectByName(client *supabase.Client, name string) (string, string, error) {
	sp := ui.NewSpinner(fmt.Sprintf("Looking for project: %s", name))
	sp.Start()

	matches, err := client.FindProjectsByName(name)
	sp.Stop()

	if err != nil {
		return "", "", err
	}

	if len(matches) == 0 {
		// Try listing all projects as a fallback
		projects, err := client.ListProjects()
		if err != nil {
			return "", "", fmt.Errorf("project '%s' not found", name)
		}

		// Show available projects
		ui.Warningf("No project named '%s' found", name)
		ui.Info("Available projects:")
		for _, p := range projects {
			ui.List(p.FormatProjectDisplay())
		}
		return "", "", fmt.Errorf("project '%s' not found", name)
	}

	if len(matches) == 1 {
		return matches[0].Ref, matches[0].Name, nil
	}

	// Multiple matches - let user select
	ui.Warningf("Multiple projects named '%s' found", name)
	options := make([]string, len(matches))
	for i, p := range matches {
		options[i] = p.FormatProjectDisplay()
	}

	idx, _, err := ui.PromptSelectWithIndex("Select project", options)
	if err != nil {
		return "", "", err
	}

	selected := matches[idx]
	return selected.Ref, selected.Name, nil
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
	// Check for Next.js / web project (package.json with next)
	if _, err := os.Stat("package.json"); err == nil {
		data, err := os.ReadFile("package.json")
		if err == nil {
			content := string(data)
			// Check for Next.js, React, Vue, etc.
			if strings.Contains(content, `"next"`) {
				return "web"
			}
			if strings.Contains(content, `"react"`) && !strings.Contains(content, `"react-native"`) {
				return "web"
			}
			if strings.Contains(content, `"vue"`) || strings.Contains(content, `"nuxt"`) {
				return "web"
			}
			if strings.Contains(content, `"svelte"`) {
				return "web"
			}
		}
	}

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

func generateConfig(name, projectType, teamID, bundleID, supabaseRef, supabaseName string) string {
	// Detect existing paths
	functionsDir := "supabase/functions"
	migrationsDir := "supabase/migrations"

	// Get worktree naming pattern
	wtPattern := fmt.Sprintf("%s-{branch}", name)

	// Build supabase section
	supabaseSection := ""
	if supabaseRef != "" {
		supabaseSection = fmt.Sprintf(`supabase:
  project_ref: %s
  project_name: %s
  functions_dir: %s
  migrations_dir: %s
  protected_branches:
    - main
    - master
  secrets_to_push:
    - APNS_KEY_ID
    - APNS_TEAM_ID
    - APNS_BUNDLE_ID
    - APNS_PRIVATE_KEY
    - APNS_ENVIRONMENT

`, supabaseRef, supabaseName, functionsDir, migrationsDir)
	} else {
		supabaseSection = fmt.Sprintf(`supabase:
  # project_ref: ""  # Set by 'drift init --supabase-project <name>' or 'supabase link'
  # project_name: ""
  functions_dir: %s
  migrations_dir: %s
  protected_branches:
    - main
    - master
  secrets_to_push:
    - APNS_KEY_ID
    - APNS_TEAM_ID
    - APNS_BUNDLE_ID
    - APNS_PRIVATE_KEY
    - APNS_ENVIRONMENT

`, functionsDir, migrationsDir)
	}

	config := fmt.Sprintf(`# .drift.yaml - Project configuration for drift CLI
# Generated by drift init

project:
  name: %s
  type: %s

%s`, name, projectType, supabaseSection)

	// Add platform-specific config
	if projectType == "web" {
		// Web project config
		config += `web:
  env_output: .env.local

`
	} else {
		// Apple platform config
		// Detect xcconfig path
		xcconfigPath := "Config.xcconfig"
		if matches, _ := filepath.Glob("*.xcconfig"); len(matches) > 0 {
			xcconfigPath = filepath.Dir(matches[0]) + "/Config.xcconfig"
			if xcconfigPath == "./Config.xcconfig" {
				xcconfigPath = "Config.xcconfig"
			}
		}

		// Add Apple config if we have values
		if teamID != "" || bundleID != "" {
			config += fmt.Sprintf(`apple:
  team_id: "%s"
  bundle_id: "%s"
  push_key_pattern: "AuthKey_*.p8"
  push_environment: development
  key_search_paths:
    - secrets
    - .
    - ..

`, teamID, bundleID)
		} else {
			config += `# apple:
#   team_id: "YOUR_TEAM_ID"
#   bundle_id: "com.yourcompany.yourapp"
#   push_key_pattern: "AuthKey_*.p8"
#   push_environment: development
#   key_search_paths:
#     - secrets
#     - .
#     - ..

`
		}

		// Detect Xcode schemes
		detectedSchemes := xcode.SuggestSchemes(name)
		schemesConfig := ""
		if len(detectedSchemes) > 0 {
			schemesConfig = "  schemes:\n"
			if s, ok := detectedSchemes["production"]; ok {
				schemesConfig += fmt.Sprintf("    production: \"%s\"\n", s)
			}
			if s, ok := detectedSchemes["development"]; ok {
				schemesConfig += fmt.Sprintf("    development: \"%s\"\n", s)
			}
			if s, ok := detectedSchemes["feature"]; ok {
				schemesConfig += fmt.Sprintf("    feature: \"%s\"\n", s)
			}
		} else {
			schemesConfig = `  # schemes:
  #   production: "App (Production)"
  #   development: "App (Development)"
  #   feature: "App (Feature)"
`
		}

		config += fmt.Sprintf(`xcode:
  xcconfig_output: %s
  version_file: Version.xcconfig
%s
`, xcconfigPath, schemesConfig)
	}

	// Common sections
	config += fmt.Sprintf(`database:
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
    - .env.local
  auto_setup_xcconfig: true
`, wtPattern)

	return config
}
