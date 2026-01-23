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
	Short: "Deploy Edge Functions and environment secrets",
	Long: `Deploy Edge Functions and set environment secrets for the current environment.

The deploy command determines the target environment from your current
git branch and deploys to the matching Supabase branch.

Commands:
  functions    - Deploy all Edge Functions
  secrets      - Set environment secrets (APNs, debug switch, etc.)
  all          - Deploy functions and set secrets
  status       - Show deployment target and local functions
  list-secrets - List configured secrets on environment`,
	Example: `  drift deploy functions        # Deploy all functions
  drift deploy secrets          # Set environment secrets
  drift deploy all              # Full deployment
  drift deploy status           # Check deployment target`,
}

var deployFunctionsCmd = &cobra.Command{
	Use:   "functions",
	Short: "Deploy all Edge Functions",
	Long: `Deploy all Edge Functions from your local project to the target environment.

Functions are deployed from the supabase/functions directory (or as
configured in .drift.yaml). Each function is deployed individually
and progress is shown during deployment.

Use --no-verify-jwt to deploy functions that don't require authentication.`,
	Example: `  drift deploy functions             # Deploy to current branch's environment
  drift deploy functions -b dev      # Deploy to dev environment
  drift deploy functions --no-verify-jwt  # Skip JWT verification`,
	RunE: runDeployFunctions,
}

var deploySecretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Set environment secrets",
	Long: `Set environment secrets for Edge Functions.

This configures secrets that are available to Edge Functions at runtime:
  - APNs credentials (for push notifications)
  - Debug switch (enabled for non-production)
  - Other environment-specific secrets

Production environments have the debug switch disabled automatically.`,
	Example: `  drift deploy secrets           # Set secrets for current environment
  drift deploy secrets -b prod   # Set secrets for production`,
	RunE: runDeploySecrets,
}

var deployAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Deploy functions and set secrets",
	Long: `Perform a full deployment: deploy all Edge Functions and set environment secrets.

This is equivalent to running:
  drift deploy functions
  drift deploy secrets

Confirmation is required for production deployments unless --yes is used.`,
	Example: `  drift deploy all           # Full deployment
  drift deploy all -y        # Skip confirmation
  drift deploy all -b prod   # Deploy to production`,
	RunE: runDeployAll,
}

var deployStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show deployment status",
	Long: `Show the current deployment target and list local Edge Functions.

Displays:
  - Current git branch
  - Target Supabase environment
  - List of local functions ready to deploy

Use 'drift functions list' for a comparison of local vs deployed functions.`,
	Example: `  drift deploy status`,
	RunE:    runDeployStatus,
}

var deployListSecretsCmd = &cobra.Command{
	Use:   "list-secrets",
	Short: "List secrets on target environment",
	Long: `List all secrets configured on the target Supabase environment.

Shows the names of all secrets (values are not displayed for security).
Use this to verify secrets are configured before deploying functions.`,
	Example: `  drift deploy list-secrets        # List for current environment
  drift deploy list-secrets -b dev # List for dev environment`,
	RunE: runDeployListSecrets,
}

var (
	deployBranchFlag     string
	deployNoVerifyJWT    bool
)

func init() {
	// Add branch flag to all deploy commands
	deployFunctionsCmd.Flags().StringVarP(&deployBranchFlag, "branch", "b", "", "Target Supabase branch")
	deploySecretsCmd.Flags().StringVarP(&deployBranchFlag, "branch", "b", "", "Target Supabase branch")
	deployAllCmd.Flags().StringVarP(&deployBranchFlag, "branch", "b", "", "Target Supabase branch")
	deployListSecretsCmd.Flags().StringVarP(&deployBranchFlag, "branch", "b", "", "Target Supabase branch")

	// Add --no-verify-jwt flag to functions deployment
	deployFunctionsCmd.Flags().BoolVar(&deployNoVerifyJWT, "no-verify-jwt", false, "Deploy functions without JWT verification")
	deployAllCmd.Flags().BoolVar(&deployNoVerifyJWT, "no-verify-jwt", false, "Deploy functions without JWT verification")

	deployCmd.AddCommand(deployFunctionsCmd)
	deployCmd.AddCommand(deploySecretsCmd)
	deployCmd.AddCommand(deployAllCmd)
	deployCmd.AddCommand(deployStatusCmd)
	deployCmd.AddCommand(deployListSecretsCmd)
	rootCmd.AddCommand(deployCmd)
}

