package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/internal/git"
	"github.com/undrift/drift/internal/supabase"
	"github.com/undrift/drift/internal/ui"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Edge function deployment",
	Long: `Deploy Edge Functions and set secrets for the current environment.

The deploy command determines the target environment from your current
git branch and deploys to the matching Supabase branch.`,
}

var deployFunctionsCmd = &cobra.Command{
	Use:   "functions",
	Short: "Deploy edge functions only",
	Long:  `Deploy all Edge Functions to the target environment.`,
	RunE:  runDeployFunctions,
}

var deploySecretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Set APNs secrets only",
	Long:  `Set APNs and other secrets for the target environment.`,
	RunE:  runDeploySecrets,
}

var deployAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Deploy functions and set secrets",
	Long:  `Deploy all Edge Functions and set all secrets for the target environment.`,
	RunE:  runDeployAll,
}

var deployStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show deployment status",
	Long:  `Show the current deployment status and target environment.`,
	RunE:  runDeployStatus,
}

var (
	deployBranchFlag string
)

func init() {
	// Add branch flag to all deploy commands
	deployFunctionsCmd.Flags().StringVarP(&deployBranchFlag, "branch", "b", "", "Target Supabase branch")
	deploySecretsCmd.Flags().StringVarP(&deployBranchFlag, "branch", "b", "", "Target Supabase branch")
	deployAllCmd.Flags().StringVarP(&deployBranchFlag, "branch", "b", "", "Target Supabase branch")

	deployCmd.AddCommand(deployFunctionsCmd)
	deployCmd.AddCommand(deploySecretsCmd)
	deployCmd.AddCommand(deployAllCmd)
	deployCmd.AddCommand(deployStatusCmd)
	rootCmd.AddCommand(deployCmd)
}

func getDeployTarget() (*supabase.BranchInfo, error) {
	client := supabase.NewClient()

	targetBranch := deployBranchFlag
	if targetBranch == "" {
		var err error
		targetBranch, err = git.CurrentBranch()
		if err != nil {
			return nil, fmt.Errorf("failed to get current branch: %w", err)
		}
	}

	info, err := client.GetBranchInfo(targetBranch)
	if err != nil {
		return nil, err
	}

	return info, nil
}

func runDeployFunctions(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()

	// Get target environment
	sp := ui.NewSpinner("Resolving target environment")
	sp.Start()

	info, err := getDeployTarget()
	if err != nil {
		sp.Fail("Failed to resolve environment")
		return err
	}
	sp.Stop()

	ui.Header("Deploy Edge Functions")
	ui.KeyValue("Environment", envColorString(string(info.Environment)))
	ui.KeyValue("Supabase Branch", ui.Cyan(info.SupabaseBranch.Name))
	ui.KeyValue("Project Ref", ui.Cyan(info.ProjectRef))

	if info.IsFallback {
		ui.Warning("Using fallback environment")
	}

	// Confirm for production
	if info.Environment == supabase.EnvProduction && !IsYes() {
		ui.NewLine()
		ui.Warning("You are about to deploy to PRODUCTION!")
		confirmed, err := ui.PromptYesNo("Continue?", false)
		if err != nil || !confirmed {
			ui.Info("Cancelled")
			return nil
		}
	}

	ui.NewLine()

	// List functions
	functions, err := supabase.ListFunctions(cfg.GetFunctionsPath())
	if err != nil {
		return err
	}

	ui.Infof("Found %d functions to deploy", len(functions))

	// Deploy each function
	client := supabase.NewClient()
	for _, fn := range functions {
		sp := ui.NewSpinner(fmt.Sprintf("Deploying %s", fn.Name))
		sp.Start()

		if err := client.DeployFunction(fn.Name, info.ProjectRef); err != nil {
			sp.Fail(fmt.Sprintf("Failed to deploy %s", fn.Name))
			return err
		}

		sp.Success(fmt.Sprintf("Deployed %s", fn.Name))
	}

	ui.NewLine()
	ui.Success(fmt.Sprintf("Successfully deployed %d functions", len(functions)))

	return nil
}

