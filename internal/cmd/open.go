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

var openCmd = &cobra.Command{
	Use:   "open [target]",
	Short: "Open Supabase dashboard or related URLs",
	Long: `Open the Supabase dashboard or related service URLs in your browser.

Available targets:
  dashboard, db     Supabase project dashboard
  editor, tables    Table editor
  sql               SQL editor
  functions, fn     Edge Functions
  auth              Authentication
  storage           Storage buckets
  logs              Logs Explorer
  settings          Project settings
  api, keys         API settings (keys)
  branches          Branch management

  github, gh        GitHub repository
  actions, ci       GitHub Actions
  vercel            Vercel dashboard (if configured)
  deploy            Deployment URL (production/staging)

  appstore, asc     App Store Connect (iOS/macOS projects)

If no target is specified, opens an interactive picker.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runOpen,
}

func init() {
	rootCmd.AddCommand(openCmd)
}

type openTarget struct {
	name        string
	aliases     []string
	description string
	getURL      func(cfg *config.Config, info *supabase.BranchInfo) string
	requiresRef bool // Requires Supabase project ref
}

var openTargets = []openTarget{
	{
		name:        "dashboard",
		aliases:     []string{"db", "project"},
		description: "Supabase Dashboard",
		getURL: func(cfg *config.Config, info *supabase.BranchInfo) string {
			return GetProjectDashboardURL(info.ProjectRef)
		},
		requiresRef: true,
	},
	{
		name:        "editor",
		aliases:     []string{"tables", "table"},
		description: "Table Editor",
		getURL: func(cfg *config.Config, info *supabase.BranchInfo) string {
			return GetTableEditorURL(info.ProjectRef)
		},
		requiresRef: true,
	},
	{
		name:        "sql",
		aliases:     []string{"query"},
		description: "SQL Editor",
		getURL: func(cfg *config.Config, info *supabase.BranchInfo) string {
			return GetSQLEditorURL(info.ProjectRef)
		},
		requiresRef: true,
	},
	{
		name:        "functions",
		aliases:     []string{"fn", "func"},
		description: "Edge Functions",
		getURL: func(cfg *config.Config, info *supabase.BranchInfo) string {
			return GetFunctionsURL(info.ProjectRef)
		},
		requiresRef: true,
	},
	{
		name:        "auth",
		aliases:     []string{"users"},
		description: "Authentication",
		getURL: func(cfg *config.Config, info *supabase.BranchInfo) string {
			return GetAuthURL(info.ProjectRef)
		},
		requiresRef: true,
	},
	{
		name:        "storage",
		aliases:     []string{"buckets"},
		description: "Storage Buckets",
		getURL: func(cfg *config.Config, info *supabase.BranchInfo) string {
			return GetStorageURL(info.ProjectRef)
		},
		requiresRef: true,
	},
	{
		name:        "logs",
		aliases:     []string{"log"},
		description: "Logs Explorer",
		getURL: func(cfg *config.Config, info *supabase.BranchInfo) string {
			return GetLogsURL(info.ProjectRef)
		},
		requiresRef: true,
	},
	{
		name:        "settings",
		aliases:     []string{"config"},
		description: "Project Settings",
		getURL: func(cfg *config.Config, info *supabase.BranchInfo) string {
			return GetSettingsURL(info.ProjectRef)
		},
		requiresRef: true,
	},
	{
		name:        "api",
		aliases:     []string{"keys", "apikeys"},
		description: "API Keys",
		getURL: func(cfg *config.Config, info *supabase.BranchInfo) string {
			return GetAPISettingsURL(info.ProjectRef)
		},
		requiresRef: true,
	},
	{
		name:        "branches",
		aliases:     []string{"branch"},
		description: "Branch Management",
		getURL: func(cfg *config.Config, info *supabase.BranchInfo) string {
			return GetBranchesURL(info.ProjectRef)
		},
		requiresRef: true,
	},
	{
		name:        "github",
		aliases:     []string{"gh", "repo"},
		description: "GitHub Repository",
		getURL: func(cfg *config.Config, info *supabase.BranchInfo) string {
			return GetRepoURL(cfg)
		},
		requiresRef: false,
	},
	{
		name:        "actions",
		aliases:     []string{"ci", "workflows"},
		description: "GitHub Actions",
		getURL: func(cfg *config.Config, info *supabase.BranchInfo) string {
			return GetGitHubActionsURL(cfg)
		},
		requiresRef: false,
	},
	{
		name:        "vercel",
		aliases:     []string{},
		description: "Vercel Dashboard",
		getURL: func(cfg *config.Config, info *supabase.BranchInfo) string {
			return GetVercelURL(cfg)
		},
		requiresRef: false,
	},
	{
		name:        "deploy",
		aliases:     []string{"live", "site"},
		description: "Deployment URL",
		getURL: func(cfg *config.Config, info *supabase.BranchInfo) string {
			if info != nil {
				return GetDeployURL(cfg, info.Environment)
			}
			return GetDeployURL(cfg, supabase.EnvProduction)
		},
		requiresRef: false,
	},
	{
		name:        "appstore",
		aliases:     []string{"asc", "appstoreconnect"},
		description: "App Store Connect",
		getURL: func(cfg *config.Config, info *supabase.BranchInfo) string {
			return GetAppStoreConnectURL()
		},
		requiresRef: false,
	},
}

func runOpen(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()

	// Get branch info for Supabase targets
	var info *supabase.BranchInfo
	gitBranch, err := git.CurrentBranch()
	if err == nil {
		client := supabase.NewClient()
		info, _ = client.GetBranchInfoWithOverride(gitBranch, cfg.Supabase.OverrideBranch)
	}

	var targetName string
	if len(args) == 1 {
		targetName = args[0]
	} else {
		// Interactive selection
		options := []string{}
		for _, t := range openTargets {
			// Skip targets that require ref if we don't have one
			if t.requiresRef && info == nil {
				continue
			}
			options = append(options, fmt.Sprintf("%s - %s", t.name, t.description))
		}

		idx, _, err := ui.PromptSelectWithIndex("What would you like to open?", options)
		if err != nil {
			return err
		}

		// Find actual target (accounting for skipped ones)
		validIdx := 0
		for _, t := range openTargets {
			if t.requiresRef && info == nil {
				continue
			}
			if validIdx == idx {
				targetName = t.name
				break
			}
			validIdx++
		}
	}

	// Find target
	var selectedTarget *openTarget
	for i, t := range openTargets {
		if t.name == targetName {
			selectedTarget = &openTargets[i]
			break
		}
		// Check aliases
		for _, alias := range t.aliases {
			if alias == targetName {
				selectedTarget = &openTargets[i]
				break
			}
		}
		if selectedTarget != nil {
			break
		}
	}

	if selectedTarget == nil {
		return fmt.Errorf("unknown target: %s", targetName)
	}

	// Check if we need branch info
	if selectedTarget.requiresRef && info == nil {
		return fmt.Errorf("could not resolve Supabase branch - required for '%s'", selectedTarget.name)
	}

	// Get URL
	url := selectedTarget.getURL(cfg, info)
	if url == "" {
		return fmt.Errorf("no URL configured for '%s'", selectedTarget.name)
	}

	// Show what we're opening
	ui.Infof("Opening %s...", selectedTarget.description)
	if info != nil && selectedTarget.requiresRef {
		ui.KeyValue("Environment", envColorString(string(info.Environment)))
		ui.KeyValue("Project Ref", ui.Cyan(info.ProjectRef))
	}

	// Open in browser
	if err := shell.RunInteractive("open", url); err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}

	return nil
}
