package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/internal/git"
	"github.com/undrift/drift/internal/supabase"
	"github.com/undrift/drift/internal/ui"
	"github.com/undrift/drift/pkg/shell"
)

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage edge function secrets",
	Long:  `View and copy edge function secrets between Supabase branches.`,
}

var secretsListCmd = &cobra.Command{
	Use:   "list [branch]",
	Short: "List secrets for a branch",
	Long: `List all edge function secrets configured for a Supabase branch.

If no branch is specified, uses the current git branch.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSecretsList,
}

var secretsCopyCmd = &cobra.Command{
	Use:   "copy [source] [target]",
	Short: "Copy secrets from one branch to another",
	Long: `Copy edge function secrets from a source branch to a target branch.

Examples:
  drift secrets copy dev          # Copy from dev to current branch
  drift secrets copy dev feature  # Copy from dev to feature branch
  drift secrets copy              # Interactive: select source and target`,
	Args: cobra.MaximumNArgs(2),
	RunE: runSecretsCopy,
}

func init() {
	secretsCmd.AddCommand(secretsListCmd)
	secretsCmd.AddCommand(secretsCopyCmd)
	rootCmd.AddCommand(secretsCmd)
}

func runSecretsList(cmd *cobra.Command, args []string) error {
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

	ui.Header("Edge Function Secrets")

	// Resolve Supabase branch
	client := supabase.NewClient()
	overrideBranch := cfg.Supabase.OverrideBranch
	info, err := client.GetBranchInfoWithOverride(targetBranch, overrideBranch)
	if err != nil {
		return fmt.Errorf("could not resolve Supabase branch: %w", err)
	}

	ui.KeyValue("Git Branch", ui.Cyan(targetBranch))
	ui.KeyValue("Environment", envColorString(string(info.Environment)))
	ui.KeyValue("Project Ref", ui.Cyan(info.ProjectRef))
	ui.NewLine()

	// List secrets
	secrets, err := listSecrets(info.ProjectRef)
	if err != nil {
		return fmt.Errorf("failed to list secrets: %w", err)
	}

	if len(secrets) == 0 {
		ui.Info("No secrets configured")
		return nil
	}

	ui.Infof("Found %d secret(s):", len(secrets))
	ui.NewLine()

	for _, secret := range secrets {
		ui.List(secret)
	}

	return nil
}

func runSecretsCopy(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()
	client := supabase.NewClient()

	var sourceBranch, targetBranch string

	// Handle args
	if len(args) == 0 {
		// Interactive mode - select both
		branches, err := client.GetBranches()
		if err != nil {
			return fmt.Errorf("failed to get branches: %w", err)
		}

		// Build options
		options := make([]string, len(branches))
		for i, b := range branches {
			envType := "Feature"
			if b.IsDefault {
				envType = "Production"
			} else if b.Persistent {
				envType = "Development"
			}
			options[i] = fmt.Sprintf("%s (%s)", b.GitBranch, envType)
		}

		ui.Header("Copy Secrets")

		// Select source
		sourceSelected, err := ui.PromptSelect("Copy secrets FROM", options)
		if err != nil {
			return err
		}
		sourceBranch = strings.Split(sourceSelected, " ")[0]

		// Select target (filter out source)
		var targetOptions []string
		for _, opt := range options {
			if !strings.HasPrefix(opt, sourceBranch+" ") {
				targetOptions = append(targetOptions, opt)
			}
		}

		targetSelected, err := ui.PromptSelect("Copy secrets TO", targetOptions)
		if err != nil {
			return err
		}
		targetBranch = strings.Split(targetSelected, " ")[0]

	} else if len(args) == 1 {
		// Source specified, target is current branch
		sourceBranch = args[0]
		var err error
		targetBranch, err = git.CurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
	} else {
		// Both specified
		sourceBranch = args[0]
		targetBranch = args[1]
	}

	ui.Header("Copy Secrets")

	// Resolve branches
	overrideBranch := cfg.Supabase.OverrideBranch

	sourceInfo, err := client.GetBranchInfoWithOverride(sourceBranch, overrideBranch)
	if err != nil {
		return fmt.Errorf("could not resolve source branch: %w", err)
	}

	targetInfo, err := client.GetBranchInfoWithOverride(targetBranch, overrideBranch)
	if err != nil {
		return fmt.Errorf("could not resolve target branch: %w", err)
	}

	ui.KeyValue("From", fmt.Sprintf("%s (%s)", ui.Cyan(sourceBranch), envColorString(string(sourceInfo.Environment))))
	ui.KeyValue("To", fmt.Sprintf("%s (%s)", ui.Cyan(targetBranch), envColorString(string(targetInfo.Environment))))
	ui.NewLine()

	// Get secrets from source
	sp := ui.NewSpinner("Fetching secrets from source")
	sp.Start()

	sourceSecrets, err := listSecrets(sourceInfo.ProjectRef)
	sp.Stop()

	if err != nil {
		return fmt.Errorf("failed to list source secrets: %w", err)
	}

	if len(sourceSecrets) == 0 {
		ui.Info("No secrets found on source branch")
		return nil
	}

	ui.Infof("Found %d secret(s) to copy:", len(sourceSecrets))
	for _, s := range sourceSecrets {
		ui.List(s)
	}
	ui.NewLine()

	// Confirm
	if !IsYes() {
		confirmed, err := ui.PromptYesNo(fmt.Sprintf("Copy %d secret(s) to %s?", len(sourceSecrets), targetBranch), true)
		if err != nil || !confirmed {
			ui.Info("Cancelled")
			return nil
		}
	}

	ui.NewLine()

	// Copy secrets - we need to get the actual values
	// Unfortunately supabase secrets list doesn't show values, only names
	// So we need to use the experimental API or ask user to provide them

	ui.Warning("Note: Supabase CLI doesn't expose secret values")
	ui.Info("You'll need to manually set secrets using:")
	ui.NewLine()

	for _, secret := range sourceSecrets {
		fmt.Printf("  supabase secrets set %s=<value> --project-ref %s\n", secret, targetInfo.ProjectRef)
	}

	ui.NewLine()
	ui.Info("Or copy from Supabase Dashboard:")
	ui.List(fmt.Sprintf("Source: https://supabase.com/dashboard/project/%s/settings/functions", sourceInfo.ProjectRef))
	ui.List(fmt.Sprintf("Target: https://supabase.com/dashboard/project/%s/settings/functions", targetInfo.ProjectRef))

	return nil
}

// listSecrets returns a list of secret names for a project
func listSecrets(projectRef string) ([]string, error) {
	result, err := shell.Run("supabase", "secrets", "list", "--project-ref", projectRef)
	if err != nil {
		return nil, err
	}

	var secrets []string
	lines := strings.Split(result.Stdout, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip header and empty lines
		if line == "" || strings.HasPrefix(line, "NAME") || strings.HasPrefix(line, "-") {
			continue
		}

		// Extract secret name (first column)
		parts := strings.Fields(line)
		if len(parts) > 0 {
			secrets = append(secrets, parts[0])
		}
	}

	return secrets, nil
}
