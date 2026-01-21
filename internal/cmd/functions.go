package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/internal/git"
	"github.com/undrift/drift/internal/supabase"
	"github.com/undrift/drift/internal/ui"
	"github.com/undrift/drift/pkg/shell"
)

var functionsCmd = &cobra.Command{
	Use:   "functions",
	Short: "Manage Edge Functions",
	Long: `View, compare, and manage Supabase Edge Functions.

The functions command provides tools for working with Edge Functions:
  - List deployed vs local functions and their sync status
  - View function logs for debugging
  - Compare local code with deployed versions
  - Delete deployed functions
  - Create new functions from templates
  - Serve functions locally for development

Most commands automatically detect your target environment from your
current git branch, or you can specify a branch with --branch.`,
	Example: `  drift functions list           # Compare local vs deployed functions
  drift functions logs my-func    # View logs for a function
  drift functions diff my-func    # Compare local vs deployed code
  drift functions serve           # Run functions locally
  drift functions delete my-func  # Delete a deployed function`,
}

var functionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List and compare local vs deployed functions",
	Long: `Compare Edge Functions between your local project and the deployed environment.

Shows a comprehensive view of:
  - Functions that exist locally but not deployed (need deploying)
  - Functions deployed but not in local project (orphaned)
  - Functions that exist in both (synced)

The target environment is determined by your current git branch,
or can be overridden with the --branch flag.`,
	Example: `  drift functions list              # List for current branch
  drift functions list --branch dev # List for dev environment`,
	RunE: runFunctionsList,
}

var functionsLogsCmd = &cobra.Command{
	Use:   "logs [function-name]",
	Short: "View Edge Function logs",
	Long: `View logs for a deployed Edge Function.

If no function name is provided, you'll be prompted to select from
the list of deployed functions.

Logs are fetched from the Supabase platform and include:
  - Invocation timestamps
  - Request/response details
  - Console output and errors

Use --output/-o to save logs to a file. If there are 20+ log entries
and no output file is specified, you'll be prompted to optionally
save them to a file.`,
	Example: `  drift functions logs                # Interactive: select function
  drift functions logs send-email     # View logs for send-email
  drift functions logs -b dev my-func # Logs from dev environment
  drift functions logs -o logs.txt fn # Save logs to file`,
	Args: cobra.MaximumNArgs(1),
	RunE: runFunctionsLogs,
}

var functionsDeleteCmd = &cobra.Command{
	Use:   "delete [function-name]",
	Short: "Delete a deployed Edge Function",
	Long: `Delete an Edge Function from the deployed environment.

This removes the function from Supabase but does NOT delete your local files.
Use this to clean up orphaned functions or remove functions you no longer need.

Requires confirmation unless --yes flag is used.
Extra confirmation required for production environments.`,
	Example: `  drift functions delete old-func    # Delete with confirmation
  drift functions delete old-func -y # Skip confirmation
  drift functions delete             # Interactive: select function`,
	Args: cobra.MaximumNArgs(1),
	RunE: runFunctionsDelete,
}

var functionsDiffCmd = &cobra.Command{
	Use:   "diff [function-name]",
	Short: "Compare local vs deployed function code",
	Long: `Compare your local Edge Function code with the deployed version.

Downloads the deployed function source and shows a diff against your
local version. This helps verify that deployments are up-to-date
and identify any discrepancies.

The diff shows:
  - Lines added locally (not yet deployed)
  - Lines removed locally (exist in deployed)
  - Unchanged lines for context

If no function name is provided, you'll be prompted to select from
functions that exist both locally and remotely.`,
	Example: `  drift functions diff my-func       # Diff specific function
  drift functions diff               # Interactive: select function
  drift functions diff -b prod func  # Diff against production`,
	Args: cobra.MaximumNArgs(1),
	RunE: runFunctionsDiff,
}

var functionsServeCmd = &cobra.Command{
	Use:   "serve [function-name]",
	Short: "Run Edge Functions locally for development",
	Long: `Start a local development server for Edge Functions.

This runs the Supabase functions serve command with your project's
environment configuration. The server watches for file changes and
automatically reloads.

If a function name is provided, only that function is served.
Otherwise, all functions are served.

The --env-file flag can specify a custom environment file.
By default, uses .env.local if it exists.`,
	Example: `  drift functions serve              # Serve all functions
  drift functions serve my-func      # Serve specific function
  drift functions serve --env .env   # Use custom env file`,
	Args: cobra.MaximumNArgs(1),
	RunE: runFunctionsServe,
}

