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
	Short: "Manage Edge Function secrets",
	Long: `View, compare, and copy Edge Function secrets between Supabase branches.

Commands:
  list  - List all secrets for a branch
  diff  - Compare secrets between two branches
  copy  - Copy secrets from one branch to another

Secrets are environment variables available to Edge Functions.
Common secrets include API keys, service tokens, and configuration values.`,
	Example: `  drift secrets list              # List secrets for current branch
  drift secrets diff dev prod     # Compare secrets between dev and prod
  drift secrets copy dev          # Copy from dev to current branch`,
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
	Long: `Copy Edge Function secrets from a source branch to a target branch.

If only source is provided, copies to the current branch.
If no arguments are provided, interactive mode lets you select both.

Requires SUPABASE_ACCESS_TOKEN for automatic copying. Otherwise,
falls back to showing manual commands.`,
	Example: `  drift secrets copy dev          # Copy from dev to current branch
  drift secrets copy dev feature  # Copy from dev to feature branch
  drift secrets copy              # Interactive: select source and target`,
	Args: cobra.MaximumNArgs(2),
	RunE: runSecretsCopy,
}

var secretsDiffCmd = &cobra.Command{
	Use:   "diff [branch1] [branch2]",
	Short: "Compare secrets between two branches",
	Long: `Compare Edge Function secrets between two Supabase branches.

Shows which secrets exist in each branch and highlights differences:
  - Secrets only in first branch
  - Secrets only in second branch
  - Secrets in both branches

If only one branch is provided, compares against the current branch.
If no arguments are provided, interactive mode lets you select both.`,
	Example: `  drift secrets diff dev prod     # Compare dev vs prod
  drift secrets diff dev          # Compare dev vs current branch
  drift secrets diff              # Interactive: select both branches`,
	Args: cobra.MaximumNArgs(2),
	RunE: runSecretsDiff,
}

