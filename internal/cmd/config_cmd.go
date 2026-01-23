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
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management",
	Long:  `View and modify drift configuration.`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Display the current drift configuration.`,
	RunE:  runConfigShow,
}

var configSetBranchCmd = &cobra.Command{
	Use:   "set-branch [supabase-branch]",
	Short: "Set the Supabase branch override",
	Long: `Set an override branch to use instead of the git branch name.

When set, all drift commands will use this Supabase branch for credentials
instead of trying to match by git branch name.

If no branch is specified, shows available branches for selection.`,
	Example: `  drift config set-branch           # Interactive selection
  drift config set-branch my-branch  # Set specific branch`,
	Args: cobra.MaximumNArgs(1),
	RunE: runConfigSetBranch,
}

var configClearBranchCmd = &cobra.Command{
	Use:   "clear-branch",
	Short: "Clear the Supabase branch override",
	Long:  `Remove the override branch, returning to automatic branch detection.`,
	Example: `  drift config clear-branch  # Return to automatic detection`,
	RunE:    runConfigClearBranch,
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetBranchCmd)
	configCmd.AddCommand(configClearBranchCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	ui.Header("Drift Configuration")

	ui.SubHeader("Project")
	ui.KeyValue("Name", cfg.Project.Name)
	ui.KeyValue("Type", cfg.Project.Type)
	ui.KeyValue("Config File", cfg.ConfigPath())

	ui.NewLine()
	ui.SubHeader("Supabase")
	ui.KeyValue("Project Ref", cfg.Supabase.ProjectRef)
	ui.KeyValue("Project Name", cfg.Supabase.ProjectName)

	if cfg.Supabase.OverrideBranch != "" {
		ui.KeyValue("Override Branch", ui.Cyan(cfg.Supabase.OverrideBranch))
	} else {
		ui.KeyValue("Override Branch", "(none - using git branch)")
	}

	// Show current resolution
	gitBranch, err := git.CurrentBranch()
	if err == nil {
		ui.NewLine()
		ui.SubHeader("Current Branch Resolution")
		ui.KeyValue("Git Branch", gitBranch)

		client := supabase.NewClient()
		info, err := client.GetBranchInfoWithOverride(gitBranch, cfg.Supabase.OverrideBranch)
		if err == nil {
			ui.KeyValue("Supabase Branch", info.SupabaseBranch.Name)
			ui.KeyValue("Environment", string(info.Environment))
			if info.IsOverride {
				ui.Infof("(using override)")
			}
			if info.IsFallback {
				ui.Infof("(fallback to development)")
			}
		}
	}

	return nil
}

func runConfigSetBranch(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}

	var targetBranch string

	if len(args) > 0 {
		targetBranch = args[0]
	} else {
		// Interactive selection
		client := supabase.NewClient()

		sp := ui.NewSpinner("Fetching Supabase branches")
		sp.Start()

		branches, err := client.GetBranches()
		if err != nil {
			sp.Fail("Failed to fetch branches")
			return err
		}
		sp.Stop()

		if len(branches) == 0 {
			return fmt.Errorf("no Supabase branches found")
		}

		// Get current git branch for fuzzy matching hints
		gitBranch, _ := git.CurrentBranch()
		if gitBranch != "" {
			similar, _ := client.FindSimilarBranches(gitBranch)
			if len(similar) > 0 {
				ui.Infof("Similar branches to '%s':", gitBranch)
				for _, b := range similar {
					ui.List(fmt.Sprintf("%s (%s)", b.Name, b.GitBranch))
				}
				ui.NewLine()
			}
		}

		// Build options for selection
		options := make([]string, len(branches))
		for i, b := range branches {
			env := "feature"
			if b.IsDefault {
				env = "production"
			} else if b.Persistent {
				env = "development"
			}
			options[i] = fmt.Sprintf("%s [%s]", b.Name, env)
		}

		selected, err := ui.PromptSelect("Select Supabase branch to use:", options)
		if err != nil {
			return err
		}

		// Find the selected branch
		for i, opt := range options {
			if opt == selected {
				targetBranch = branches[i].Name
				break
			}
		}
	}

	// Verify branch exists
	client := supabase.NewClient()
	branch, err := client.GetBranch(targetBranch)
	if err != nil {
		// Try fuzzy match
		similar, searchErr := client.FindSimilarBranches(targetBranch)
		if searchErr == nil && len(similar) > 0 {
			ui.Warningf("Branch '%s' not found. Did you mean:", targetBranch)
			for _, b := range similar {
				ui.List(b.Name)
			}
		}
		return fmt.Errorf("Supabase branch '%s' not found", targetBranch)
	}

	// Update config file
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if err := updateConfigOverrideBranch(cfg.ConfigPath(), targetBranch); err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	env := "feature"
	if branch.IsDefault {
		env = "production"
	} else if branch.Persistent {
		env = "development"
	}

	ui.Success(fmt.Sprintf("Set override branch to '%s' (%s)", targetBranch, env))
	ui.Infof("All drift commands will now use this Supabase branch")

	return nil
}

func runConfigClearBranch(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.Supabase.OverrideBranch == "" {
		ui.Info("No override branch is currently set")
		return nil
	}

	if err := updateConfigOverrideBranch(cfg.ConfigPath(), ""); err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	ui.Success("Cleared override branch")
	ui.Infof("Drift will now use automatic branch detection based on git branch")

	return nil
}

// updateConfigOverrideBranch updates the override_branch in the config file.
func updateConfigOverrideBranch(configPath, branch string) error {
	// Read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	// Parse as generic map to preserve structure
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}

	// Get or create supabase section
	supabaseSection, ok := cfg["supabase"].(map[string]interface{})
	if !ok {
		supabaseSection = make(map[string]interface{})
		cfg["supabase"] = supabaseSection
	}

	// Update or remove override_branch
	if branch != "" {
		supabaseSection["override_branch"] = branch
	} else {
		delete(supabaseSection, "override_branch")
	}

	// Write back
	newData, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(configPath, newData, 0644)
}