var functionsNewCmd = &cobra.Command{
	Use:   "new <function-name>",
	Short: "Create a new Edge Function",
	Long: `Create a new Edge Function from the Supabase template.

Creates a new function directory with the standard structure:
  supabase/functions/<name>/index.ts

The function is created locally and can be deployed with:
  drift deploy functions`,
	Example: `  drift functions new send-email     # Create send-email function
  drift functions new process-webhook`,
	Args: cobra.ExactArgs(1),
	RunE: runFunctionsNew,
}

var (
	functionsBranchFlag string
	functionsEnvFile    string
	functionsLogsOutput string
)

func init() {
	// Add branch flag to relevant commands
	functionsListCmd.Flags().StringVarP(&functionsBranchFlag, "branch", "b", "", "Target Supabase branch (default: current git branch)")
	functionsLogsCmd.Flags().StringVarP(&functionsBranchFlag, "branch", "b", "", "Target Supabase branch")
	functionsDeleteCmd.Flags().StringVarP(&functionsBranchFlag, "branch", "b", "", "Target Supabase branch")
	functionsDiffCmd.Flags().StringVarP(&functionsBranchFlag, "branch", "b", "", "Target Supabase branch")

	// Env file for serve
	functionsServeCmd.Flags().StringVar(&functionsEnvFile, "env", "", "Path to environment file (default: .env.local)")

	// Output file for logs
	functionsLogsCmd.Flags().StringVarP(&functionsLogsOutput, "output", "o", "", "Save logs to file instead of displaying")

	functionsCmd.AddCommand(functionsListCmd)
	functionsCmd.AddCommand(functionsLogsCmd)
	functionsCmd.AddCommand(functionsDeleteCmd)
	functionsCmd.AddCommand(functionsDiffCmd)
	functionsCmd.AddCommand(functionsServeCmd)
	functionsCmd.AddCommand(functionsNewCmd)
	rootCmd.AddCommand(functionsCmd)
}

// getFunctionsTarget resolves the target environment for functions commands.
func getFunctionsTarget() (*supabase.BranchInfo, error) {
	cfg := config.LoadOrDefault()
	client := supabase.NewClient()

	targetBranch := functionsBranchFlag
	if targetBranch == "" {
		var err error
		targetBranch, err = git.CurrentBranch()
		if err != nil {
			return nil, fmt.Errorf("failed to get current branch: %w", err)
		}
	}

	overrideBranch := ""
	if functionsBranchFlag == "" && cfg.Supabase.OverrideBranch != "" {
		overrideBranch = cfg.Supabase.OverrideBranch
	}

	info, err := client.GetBranchInfoWithOverride(targetBranch, overrideBranch)
	if err != nil {
		return nil, err
	}

	return info, nil
}

