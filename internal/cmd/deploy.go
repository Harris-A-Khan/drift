package cmd

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

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
If no match exists, Drift uses fallback resolution in this order:
  1) --fallback-branch flag
  2) supabase.fallback_branch from .drift.local.yaml
  3) interactive non-production branch selection

Commands:
  functions    - Deploy all Edge Functions
  secrets      - Set configured environment secrets
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
  drift deploy functions --fallback-branch development
  drift deploy functions --no-verify-jwt  # Skip JWT verification`,
	RunE: runDeployFunctions,
}

var deploySecretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Set environment secrets",
	Long: `Set environment secrets for Edge Functions.

This configures secrets that are available to Edge Functions at runtime:
  - APNs credentials (for push notifications)
  - Environment-specific secrets from merged config

If supabase.secrets_to_push is configured in .drift.yaml, only those
keys are pushed.
Baseline values can be set in supabase.default_secrets.
Values are typically defined in .drift.local.yaml under environments.<env>.secrets.
Use environments.<env>.skip_secrets to avoid pushing keys on specific environments.`,
	Example: `  drift deploy secrets           # Set secrets for current environment
  drift deploy secrets -b feature/x   # Set secrets for a specific branch
  drift deploy secrets --key-search-dir ../shared-keys`,
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
  drift deploy all -b feature/my-branch   # Deploy to specific non-production branch`,
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
	deployBranchFlag    string
	deployNoVerifyJWT   bool
	deployKeySearchDirs []string
)

func init() {
	// Add branch flag to all deploy commands
	deployFunctionsCmd.Flags().StringVarP(&deployBranchFlag, "branch", "b", "", "Target Supabase branch")
	deploySecretsCmd.Flags().StringVarP(&deployBranchFlag, "branch", "b", "", "Target Supabase branch")
	deployAllCmd.Flags().StringVarP(&deployBranchFlag, "branch", "b", "", "Target Supabase branch")
	deployListSecretsCmd.Flags().StringVarP(&deployBranchFlag, "branch", "b", "", "Target Supabase branch")
	deploySecretsCmd.Flags().StringSliceVar(&deployKeySearchDirs, "key-search-dir", nil, "Directory to search for APNs key files (can be repeated; overrides configured search paths)")
	deployAllCmd.Flags().StringSliceVar(&deployKeySearchDirs, "key-search-dir", nil, "Directory to search for APNs key files (can be repeated; overrides configured search paths)")

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

	currentBranch, err := git.CurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	info, err := ResolveSupabaseTargetForCurrentBranch(client, cfg, currentBranch, deployBranchFlag)
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
		ui.Warningf("Using fallback target branch: %s", info.SupabaseBranch.GitBranch)
	}

	// Confirm for protected/development environments
	confirmed, err := ConfirmDeploymentOperation(info, cfg, "deploy Edge Functions")
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
	if info.IsFallback {
		ui.Warningf("Using fallback target branch: %s", info.SupabaseBranch.GitBranch)
	}

	// Confirm for protected/development environments
	confirmed, err := ConfirmDeploymentOperation(info, cfg, "set secrets")
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

	searchPaths := cfg.Apple.KeySearchPaths
	if len(deployKeySearchDirs) > 0 {
		searchPaths = deployKeySearchDirs
	}
	if len(searchPaths) == 0 {
		searchPaths = []string{cfg.Apple.SecretsDir, ".", ".."}
	}

	ui.SubHeader("APNs Key Search")
	for _, path := range resolveSearchDirs(cfg.ProjectRoot(), searchPaths) {
		ui.List(path)
	}
	ui.NewLine()

	apnsSecrets, apnsLookup, err := supabase.LoadAPNSSecretsFromConfigWithSearchPaths(
		cfg.Apple.TeamID,
		cfg.Apple.BundleID,
		pushKeyPattern,
		apnsEnv,
		cfg.ProjectRoot(),
		cfg.Apple.SecretsDir,
		searchPaths,
	)

	availableSecrets := make(map[string]string)
	for key, value := range cfg.Supabase.DefaultSecrets {
		availableSecrets[key] = value
	}

	if err != nil {
		ui.Warning(fmt.Sprintf("Could not load APNs secrets: %v", err))
		ui.Info("Skipping APNs-derived secrets")
	} else {
		if apnsLookup != nil && apnsLookup.MatchedFile != "" {
			ui.KeyValue("Matched Key File", apnsLookup.MatchedFile)
		}
		availableSecrets["APNS_KEY_ID"] = apnsSecrets.KeyID
		availableSecrets["APNS_TEAM_ID"] = apnsSecrets.TeamID
		availableSecrets["APNS_BUNDLE_ID"] = apnsSecrets.BundleID
		availableSecrets["APNS_PRIVATE_KEY"] = apnsSecrets.PrivateKey
		availableSecrets["APNS_ENVIRONMENT"] = apnsSecrets.Environment
	}

	// Add per-environment secrets
	if envConfig != nil && len(envConfig.Secrets) > 0 {
		for key, value := range envConfig.Secrets {
			availableSecrets[key] = value
		}
	}

	skippedByPolicy := make(map[string]bool)
	if envConfig != nil && len(envConfig.SkipSecrets) > 0 {
		for _, key := range envConfig.SkipSecrets {
			if key == "" {
				continue
			}
			skippedByPolicy[key] = true
			delete(availableSecrets, key)
		}
	}

	if len(skippedByPolicy) > 0 {
		ui.Infof("Skipping secrets by environment policy: %s", stringsJoinSorted(secretSetKeys(skippedByPolicy)))
	}

	secretsToPush, missingConfigured, configuredSkipped := selectSecretsToPush(cfg.Supabase.SecretsToPush, availableSecrets, skippedByPolicy)
	if IsVerbose() {
		if len(cfg.Supabase.SecretsToPush) > 0 {
			ui.Infof("Configured supabase.secrets_to_push: %s", stringsJoinSorted(cfg.Supabase.SecretsToPush))
		} else {
			ui.Info("No supabase.secrets_to_push configured; pushing all discovered secrets")
		}
		if len(cfg.Supabase.DefaultSecrets) > 0 {
			ui.Infof("Configured supabase.default_secrets: %s", stringsJoinSorted(secretMapKeys(cfg.Supabase.DefaultSecrets)))
		}
		ui.Infof("Discovered %d secret candidate(s): %s", len(availableSecrets), stringsJoinSorted(secretMapKeys(availableSecrets)))
	}
	if len(configuredSkipped) > 0 {
		ui.Infof("Configured secrets skipped by policy: %s", stringsJoinSorted(configuredSkipped))
	}
	if len(missingConfigured) > 0 {
		ui.Warningf("Configured secrets not available for this run: %s", stringsJoinSorted(missingConfigured))
	}

	if len(secretsToPush) == 0 {
		ui.Warning("No secrets selected to push")
		ui.Info("Update supabase.secrets_to_push/default_secrets or environment secrets in .drift.yaml/.drift.local.yaml")
		return nil
	}

	ui.Infof("Pushing %d secret(s): %s", len(secretsToPush), stringsJoinSecretNames(secretsToPush))
	sp = ui.NewSpinner(fmt.Sprintf("Setting %d secret(s)", len(secretsToPush)))
	sp.Start()

	if err := client.SetSecrets(info.ProjectRef, secretsToPush); err != nil {
		sp.Fail("Failed to set secrets")
		return err
	}
	sp.Success(fmt.Sprintf("Set %d secret(s)", len(secretsToPush)))

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
	info, err := ResolveSupabaseTargetForCurrentBranch(client, cfg, gitBranch, "")
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
		ui.Warningf("Using fallback target branch: %s", info.SupabaseBranch.GitBranch)
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
		ui.Warningf("Using fallback target branch: %s", info.SupabaseBranch.GitBranch)
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