func runDeploySecrets(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()

	// Get target environment
	sp := ui.NewSpinner("Resolving target environment")
	sp.Start()

	info, err := getDeployTarget()
	if err != nil {
		sp.Fail("Failed to resolve environment")
		return err
	}
	sp.Stop()

	ui.Header("Set Secrets")
	ui.KeyValue("Environment", envColorString(string(info.Environment)))
	ui.KeyValue("Supabase Branch", ui.Cyan(info.SupabaseBranch.Name))
	ui.KeyValue("Project Ref", ui.Cyan(info.ProjectRef))

	// Confirm for production
	if info.Environment == supabase.EnvProduction && !IsYes() {
		ui.NewLine()
		ui.Warning("You are about to set secrets on PRODUCTION!")
		confirmed, err := ui.PromptYesNo("Continue?", false)
		if err != nil || !confirmed {
			ui.Info("Cancelled")
			return nil
		}
	}

	ui.NewLine()

	client := supabase.NewClient()

	// Load APNs secrets
	apnsEnv := cfg.APNS.Environment
	if info.Environment == supabase.EnvProduction {
		apnsEnv = "production"
	}

	apnsSecrets, err := supabase.LoadAPNSSecretsFromConfig(
		cfg.APNS.TeamID,
		cfg.APNS.BundleID,
		cfg.APNS.KeyPattern,
		apnsEnv,
		cfg.ProjectRoot(),
	)
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not load APNs secrets: %v", err))
		ui.Info("Skipping APNs secret setup")
	} else {
		sp = ui.NewSpinner("Setting APNs secrets")
		sp.Start()

		if err := client.SetAPNSSecrets(info.ProjectRef, *apnsSecrets); err != nil {
			sp.Fail("Failed to set APNs secrets")
			return err
		}

		sp.Success("Set APNs secrets")
	}

	// Set debug switch (only for non-production)
	if info.Environment != supabase.EnvProduction {
		sp = ui.NewSpinner("Enabling debug switch")
		sp.Start()

		if err := client.SetDebugSwitch(info.ProjectRef, true); err != nil {
			sp.Fail("Failed to set debug switch")
			return err
		}

		sp.Success("Enabled debug switch")
	} else {
		sp = ui.NewSpinner("Disabling debug switch for production")
		sp.Start()

		if err := client.SetDebugSwitch(info.ProjectRef, false); err != nil {
			sp.Fail("Failed to set debug switch")
			return err
		}

		sp.Success("Disabled debug switch")
	}

	ui.NewLine()
	ui.Success("Secrets configured successfully")

	return nil
}

func runDeployAll(cmd *cobra.Command, args []string) error {
	ui.Header("Full Deployment")

	// Deploy functions
	if err := runDeployFunctions(cmd, args); err != nil {
		return err
	}

	ui.NewLine()

	// Set secrets
	if err := runDeploySecrets(cmd, args); err != nil {
		return err
	}

	ui.NewLine()
	ui.Success("Full deployment complete!")

	return nil
}

func runDeployStatus(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()

	// Get current git branch
	gitBranch, err := git.CurrentBranch()
	if err != nil {
		return err
	}

	client := supabase.NewClient()
	info, err := client.GetBranchInfo(gitBranch)
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not resolve Supabase branch: %v", err))
		ui.KeyValue("Git Branch", ui.Cyan(gitBranch))
		return nil
	}

	ui.Header("Deployment Status")

	ui.KeyValue("Git Branch", ui.Cyan(gitBranch))
	ui.KeyValue("Environment", envColorString(string(info.Environment)))
	ui.KeyValue("Supabase Branch", ui.Cyan(info.SupabaseBranch.Name))
	ui.KeyValue("Project Ref", ui.Cyan(info.ProjectRef))

	if info.IsFallback {
		ui.NewLine()
		ui.Warning("Using fallback: no Supabase branch for this git branch")
	}

	// List functions
	ui.NewLine()
	ui.SubHeader("Edge Functions")

	functions, err := supabase.ListFunctions(cfg.GetFunctionsPath())
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not list functions: %v", err))
	} else {
		for _, fn := range functions {
			ui.List(fn.Name)
		}
		ui.Infof("Total: %d functions", len(functions))
	}

	return nil
}