func runFunctionsList(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()

	// Resolve target environment
	sp := ui.NewSpinner("Resolving target environment")
	sp.Start()

	info, err := getFunctionsTarget()
	if err != nil {
		sp.Fail("Failed to resolve environment")
		return err
	}
	sp.Stop()

	ui.Header("Edge Functions")
	ui.KeyValue("Environment", envColorString(string(info.Environment)))
	ui.KeyValue("Project Ref", ui.Cyan(info.ProjectRef))
	if info.IsOverride {
		ui.Infof("Override: using %s instead of %s", ui.Cyan(info.SupabaseBranch.Name), ui.Cyan(info.OverrideFrom))
	}
	ui.NewLine()

	// Get local functions
	localFunctions, err := supabase.ListFunctions(cfg.GetFunctionsPath())
	localFuncMap := make(map[string]bool)
	if err != nil {
		if IsVerbose() {
			ui.Warningf("Could not list local functions: %v", err)
		}
	} else {
		for _, fn := range localFunctions {
			localFuncMap[fn.Name] = true
		}
	}

	// Get deployed functions
	sp = ui.NewSpinner("Fetching deployed functions")
	sp.Start()

	client := supabase.NewClient()
	deployedFunctions, err := client.ListDeployedFunctions(info.ProjectRef)
	sp.Stop()

	deployedFuncMap := make(map[string]supabase.DeployedFunction)
	if err != nil {
		ui.Warningf("Could not list deployed functions: %v", err)
	} else {
		for _, fn := range deployedFunctions {
			deployedFuncMap[fn.Name] = fn
		}
	}

	// Combine into unified list
	allFuncs := make(map[string]bool)
	for name := range localFuncMap {
		allFuncs[name] = true
	}
	for name := range deployedFuncMap {
		allFuncs[name] = true
	}

	if len(allFuncs) == 0 {
		ui.Info("No functions found locally or deployed")
		ui.NewLine()
		ui.SubHeader("Next Steps")
		ui.List("drift functions new <name>  - Create a new function")
		return nil
	}

	// Sort function names
	var funcNames []string
	for name := range allFuncs {
		funcNames = append(funcNames, name)
	}
	sort.Strings(funcNames)

	// Display comparison
	ui.SubHeader("Function Status")
	ui.NewLine()

	var needsDeploy, orphaned, synced []string

	for _, name := range funcNames {
		isLocal := localFuncMap[name]
		_, isDeployed := deployedFuncMap[name]

		var status string
		if isLocal && isDeployed {
			status = ui.Green("synced")
			synced = append(synced, name)
		} else if isLocal && !isDeployed {
			status = ui.Yellow("local only")
			needsDeploy = append(needsDeploy, name)
		} else {
			status = ui.Red("deployed only")
			orphaned = append(orphaned, name)
		}

		fmt.Printf("  %-30s %s\n", name, status)
	}

	// Summary
	ui.NewLine()
	ui.SubHeader("Summary")
	ui.KeyValue("Total", fmt.Sprintf("%d functions", len(allFuncs)))
	if len(synced) > 0 {
		ui.KeyValue("Synced", ui.Green(fmt.Sprintf("%d", len(synced))))
	}
	if len(needsDeploy) > 0 {
		ui.KeyValue("Need Deploy", ui.Yellow(fmt.Sprintf("%d", len(needsDeploy))))
	}
	if len(orphaned) > 0 {
		ui.KeyValue("Orphaned", ui.Red(fmt.Sprintf("%d", len(orphaned))))
	}

	// Next steps
	ui.NewLine()
	ui.SubHeader("Next Steps")
	if len(needsDeploy) > 0 {
		ui.List("drift deploy functions        - Deploy local functions")
	}
	if len(orphaned) > 0 {
		ui.List("drift functions delete <name> - Remove orphaned functions")
	}
	if len(synced) > 0 {
		ui.List("drift functions diff <name>   - Verify deployed code matches local")
	}
	ui.List("drift functions logs <name>   - View function logs")

	return nil
}