func init() {
	secretsCmd.AddCommand(secretsListCmd)
	secretsCmd.AddCommand(secretsCopyCmd)
	secretsCmd.AddCommand(secretsDiffCmd)
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

	// Next steps
	ui.NewLine()
	ui.SubHeader("Next Steps")
	ui.List("drift secrets copy <src>  - Copy secrets from another branch")
	ui.List("drift deploy functions    - Redeploy functions to use secrets")

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

	// Try to use Management API for actual secret values
	mgmtClient, mgmtErr := supabase.NewManagementClient()
	if mgmtErr != nil {
		// Fall back to CLI-only mode (names only)
		return copySecretsManual(sourceInfo, targetInfo, sourceBranch, targetBranch)
	}

	// Get secrets with values from source
	sp := ui.NewSpinner("Fetching secrets from source")
	sp.Start()

	sourceSecrets, err := mgmtClient.GetSecrets(sourceInfo.ProjectRef)
	sp.Stop()

	if err != nil {
		ui.Warning(fmt.Sprintf("Could not fetch secrets via API: %v", err))
		return copySecretsManual(sourceInfo, targetInfo, sourceBranch, targetBranch)
	}

	if len(sourceSecrets) == 0 {
		ui.Info("No secrets found on source branch")
		return nil
	}

	ui.Infof("Found %d secret(s) to copy:", len(sourceSecrets))
	for _, s := range sourceSecrets {
		// Show name but mask value
		maskedValue := "****"
		if len(s.Value) > 4 {
			maskedValue = s.Value[:2] + "****" + s.Value[len(s.Value)-2:]
		}
		ui.List(fmt.Sprintf("%s = %s", s.Name, maskedValue))
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

	// Copy secrets to target
	sp = ui.NewSpinner("Copying secrets to target")
	sp.Start()

	err = mgmtClient.SetSecrets(targetInfo.ProjectRef, sourceSecrets)
	sp.Stop()

	if err != nil {
		ui.Error(fmt.Sprintf("Failed to copy secrets: %v", err))
		return copySecretsManual(sourceInfo, targetInfo, sourceBranch, targetBranch)
	}

	ui.Success(fmt.Sprintf("Copied %d secret(s) to %s", len(sourceSecrets), targetBranch))

	// Next steps
	ui.NewLine()
	ui.SubHeader("Next Steps")
	ui.List("drift deploy functions   - Redeploy functions to use new secrets")
	ui.List("drift secrets list       - Verify secrets were copied")

	return nil
}

// copySecretsManual falls back to showing manual commands when API access is unavailable.
func copySecretsManual(sourceInfo, targetInfo *supabase.BranchInfo, sourceBranch, targetBranch string) error {
	// Get secrets names only via CLI
	sourceSecrets, err := listSecrets(sourceInfo.ProjectRef)
	if err != nil {
		return fmt.Errorf("failed to list source secrets: %w", err)
	}

	if len(sourceSecrets) == 0 {
		ui.Info("No secrets found on source branch")
		return nil
	}

	ui.Infof("Found %d secret(s):", len(sourceSecrets))
	for _, s := range sourceSecrets {
		ui.List(s)
	}
	ui.NewLine()

	ui.Warning("Could not access Management API for secret values")
	ui.Info("Set SUPABASE_ACCESS_TOKEN or run 'supabase login' to enable automatic copying")
	ui.NewLine()
	ui.Info("Manual copy options:")
	ui.NewLine()

	ui.Info("1. Set each secret manually:")
	for _, secret := range sourceSecrets {
		fmt.Printf("   supabase secrets set %s=<value> --project-ref %s\n", secret, targetInfo.ProjectRef)
	}

	ui.NewLine()
	ui.Info("2. Copy from Supabase Dashboard:")
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

func runSecretsDiff(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()
	client := supabase.NewClient()

	var branch1, branch2 string

	// Handle args
	if len(args) == 0 {
		// Interactive mode - select both branches
		branches, err := client.GetBranches()
		if err != nil {
			return fmt.Errorf("failed to get branches: %w", err)
		}

		if len(branches) < 2 {
			ui.Warning("Need at least 2 branches to compare")
			return nil
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

		ui.Header("Compare Secrets")

		// Select first branch
		selected1, err := ui.PromptSelect("Select first branch", options)
		if err != nil {
			return err
		}
		branch1 = strings.Split(selected1, " ")[0]

		// Select second branch (filter out first)
		var options2 []string
		for _, opt := range options {
			if !strings.HasPrefix(opt, branch1+" ") {
				options2 = append(options2, opt)
			}
		}

		selected2, err := ui.PromptSelect("Select second branch", options2)
		if err != nil {
			return err
		}
		branch2 = strings.Split(selected2, " ")[0]

	} else if len(args) == 1 {
		// First branch specified, compare with current
		branch1 = args[0]
		var err error
		branch2, err = git.CurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
	} else {
		// Both specified
		branch1 = args[0]
		branch2 = args[1]
	}

	ui.Header("Secrets Diff")

	// Resolve branches
	overrideBranch := cfg.Supabase.OverrideBranch

	info1, err := client.GetBranchInfoWithOverride(branch1, overrideBranch)
	if err != nil {
		return fmt.Errorf("could not resolve branch %s: %w", branch1, err)
	}

	info2, err := client.GetBranchInfoWithOverride(branch2, overrideBranch)
	if err != nil {
		return fmt.Errorf("could not resolve branch %s: %w", branch2, err)
	}

	ui.KeyValue("Branch 1", fmt.Sprintf("%s (%s)", ui.Cyan(branch1), envColorString(string(info1.Environment))))
	ui.KeyValue("Branch 2", fmt.Sprintf("%s (%s)", ui.Cyan(branch2), envColorString(string(info2.Environment))))
	ui.NewLine()

	// Get secrets from both branches
	sp := ui.NewSpinner("Fetching secrets from both branches")
	sp.Start()

	secrets1, err1 := listSecrets(info1.ProjectRef)
	secrets2, err2 := listSecrets(info2.ProjectRef)
	sp.Stop()

	if err1 != nil {
		return fmt.Errorf("failed to get secrets from %s: %w", branch1, err1)
	}
	if err2 != nil {
		return fmt.Errorf("failed to get secrets from %s: %w", branch2, err2)
	}

	// Build maps for comparison
	secrets1Map := make(map[string]bool)
	for _, s := range secrets1 {
		secrets1Map[s] = true
	}

	secrets2Map := make(map[string]bool)
	for _, s := range secrets2 {
		secrets2Map[s] = true
	}

	// Categorize secrets
	var onlyIn1, onlyIn2, inBoth []string

	for _, s := range secrets1 {
		if secrets2Map[s] {
			inBoth = append(inBoth, s)
		} else {
			onlyIn1 = append(onlyIn1, s)
		}
	}

	for _, s := range secrets2 {
		if !secrets1Map[s] {
			onlyIn2 = append(onlyIn2, s)
		}
	}

	// Display results
	if len(onlyIn1) == 0 && len(onlyIn2) == 0 {
		ui.Success("Secrets are identical between branches!")
		ui.NewLine()
		ui.Infof("Both branches have %d secret(s):", len(inBoth))
		for _, s := range inBoth {
			ui.List(s)
		}
	} else {
		// Show differences
		if len(onlyIn1) > 0 {
			ui.SubHeader(fmt.Sprintf("Only in %s (%d)", branch1, len(onlyIn1)))
			for _, s := range onlyIn1 {
				fmt.Printf("  %s %s\n", ui.Yellow("-"), s)
			}
		}

		if len(onlyIn2) > 0 {
			ui.SubHeader(fmt.Sprintf("Only in %s (%d)", branch2, len(onlyIn2)))
			for _, s := range onlyIn2 {
				fmt.Printf("  %s %s\n", ui.Green("+"), s)
			}
		}

		if len(inBoth) > 0 {
			ui.SubHeader(fmt.Sprintf("In both branches (%d)", len(inBoth)))
			for _, s := range inBoth {
				fmt.Printf("  %s %s\n", ui.Dim("="), s)
			}
		}

		// Summary
		ui.NewLine()
		ui.SubHeader("Summary")
		ui.KeyValue(fmt.Sprintf("Only in %s", branch1), ui.Yellow(fmt.Sprintf("%d", len(onlyIn1))))
		ui.KeyValue(fmt.Sprintf("Only in %s", branch2), ui.Green(fmt.Sprintf("%d", len(onlyIn2))))
		ui.KeyValue("In both", fmt.Sprintf("%d", len(inBoth)))
	}

	// Next steps
	ui.NewLine()
	ui.SubHeader("Next Steps")
	if len(onlyIn1) > 0 || len(onlyIn2) > 0 {
		ui.List(fmt.Sprintf("drift secrets copy %s %s  - Sync secrets", branch1, branch2))
	}
	ui.List("drift secrets list <branch>   - View all secrets for a branch")
	ui.List("drift deploy functions        - Redeploy after changes")

	return nil
}