func resolveSearchDirs(projectRoot string, rawPaths []string) []string {
	seen := make(map[string]bool)
	var dirs []string
	for _, p := range rawPaths {
		if p == "" {
			continue
		}
		resolved := p
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(projectRoot, resolved)
		}
		resolved = filepath.Clean(resolved)
		if seen[resolved] {
			continue
		}
		seen[resolved] = true
		dirs = append(dirs, resolved)
	}
	return dirs
}

func selectSecretsToPush(configured []string, available map[string]string, skipped map[string]bool) ([]supabase.Secret, []string, []string) {
	if len(available) == 0 {
		return nil, nil, nil
	}

	seenConfigured := make(map[string]bool)
	var selected []string
	var missing []string
	var skippedConfigured []string

	if len(configured) > 0 {
		for _, key := range configured {
			if key == "" || seenConfigured[key] {
				continue
			}
			seenConfigured[key] = true
			if skipped[key] {
				skippedConfigured = append(skippedConfigured, key)
				continue
			}
			if _, ok := available[key]; ok {
				selected = append(selected, key)
			} else {
				missing = append(missing, key)
			}
		}
	} else {
		for key := range available {
			selected = append(selected, key)
		}
	}

	sort.Strings(selected)
	sort.Strings(missing)
	sort.Strings(skippedConfigured)

	secrets := make([]supabase.Secret, 0, len(selected))
	for _, key := range selected {
		secrets = append(secrets, supabase.Secret{Name: key, Value: available[key]})
	}

	return secrets, missing, skippedConfigured
}

func stringsJoinSecretNames(secrets []supabase.Secret) string {
	names := make([]string, len(secrets))
	for i, s := range secrets {
		names[i] = s.Name
	}
	sort.Strings(names)
	return stringsJoinSorted(names)
}

func stringsJoinSorted(values []string) string {
	if len(values) == 0 {
		return ""
	}
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return strings.Join(sorted, ", ")
}

func secretMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func secretSetKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