func runFunctionsLogs(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}

	// Resolve target
	sp := ui.NewSpinner("Resolving target environment")
	sp.Start()

	info, err := getFunctionsTarget()
	if err != nil {
		sp.Fail("Failed to resolve environment")
		return err
	}
	sp.Stop()

	var functionName string

	if len(args) > 0 {
		functionName = args[0]
	} else {
		// Interactive: select from deployed functions
		client := supabase.NewClient()
		sp = ui.NewSpinner("Fetching deployed functions")
		sp.Start()

		deployedFunctions, err := client.ListDeployedFunctions(info.ProjectRef)
		sp.Stop()

		if err != nil {
			return fmt.Errorf("failed to list deployed functions: %w", err)
		}

		if len(deployedFunctions) == 0 {
			ui.Info("No deployed functions found")
			return nil
		}

		options := make([]string, len(deployedFunctions))
		for i, fn := range deployedFunctions {
			options[i] = fn.Name
		}

		selected, err := ui.PromptSelect("Select function to view logs", options)
		if err != nil {
			return err
		}
		functionName = selected
	}

	ui.Header("Function Logs")
	ui.KeyValue("Function", ui.Cyan(functionName))
	ui.KeyValue("Environment", envColorString(string(info.Environment)))
	ui.KeyValue("Project Ref", ui.Cyan(info.ProjectRef))
	ui.NewLine()

	// Fetch logs
	sp = ui.NewSpinner("Fetching logs")
	sp.Start()

	client := supabase.NewClient()
	logs, err := client.GetFunctionLogs(functionName, info.ProjectRef)
	sp.Stop()

	if err != nil {
		return fmt.Errorf("failed to get logs: %w", err)
	}

	if len(logs) == 0 {
		ui.Info("No logs found in the last hour")
		ui.NewLine()
		ui.Dim("Tip: Invoke the function to generate logs, then try again")
	} else {
		// Check if we should save to file
		outputFile := functionsLogsOutput

		// If many logs and no output specified, offer to save to file
		if outputFile == "" && len(logs) >= 20 {
			ui.Infof("Found %d log entries", len(logs))
			saveToFile, err := ui.PromptYesNo("Save logs to file?", false)
			if err == nil && saveToFile {
				// Suggest a default filename
				defaultName := fmt.Sprintf("%s-logs-%s.txt", functionName, time.Now().Format("2006-01-02-150405"))
				outputFile, err = ui.PromptString("Filename", defaultName)
				if err != nil {
					outputFile = defaultName
				}
			}
		}

		if outputFile != "" {
			// Save to file
			if err := saveLogsToFile(logs, outputFile, functionName, info); err != nil {
				return fmt.Errorf("failed to save logs: %w", err)
			}
			ui.Successf("Saved %d log entries to %s", len(logs), outputFile)
		} else {
			// Display logs in terminal
			ui.Infof("Showing %d log entries (last hour):", len(logs))
			ui.NewLine()
			for _, entry := range logs {
				// Format timestamp
				timestamp := entry.Timestamp
				if t, err := time.Parse(time.RFC3339Nano, entry.Timestamp); err == nil {
					timestamp = t.Local().Format("15:04:05")
				}

				// Color based on level
				level := entry.Level
				switch strings.ToLower(level) {
				case "error":
					level = ui.Red(level)
				case "warning", "warn":
					level = ui.Yellow(level)
				case "info":
					level = ui.Cyan(level)
				default:
					level = ui.Dim(level)
				}

				fmt.Printf("%s [%s] %s\n", ui.Dim(timestamp), level, entry.EventMessage)
			}
		}
	}

	// Next steps
	ui.NewLine()
	ui.SubHeader("Next Steps")
	ui.List("drift functions list          - See all functions")
	ui.List("drift functions diff <name>   - Compare local vs deployed")
	ui.List("drift deploy functions        - Redeploy functions")

	return nil
}

// saveLogsToFile writes function logs to a file in a readable format.
func saveLogsToFile(logs []supabase.FunctionLogEntry, filename, functionName string, info *supabase.BranchInfo) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Write header
	fmt.Fprintf(file, "Function Logs: %s\n", functionName)
	fmt.Fprintf(file, "Environment: %s\n", info.Environment)
	fmt.Fprintf(file, "Project Ref: %s\n", info.ProjectRef)
	fmt.Fprintf(file, "Generated: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "Entries: %d\n", len(logs))
	fmt.Fprintf(file, "\n%s\n\n", strings.Repeat("-", 80))

	// Write log entries
	for _, entry := range logs {
		// Format timestamp
		timestamp := entry.Timestamp
		if t, err := time.Parse(time.RFC3339Nano, entry.Timestamp); err == nil {
			timestamp = t.Local().Format("2006-01-02 15:04:05.000")
		}

		fmt.Fprintf(file, "%s [%s] %s\n", timestamp, entry.Level, entry.EventMessage)
	}

	return nil
}