func getDeployTarget() (*supabase.BranchInfo, error) {
	cfg := config.LoadOrDefault()
	client := supabase.NewClient()

	targetBranch := deployBranchFlag
	if targetBranch == "" {
		var err error
		targetBranch, err = git.CurrentBranch()
		if err != nil {
			return nil, fmt.Errorf("failed to get current branch: %w", err)
		}
	}

	// Apply override from config (if no flag override)
	overrideBranch := ""
	if deployBranchFlag == "" && cfg.Supabase.OverrideBranch != "" {
		overrideBranch = cfg.Supabase.OverrideBranch
	}

	info, err := client.GetBranchInfoWithOverride(targetBranch, overrideBranch)
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

	if info.IsOverride {
		ui.Infof("Override: using %s instead of %s", ui.Cyan(info.SupabaseBranch.Name), ui.Cyan(info.OverrideFrom))
	}

	if info.IsFallback {
		ui.Warning("Using fallback environment")
	}

	// Confirm for production
	confirmed, err := ConfirmProductionOperation(info.Environment, "deploy Edge Functions")
	if err != nil || !confirmed {
		return nil
	}

	ui.NewLine()

	// List functions
	allFunctions, err := supabase.ListFunctions(cfg.GetFunctionsPath())
	if err != nil {
		return err
	}

	// Filter out restricted functions for this environment
	var functions []supabase.Function
	var skippedFunctions []string
	envName := string(info.Environment)

	for _, fn := range allFunctions {
		if cfg.IsFunctionRestricted(fn.Name, envName) {
			skippedFunctions = append(skippedFunctions, fn.Name)
		} else {
			functions = append(functions, fn)
		}
	}

	if len(skippedFunctions) > 0 {
		ui.Warningf("Skipping %d restricted function(s) for %s:", len(skippedFunctions), envName)
		for _, fn := range skippedFunctions {
			ui.List(ui.Dim(fn))
		}
		ui.NewLine()
	}

	ui.Infof("Found %d functions to deploy", len(functions))

	if len(functions) == 0 {
		ui.Info("No functions to deploy")
		return nil
	}

	// Deploy each function
	client := supabase.NewClient()
	opts := supabase.DeployOptions{
		NoVerifyJWT: deployNoVerifyJWT,
	}

	if deployNoVerifyJWT {
		ui.Infof("Deploying with --no-verify-jwt")
	}

	for _, fn := range functions {
		sp := ui.NewSpinner(fmt.Sprintf("Deploying %s", fn.Name))
		sp.Start()

		if err := client.DeployFunctionWithOptions(fn.Name, info.ProjectRef, opts); err != nil {
			sp.Fail(fmt.Sprintf("Failed to deploy %s", fn.Name))
			return err
		}

		sp.Success(fmt.Sprintf("Deployed %s", fn.Name))
	}

	ui.NewLine()
	ui.Success(fmt.Sprintf("Successfully deployed %d functions", len(functions)))

	// Next steps
	ui.NewLine()
	ui.SubHeader("Next Steps")
	ui.List("drift deploy secrets      - Set environment secrets")
	ui.List("drift functions list      - Compare local vs deployed")
	ui.List("drift functions diff      - Verify deployed code")
	ui.List("drift secrets list        - View configured secrets")

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

	if info.IsOverride {
		ui.Infof("Override: using %s instead of %s", ui.Cyan(info.SupabaseBranch.Name), ui.Cyan(info.OverrideFrom))
	}

	// Confirm for production
	confirmed, err := ConfirmProductionOperation(info.Environment, "set secrets")
	if err != nil || !confirmed {
		return nil
	}

	ui.NewLine()

	client := supabase.NewClient()

	// Check for per-environment configuration
	envName := string(info.Environment)
	envConfig := cfg.GetEnvironmentConfig(envName)

	// Determine APNs settings
	apnsEnv := cfg.Apple.PushEnvironment
	pushKeyPattern := cfg.Apple.PushKeyPattern

	if info.Environment == supabase.EnvProduction {
		apnsEnv = "production"
	}

	// Override with per-environment push key if configured
	if envConfig != nil && envConfig.PushKey != "" {
		pushKeyPattern = envConfig.PushKey
		ui.Infof("Using per-environment push key: %s", pushKeyPattern)
	}

	apnsSecrets, err := supabase.LoadAPNSSecretsFromConfig(
		cfg.Apple.TeamID,
		cfg.Apple.BundleID,
		pushKeyPattern,
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

	// Set per-environment secrets if configured
	if envConfig != nil && len(envConfig.Secrets) > 0 {
		sp = ui.NewSpinner(fmt.Sprintf("Setting %d per-environment secrets", len(envConfig.Secrets)))
		sp.Start()

		for key, value := range envConfig.Secrets {
			if err := client.SetSecret(info.ProjectRef, key, value); err != nil {
				sp.Fail(fmt.Sprintf("Failed to set secret %s", key))
				return err
			}
		}

		sp.Success(fmt.Sprintf("Set %d per-environment secrets", len(envConfig.Secrets)))
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

	// Apply override from config
	overrideBranch := cfg.Supabase.OverrideBranch
	client := supabase.NewClient()
	info, err := client.GetBranchInfoWithOverride(gitBranch, overrideBranch)
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

	if info.IsOverride {
		ui.NewLine()
		ui.Infof("Override: using %s instead of %s", ui.Cyan(info.SupabaseBranch.Name), ui.Cyan(info.OverrideFrom))
	}

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

func runDeployListSecrets(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}

	// Get target environment
	sp := ui.NewSpinner("Resolving target environment")
	sp.Start()

	info, err := getDeployTarget()
	if err != nil {
		sp.Fail("Failed to resolve environment")
		return err
	}
	sp.Stop()

	ui.Header("Secrets")
	ui.KeyValue("Environment", envColorString(string(info.Environment)))
	ui.KeyValue("Supabase Branch", ui.Cyan(info.SupabaseBranch.Name))
	ui.KeyValue("Project Ref", ui.Cyan(info.ProjectRef))

	if info.IsOverride {
		ui.Infof("Override: using %s instead of %s", ui.Cyan(info.SupabaseBranch.Name), ui.Cyan(info.OverrideFrom))
	}

	if info.IsFallback {
		ui.Warning("Using fallback environment")
	}

	ui.NewLine()

	// List secrets
	client := supabase.NewClient()
	sp = ui.NewSpinner("Fetching secrets")
	sp.Start()

	secrets, err := client.ListSecrets(info.ProjectRef)
	if err != nil {
		sp.Fail("Failed to list secrets")
		return err
	}
	sp.Stop()

	if len(secrets) == 0 {
		ui.Info("No secrets configured")
		return nil
	}

	ui.SubHeader("Configured Secrets")
	for _, secret := range secrets {
		ui.List(secret)
	}

	ui.NewLine()
	ui.Infof("Total: %d secrets", len(secrets))

	return nil
}