func runFunctionsDelete(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}

	// Resolve target
	sp := ui.NewSpinner("Resolving target environment")
	sp.Start()

	info, err := getFunctionsTarget()
	if err != nil {
		sp.Fail("Failed to resolve environment")
		return err
	}
	sp.Stop()

	var functionName string

	if len(args) > 0 {
		functionName = args[0]
	} else {
		// Interactive: select from deployed functions
		client := supabase.NewClient()
		sp = ui.NewSpinner("Fetching deployed functions")
		sp.Start()

		deployedFunctions, err := client.ListDeployedFunctions(info.ProjectRef)
		sp.Stop()

		if err != nil {
			return fmt.Errorf("failed to list deployed functions: %w", err)
		}

		if len(deployedFunctions) == 0 {
			ui.Info("No deployed functions found")
			return nil
		}

		options := make([]string, len(deployedFunctions))
		for i, fn := range deployedFunctions {
			options[i] = fn.Name
		}

		selected, err := ui.PromptSelect("Select function to delete", options)
		if err != nil {
			return err
		}
		functionName = selected
	}

	ui.Header("Delete Function")
	ui.KeyValue("Function", ui.Red(functionName))
	ui.KeyValue("Environment", envColorString(string(info.Environment)))
	ui.KeyValue("Project Ref", ui.Cyan(info.ProjectRef))
	ui.NewLine()

	// Confirm deletion
	if !IsYes() {
		ui.Warning("This will delete the deployed function from Supabase.")
		ui.Info("Your local files will NOT be deleted.")
		ui.NewLine()

		// Extra warning for production
		if info.Environment == supabase.EnvProduction {
			ui.Error("WARNING: You are about to delete from PRODUCTION!")
			ui.NewLine()
		}

		confirmed, err := ui.PromptYesNo(fmt.Sprintf("Delete %s?", functionName), false)
		if err != nil || !confirmed {
			ui.Info("Cancelled")
			return nil
		}
	}

	// Delete the function
	sp = ui.NewSpinner(fmt.Sprintf("Deleting %s", functionName))
	sp.Start()

	client := supabase.NewClient()
	if err := client.DeleteFunction(functionName, info.ProjectRef); err != nil {
		sp.Fail("Failed to delete function")
		return err
	}
	sp.Success(fmt.Sprintf("Deleted %s", functionName))

	// Next steps
	ui.NewLine()
	ui.SubHeader("Next Steps")
	ui.List("drift functions list          - Verify deletion")
	ui.List("drift deploy functions        - Redeploy if needed")

	return nil
}

func runFunctionsDiff(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()

	// Resolve target
	sp := ui.NewSpinner("Resolving target environment")
	sp.Start()

	info, err := getFunctionsTarget()
	if err != nil {
		sp.Fail("Failed to resolve environment")
		return err
	}
	sp.Stop()

	var functionName string

	if len(args) > 0 {
		functionName = args[0]
	} else {
		// Interactive: find functions that exist both locally and remotely
		localFunctions, err := supabase.ListFunctions(cfg.GetFunctionsPath())
		if err != nil {
			return fmt.Errorf("failed to list local functions: %w", err)
		}

		client := supabase.NewClient()
		sp = ui.NewSpinner("Fetching deployed functions")
		sp.Start()

		deployedFunctions, err := client.ListDeployedFunctions(info.ProjectRef)
		sp.Stop()

		if err != nil {
			return fmt.Errorf("failed to list deployed functions: %w", err)
		}

		// Find intersection
		deployedMap := make(map[string]bool)
		for _, fn := range deployedFunctions {
			deployedMap[fn.Name] = true
		}

		var common []string
		for _, fn := range localFunctions {
			if deployedMap[fn.Name] {
				common = append(common, fn.Name)
			}
		}

		if len(common) == 0 {
			ui.Info("No functions exist both locally and deployed")
			ui.NewLine()
			ui.SubHeader("Next Steps")
			ui.List("drift functions list      - See all functions")
			ui.List("drift deploy functions    - Deploy local functions")
			return nil
		}

		selected, err := ui.PromptSelect("Select function to diff", common)
		if err != nil {
			return err
		}
		functionName = selected
	}

	ui.Header("Function Diff")
	ui.KeyValue("Function", ui.Cyan(functionName))
	ui.KeyValue("Environment", envColorString(string(info.Environment)))
	ui.KeyValue("Project Ref", ui.Cyan(info.ProjectRef))
	ui.NewLine()

	// Check local function exists
	localPath := filepath.Join(cfg.GetFunctionsPath(), functionName, "index.ts")
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		ui.Errorf("Local function not found: %s", localPath)
		return nil
	}

	// Download deployed function
	sp = ui.NewSpinner("Downloading deployed function")
	sp.Start()

	client := supabase.NewClient()
	tempDir, err := client.DownloadFunctionToTemp(functionName, info.ProjectRef)
	if err != nil {
		sp.Fail("Failed to download function")
		return err
	}
	defer os.RemoveAll(tempDir)
	sp.Stop()

	// Try different possible paths where the function might be downloaded
	possiblePaths := []string{
		filepath.Join(tempDir, "supabase", "functions", functionName, "index.ts"),
		filepath.Join(tempDir, functionName, "index.ts"),
		filepath.Join(tempDir, "index.ts"),
	}

	var remotePath string
	for _, p := range possiblePaths {
		if _, err := os.Stat(p); err == nil {
			remotePath = p
			break
		}
	}

	if remotePath == "" {
		// Debug: list what's in temp dir
		if IsVerbose() {
			ui.Warningf("Downloaded function has unexpected structure in %s", tempDir)
		}
		return fmt.Errorf("could not find index.ts in downloaded function")
	}

	// Run diff
	ui.SubHeader("Differences (local vs deployed)")
	ui.NewLine()

	result, _ := shell.Run("diff", "--color=always", "-u", remotePath, localPath)

	if result.Stdout == "" && result.ExitCode == 0 {
		ui.Success("No differences found - local matches deployed!")
	} else if result.Stdout != "" {
		fmt.Println(result.Stdout)
		ui.NewLine()
		ui.Info("Legend: Lines starting with + are in local, - are in deployed")
	} else if result.ExitCode != 0 && result.Stderr != "" {
		ui.Warningf("Diff error: %s", result.Stderr)
	}

	// Next steps
	ui.NewLine()
	ui.SubHeader("Next Steps")
	ui.List("drift deploy functions        - Deploy changes")
	ui.List("drift functions list          - See all function statuses")

	return nil
}

func runFunctionsServe(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()

	var functionName string
	if len(args) > 0 {
		functionName = args[0]
	}

	// Determine env file
	envFile := functionsEnvFile
	if envFile == "" {
		// Check for default env files
		candidates := []string{".env.local", ".env"}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				envFile = candidate
				break
			}
		}
	}

	ui.Header("Serve Edge Functions")
	if functionName != "" {
		ui.KeyValue("Function", ui.Cyan(functionName))
	} else {
		ui.KeyValue("Functions", ui.Cyan("all"))
	}
	if envFile != "" {
		ui.KeyValue("Env File", ui.Cyan(envFile))
	}
	ui.KeyValue("Functions Path", ui.Cyan(cfg.GetFunctionsPath()))
	ui.NewLine()

	ui.Info("Starting local function server...")
	ui.Info("Press Ctrl+C to stop")
	ui.NewLine()

	// Run serve command
	client := supabase.NewClient()
	if err := client.ServeFunction(functionName, envFile); err != nil {
		return fmt.Errorf("failed to serve functions: %w", err)
	}

	return nil
}

func runFunctionsNew(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()

	functionName := args[0]

	// Validate name
	if strings.ContainsAny(functionName, " /\\") {
		return fmt.Errorf("function name cannot contain spaces or slashes")
	}

	// Check if function already exists
	funcPath := filepath.Join(cfg.GetFunctionsPath(), functionName)
	if _, err := os.Stat(funcPath); err == nil {
		ui.Errorf("Function already exists: %s", funcPath)
		return nil
	}

	ui.Header("Create New Function")
	ui.KeyValue("Name", ui.Cyan(functionName))
	ui.KeyValue("Path", ui.Cyan(funcPath))
	ui.NewLine()

	sp := ui.NewSpinner(fmt.Sprintf("Creating %s", functionName))
	sp.Start()

	client := supabase.NewClient()
	if err := client.NewFunction(functionName); err != nil {
		sp.Fail("Failed to create function")
		return err
	}
	sp.Success(fmt.Sprintf("Created %s", functionName))

	// Next steps
	ui.NewLine()
	ui.SubHeader("Next Steps")
	ui.List(fmt.Sprintf("Edit your function: %s/index.ts", funcPath))
	ui.List("drift functions serve         - Test locally")
	ui.List("drift deploy functions        - Deploy to Supabase")
	ui.List("drift functions list          - See all functions")

	return nil
}
