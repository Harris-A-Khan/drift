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

var worktreeCmd = &cobra.Command{
	Use:     "worktree",
	Aliases: []string{"wt"},
	Short:   "Git worktree management",
	Long: `Manage git worktrees for parallel development.

Worktrees allow you to have multiple branches checked out simultaneously
in different directories. This is useful for:
- Working on multiple features in parallel
- Quick context switching without stashing
- Testing changes in isolation`,
}

var wtListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all worktrees",
	Long:  `List all git worktrees with formatted, color-coded output.`,
	RunE:  runWorktreeList,
}

var wtCreateCmd = &cobra.Command{
	Use:   "create [branch]",
	Short: "Create a new worktree with full setup",
	Long: `Create a new worktree for a branch with full setup.

By default, this command:
- Creates the git worktree
- Copies configured files (.env, .p8 keys, etc.)
- Generates environment config (.env.local for web, Config.xcconfig for iOS)

If no branch is specified, interactive mode will prompt you to enter a
branch name and select the base branch.

The branch can be:
- An existing local branch
- An existing remote branch (will create a tracking branch)
- A new branch name (will create from the selected base)`,
	Example: `  drift worktree create
  drift worktree create feat/my-feature
  drift worktree create feat/my-feature --open
  drift worktree create fix/bug-123 --from main
  drift worktree create feat/quick-test --no-setup`,
	Args: cobra.MaximumNArgs(1),
	RunE: runWorktreeCreate,
}


var wtOpenCmd = &cobra.Command{
	Use:   "open [branch]",
	Short: "Open worktree in VS Code",
	Long:  `Open a worktree in VS Code. If no branch is specified, shows an interactive picker.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runWorktreeOpen,
}

var wtDeleteCmd = &cobra.Command{
	Use:   "delete [branch]",
	Short: "Delete a worktree",
	Long:  `Delete a worktree. If no branch is specified, shows an interactive picker.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runWorktreeDelete,
}

var wtPathCmd = &cobra.Command{
	Use:   "path <branch>",
	Short: "Print absolute path to worktree",
	Long:  `Print the absolute path to a worktree directory.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runWorktreePath,
}

var wtPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Clean stale worktree entries",
	Long:  `Remove stale worktree entries for worktrees that no longer exist on disk.`,
	RunE:  runWorktreePrune,
}

var wtInfoCmd = &cobra.Command{
	Use:   "info [branch]",
	Short: "Show detailed worktree info",
	Long: `Show detailed information about a worktree, including:
- Commits ahead/behind origin
- Uncommitted changes count
- Supabase branch mapping
- Environment (production/development/feature)

If no branch is specified, shows info for the current worktree.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runWorktreeInfo,
}

var wtCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up merged worktrees",
	Long: `Find and delete worktrees for branches that have been merged into main.

This command:
1. Detects branches merged into main/master
2. Shows which worktrees can be safely deleted
3. Prompts for confirmation before each deletion
4. Optionally deletes the remote branch as well`,
	RunE: runWorktreeCleanup,
}

var wtSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync worktrees with remote",
	Long: `Interactively select worktrees to sync with their remote branches.

Shows all worktrees with checkboxes for selection.
For each selected worktree, pulls the latest changes from origin.`,
	RunE: runWorktreeSync,
}

var (
	wtFromFlag    string
	wtOpenFlag    bool
	wtForceFlag   bool
	wtFinderFlag  bool
	wtTermFlag    bool
	wtNoSetupFlag bool
)

func init() {
	// Create flags
	wtCreateCmd.Flags().StringVar(&wtFromFlag, "from", "development", "Base branch for new branches")
	wtCreateCmd.Flags().BoolVar(&wtOpenFlag, "open", false, "Open in VS Code after setup")
	wtCreateCmd.Flags().BoolVar(&wtNoSetupFlag, "no-setup", false, "Skip file copying and environment setup")

	// Open flags
	wtOpenCmd.Flags().BoolVar(&wtFinderFlag, "finder", false, "Open in Finder instead of VS Code")
	wtOpenCmd.Flags().BoolVar(&wtTermFlag, "terminal", false, "Open in Terminal instead of VS Code")

	// Delete flags
	wtDeleteCmd.Flags().BoolVarP(&wtForceFlag, "force", "f", false, "Force delete even with uncommitted changes")

	worktreeCmd.AddCommand(wtListCmd)
	worktreeCmd.AddCommand(wtCreateCmd)
	worktreeCmd.AddCommand(wtOpenCmd)
	worktreeCmd.AddCommand(wtDeleteCmd)
	worktreeCmd.AddCommand(wtPathCmd)
	worktreeCmd.AddCommand(wtPruneCmd)
	worktreeCmd.AddCommand(wtInfoCmd)
	worktreeCmd.AddCommand(wtCleanupCmd)
	worktreeCmd.AddCommand(wtSyncCmd)
	rootCmd.AddCommand(worktreeCmd)
}

func runWorktreeList(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	worktrees, err := git.ListWorktrees()
	if err != nil {
		return err
	}

	if len(worktrees) == 0 {
		ui.Info("No worktrees found")
		return nil
	}

	ui.Header("Git Worktrees")

	for _, wt := range worktrees {
		branchDisplay := wt.Branch
		if branchDisplay == "" {
			branchDisplay = "(bare)"
		}

		// Color based on branch type
		branchColored := branchDisplay
		switch {
		case branchDisplay == "main" || branchDisplay == "master":
			branchColored = ui.Red(branchDisplay)
		case branchDisplay == "development":
			branchColored = ui.Yellow(branchDisplay)
		case strings.HasPrefix(branchDisplay, "feat") || strings.HasPrefix(branchDisplay, "feature"):
			branchColored = ui.Green(branchDisplay)
		case strings.HasPrefix(branchDisplay, "fix") || strings.HasPrefix(branchDisplay, "bugfix"):
			branchColored = ui.Cyan(branchDisplay)
		default:
			branchColored = ui.Blue(branchDisplay)
		}

		// Mark current worktree
		current := ""
		if wt.IsCurrent {
			current = ui.Bold(" ← you are here")
		}

		// Locked indicator
		locked := ""
		if wt.IsLocked {
			locked = ui.Yellow(" [locked]")
		}

		// Status indicators
		statusIndicators := ""

		// Uncommitted changes indicator
		if !wt.IsBare {
			changes, err := git.GetUncommittedChanges(wt.Path)
			if err == nil && changes > 0 {
				statusIndicators += ui.Yellow(" ●")
			}
		}

		// Ahead/behind indicator (skip for current to avoid slowdown)
		if !wt.IsCurrent && !wt.IsBare && wt.Branch != "" {
			ahead, behind, err := git.GetAheadBehind(wt.Path, wt.Branch)
			if err == nil {
				if ahead > 0 {
					statusIndicators += ui.Green(fmt.Sprintf(" ↑%d", ahead))
				}
				if behind > 0 {
					statusIndicators += ui.Red(fmt.Sprintf(" ↓%d", behind))
				}
			}
		}

		fmt.Printf("  %s%s %s%s%s\n", branchColored, statusIndicators, ui.Dim(wt.Path), locked, current)
	}

	ui.NewLine()
	ui.Infof("Total: %d worktrees", len(worktrees))

	// Show legend
	ui.NewLine()
	ui.Info("Legend: " + ui.Yellow("●") + " uncommitted  " + ui.Green("↑") + " ahead  " + ui.Red("↓") + " behind")

	return nil
}

func runWorktreeCreate(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}
	cfg := config.LoadOrDefault()

	var branch string
	if len(args) == 1 {
		branch = args[0]
	} else {
		// Interactive mode - prompt for branch name and base
		selectedBranch, err := createNewBranchInteractive(cfg)
		if err != nil {
			return err
		}
		branch = selectedBranch
	}

	// Check if worktree already exists
	worktreeExists := git.WorktreeExists(branch)
	if worktreeExists && wtNoSetupFlag {
		return fmt.Errorf("worktree for branch '%s' already exists", branch)
	}

	var wtPath string

	if !worktreeExists {
		// Determine worktree path
		wtPath = git.GetWorktreePath(cfg.Project.Name, branch, cfg.Worktree.NamingPattern)

		ui.Infof("Creating worktree for branch '%s'", branch)
		ui.KeyValue("Path", wtPath)

		// Check if branch exists locally
		if git.BranchExists(branch) {
			ui.Info("Using existing local branch")
			if err := git.CreateWorktree(wtPath, branch, false, ""); err != nil {
				return err
			}
		} else if git.RemoteBranchExists("origin", branch) {
			// Branch exists on remote, create tracking branch
			ui.Info("Creating from remote branch")
			if err := git.CreateWorktreeFromRemote(wtPath, branch, branch); err != nil {
				return err
			}
		} else {
			// Create new branch from base
			ui.Infof("Creating new branch from %s", wtFromFlag)

			// First ensure we have the latest from remote
			_ = git.Fetch("origin")

			baseBranch := "origin/" + wtFromFlag
			if !git.RemoteBranchExists("origin", wtFromFlag) {
				if git.BranchExists(wtFromFlag) {
					baseBranch = wtFromFlag
				} else {
					return fmt.Errorf("base branch '%s' not found", wtFromFlag)
				}
			}

			if err := git.CreateWorktree(wtPath, branch, true, baseBranch); err != nil {
				return err
			}
		}

		ui.Success(fmt.Sprintf("Worktree created at %s", wtPath))
	} else {
		// Worktree exists, get its path for setup
		wt, err := git.GetWorktree(branch)
		if err != nil {
			return err
		}
		wtPath = wt.Path
		ui.Info("Worktree already exists, continuing with setup...")
	}

	// Skip setup if --no-setup flag is set
	if wtNoSetupFlag {
		return nil
	}

	// Perform full setup
	mainPath, _ := git.GetMainWorktreePath()

	ui.SubHeader("Setting up worktree")

	// Copy files
	for _, pattern := range cfg.Worktree.CopyOnCreate {
		matches, err := filepath.Glob(filepath.Join(mainPath, pattern))
		if err != nil {
			continue
		}

		for _, src := range matches {
			filename := filepath.Base(src)
			dst := filepath.Join(wtPath, filename)

			// Copy file
			data, err := os.ReadFile(src)
			if err != nil {
				ui.Warning(fmt.Sprintf("Could not read %s: %v", filename, err))
				continue
			}

			if err := os.WriteFile(dst, data, 0644); err != nil {
				ui.Warning(fmt.Sprintf("Could not copy %s: %v", filename, err))
				continue
			}

			ui.Success(fmt.Sprintf("Copied %s", filename))
		}
	}

	// Setup environment config if enabled
	if cfg.Worktree.AutoSetupXcconfig {
		// Change to worktree directory and run env setup
		originalDir, _ := os.Getwd()
		if err := os.Chdir(wtPath); err == nil {
			if cfg.Project.IsWebPlatform() {
				ui.Info("Setting up .env.local...")

				// Check if main worktree has custom variables to copy
				mainEnvPath := filepath.Join(mainPath, ".env.local")
				if _, statErr := os.Stat(mainEnvPath); statErr == nil {
					envCopyCustomFromFlag = mainEnvPath
				}
			} else {
				ui.Info("Setting up Config.xcconfig...")
			}

			// Run env setup in the new worktree
			envBranchFlag = "" // Reset flag
			if err := runEnvSetup(cmd, nil); err != nil {
				ui.Warning(fmt.Sprintf("Could not setup environment config: %v", err))
			}

			// Reset the copy flag
			envCopyCustomFromFlag = ""

			os.Chdir(originalDir)
		}
	}

	ui.NewLine()
	ui.Success("Worktree is ready!")
	ui.KeyValue("Path", wtPath)

	// Open in VS Code if requested
	if wtOpenFlag {
		ui.Info("Opening in VS Code...")
		if err := shell.RunInteractive("code", wtPath); err != nil {
			ui.Warning("Could not open VS Code")
		}
	}

	return nil
}

// selectOrCreateBranch presents an interactive menu to select an existing branch
// or create a new one.
func selectOrCreateBranch(cfg *config.Config) (string, error) {
	ui.Header("Select Branch")

	// Get existing worktrees to filter out
	existingWorktrees := make(map[string]bool)
	worktrees, _ := git.ListWorktrees()
	for _, wt := range worktrees {
		existingWorktrees[wt.Branch] = true
	}

	// Build options list
	options := []string{"+ Create new branch"}

	// Add local branches that don't have worktrees
	localBranches, _ := git.ListBranches()
	for _, b := range localBranches {
		if !existingWorktrees[b] && b != "main" && b != "master" && b != "development" {
			options = append(options, b)
		}
	}

	// Add remote branches that don't have local worktrees
	remoteBranches, _ := git.ListRemoteBranches("origin")
	for _, b := range remoteBranches {
		if !existingWorktrees[b] && b != "main" && b != "master" && b != "development" {
			// Check if not already in options
			found := false
			for _, opt := range options {
				if opt == b {
					found = true
					break
				}
			}
			if !found {
				options = append(options, fmt.Sprintf("%s (remote)", b))
			}
		}
	}

	if len(options) == 1 {
		// Only "Create new branch" option
		ui.Info("No existing branches available")
	}

	idx, selected, err := ui.PromptSelectWithIndex("Choose a branch", options)
	if err != nil {
		return "", err
	}

	if idx == 0 {
		// Create new branch
		return createNewBranchInteractive(cfg)
	}

	// Extract branch name (remove " (remote)" suffix if present)
	branch := strings.TrimSuffix(selected, " (remote)")

	return branch, nil
}

// createNewBranchInteractive prompts the user for a new branch name and base branch.
func createNewBranchInteractive(_ *config.Config) (string, error) {
	ui.SubHeader("Create New Branch")

	// Prompt for branch name
	branchName, err := ui.PromptString("Branch name", "")
	if err != nil {
		return "", err
	}

	if branchName == "" {
		return "", fmt.Errorf("branch name cannot be empty")
	}

	// Clean up branch name (replace spaces with dashes)
	branchName = strings.ReplaceAll(branchName, " ", "-")
	branchName = strings.ToLower(branchName)

	// Suggest common prefixes
	prefixOptions := []string{
		"feat/" + branchName,
		"fix/" + branchName,
		"feature/" + branchName,
		branchName,
	}

	idx, selected, err := ui.PromptSelectWithIndex("Branch name format", prefixOptions)
	if err != nil {
		return "", err
	}

	if idx < len(prefixOptions) {
		branchName = selected
	}

	// Check if branch already exists
	if git.BranchExists(branchName) {
		return "", fmt.Errorf("branch '%s' already exists locally", branchName)
	}

	if git.RemoteBranchExists("origin", branchName) {
		return "", fmt.Errorf("branch '%s' already exists on remote", branchName)
	}

	// Build base branch options - start with common ones, then add others
	baseOptions := []string{}

	// Add development first if it exists (recommended)
	if git.BranchExists("development") || git.RemoteBranchExists("origin", "development") {
		baseOptions = append(baseOptions, "development (recommended)")
	}

	// Add main/master
	if git.BranchExists("main") || git.RemoteBranchExists("origin", "main") {
		baseOptions = append(baseOptions, "main")
	} else if git.BranchExists("master") || git.RemoteBranchExists("origin", "master") {
		baseOptions = append(baseOptions, "master")
	}

	// Add other local branches
	localBranches, _ := git.ListBranches()
	for _, b := range localBranches {
		// Skip if already added or is the new branch name
		if b == "development" || b == "main" || b == "master" || b == branchName {
			continue
		}
		baseOptions = append(baseOptions, b)
	}

	// Fallback if no branches found
	if len(baseOptions) == 0 {
		baseOptions = []string{"development", "main"}
	}

	_, selectedBase, err := ui.PromptSelectWithIndex("Create from", baseOptions)
	if err != nil {
		return "", err
	}

	// Clean up selection (remove " (recommended)" suffix)
	wtFromFlag = strings.TrimSuffix(selectedBase, " (recommended)")

	ui.Infof("Will create branch '%s' from '%s'", ui.Cyan(branchName), ui.Cyan(wtFromFlag))

	return branchName, nil
}

func runWorktreeOpen(cmd *cobra.Command, args []string) error {
	var wtPath string

	if len(args) == 1 {
		wt, err := git.GetWorktree(args[0])
		if err != nil {
			return err
		}
		wtPath = wt.Path
	} else {
		// Interactive selection
		worktrees, err := git.ListWorktrees()
		if err != nil {
			return err
		}

		if len(worktrees) == 0 {
			return fmt.Errorf("no worktrees found")
		}

		// Filter out current worktree
		options := []string{}
		paths := []string{}
		for _, wt := range worktrees {
			if !wt.IsCurrent {
				display := fmt.Sprintf("%s (%s)", wt.Branch, wt.Path)
				options = append(options, display)
				paths = append(paths, wt.Path)
			}
		}

		if len(options) == 0 {
			return fmt.Errorf("no other worktrees to open")
		}

		idx, _, err := ui.PromptSelectWithIndex("Select worktree to open", options)
		if err != nil {
			return err
		}

		wtPath = paths[idx]
	}

	if wtFinderFlag {
		ui.Info("Opening in Finder...")
		return shell.RunInteractive("open", wtPath)
	}

	if wtTermFlag {
		ui.Info("Opening in Terminal...")
		return shell.RunInteractive("open", "-a", "Terminal", wtPath)
	}

	ui.Info("Opening in VS Code...")
	return shell.RunInteractive("code", wtPath)
}

func runWorktreeDelete(cmd *cobra.Command, args []string) error {
	var wt *git.Worktree

	if len(args) == 1 {
		var err error
		wt, err = git.GetWorktree(args[0])
		if err != nil {
			return err
		}
	} else {
		// Interactive selection
		worktrees, err := git.ListWorktrees()
		if err != nil {
			return err
		}

		// Filter out current worktree and bare repos
		options := []string{}
		candidates := []*git.Worktree{}
		for i := range worktrees {
			wt := &worktrees[i]
			if !wt.IsCurrent && !wt.IsBare {
				display := fmt.Sprintf("%s (%s)", wt.Branch, wt.Path)
				options = append(options, display)
				candidates = append(candidates, wt)
			}
		}

		if len(options) == 0 {
			return fmt.Errorf("no worktrees available to delete")
		}

		idx, _, err := ui.PromptSelectWithIndex("Select worktree to delete", options)
		if err != nil {
			return err
		}

		wt = candidates[idx]
	}

	// Confirm deletion
	if !IsYes() {
		ui.Warningf("This will delete worktree: %s", wt.Path)
		confirmed, err := ui.PromptYesNo("Are you sure?", false)
		if err != nil || !confirmed {
			ui.Info("Cancelled")
			return nil
		}
	}

	// Delete worktree
	ui.Infof("Deleting worktree for branch '%s'", wt.Branch)

	if err := git.RemoveWorktree(wt.Path, wtForceFlag); err != nil {
		return err
	}

	// Optionally delete branch
	if !wtForceFlag {
		deleteBranch, _ := ui.PromptYesNo(fmt.Sprintf("Also delete branch '%s'?", wt.Branch), false)
		if deleteBranch {
			if err := git.DeleteBranch(wt.Branch, false); err != nil {
				ui.Warning(fmt.Sprintf("Could not delete branch: %v", err))
			} else {
				ui.Success(fmt.Sprintf("Deleted branch '%s'", wt.Branch))
			}
		}
	}

	ui.Success("Worktree deleted")
	return nil
}

func runWorktreePath(cmd *cobra.Command, args []string) error {
	wt, err := git.GetWorktree(args[0])
	if err != nil {
		return err
	}

	fmt.Println(wt.Path)
	return nil
}

func runWorktreePrune(cmd *cobra.Command, args []string) error {
	ui.Info("Pruning stale worktree entries...")

	if err := git.PruneWorktrees(); err != nil {
		return err
	}

	ui.Success("Worktrees pruned")
	return nil
}

func runWorktreeInfo(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}

	var wt *git.Worktree
	var err error

	if len(args) == 1 {
		wt, err = git.GetWorktree(args[0])
	} else {
		// Get current worktree
		worktrees, err := git.ListWorktrees()
		if err != nil {
			return err
		}
		for i := range worktrees {
			if worktrees[i].IsCurrent {
				wt = &worktrees[i]
				break
			}
		}
		if wt == nil {
			return fmt.Errorf("could not determine current worktree")
		}
	}
	if err != nil {
		return err
	}

	ui.Header("Worktree Info")
	ui.KeyValue("Branch", ui.Cyan(wt.Branch))
	ui.KeyValue("Path", wt.Path)

	// Get ahead/behind counts
	ahead, behind, err := git.GetAheadBehind(wt.Path, wt.Branch)
	if err == nil {
		status := ""
		if ahead > 0 && behind > 0 {
			status = fmt.Sprintf("%s, %s", ui.Green(fmt.Sprintf("+%d ahead", ahead)), ui.Red(fmt.Sprintf("-%d behind", behind)))
		} else if ahead > 0 {
			status = ui.Green(fmt.Sprintf("+%d ahead", ahead))
		} else if behind > 0 {
			status = ui.Red(fmt.Sprintf("-%d behind", behind))
		} else {
			status = ui.Green("up to date")
		}
		ui.KeyValue("Remote Status", status)
	}

	// Get uncommitted changes
	changes, err := git.GetUncommittedChanges(wt.Path)
	if err == nil {
		if changes > 0 {
			ui.KeyValue("Uncommitted", ui.Yellow(fmt.Sprintf("%d file(s)", changes)))
		} else {
			ui.KeyValue("Uncommitted", ui.Green("clean"))
		}
	}

	// Environment detection
	env := "Feature"
	if wt.Branch == "main" || wt.Branch == "master" {
		env = "Production"
	} else if wt.Branch == "development" || wt.Branch == "dev" {
		env = "Development"
	}
	ui.KeyValue("Environment", envColorString(env))

	return nil
}

func runWorktreeCleanup(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}

	ui.Header("Worktree Cleanup")

	// Get merged branches
	mergedBranches, err := git.GetMergedBranches()
	if err != nil {
		return fmt.Errorf("could not get merged branches: %w", err)
	}

	// Get worktrees
	worktrees, err := git.ListWorktrees()
	if err != nil {
		return err
	}

	// Find worktrees with merged branches
	var cleanupCandidates []*git.Worktree
	for i := range worktrees {
		wt := &worktrees[i]
		if wt.IsBare || wt.IsCurrent {
			continue
		}
		// Skip protected branches
		if wt.Branch == "main" || wt.Branch == "master" || wt.Branch == "development" {
			continue
		}
		// Check if branch is merged
		for _, merged := range mergedBranches {
			if wt.Branch == merged {
				cleanupCandidates = append(cleanupCandidates, wt)
				break
			}
		}
	}

	if len(cleanupCandidates) == 0 {
		ui.Success("No worktrees to clean up - all branches are unmerged")
		return nil
	}

	ui.Infof("Found %d worktree(s) with merged branches:", len(cleanupCandidates))
	ui.NewLine()

	for _, wt := range cleanupCandidates {
		ui.List(fmt.Sprintf("%s → %s", ui.Cyan(wt.Branch), ui.Dim(wt.Path)))
	}

	ui.NewLine()

	// Process each candidate
	deletedCount := 0
	for _, wt := range cleanupCandidates {
		ui.NewLine()
		ui.Infof("Branch '%s' has been merged", wt.Branch)

		// Check for uncommitted changes
		changes, _ := git.GetUncommittedChanges(wt.Path)
		if changes > 0 {
			ui.Warning(fmt.Sprintf("Has %d uncommitted changes", changes))
		}

		confirmed, err := ui.PromptYesNo(fmt.Sprintf("Delete worktree for '%s'?", wt.Branch), false)
		if err != nil || !confirmed {
			ui.Info("Skipped")
			continue
		}

		// Delete worktree
		if err := git.RemoveWorktree(wt.Path, false); err != nil {
			ui.Warning(fmt.Sprintf("Could not delete worktree: %v", err))
			continue
		}
		ui.Success(fmt.Sprintf("Deleted worktree: %s", wt.Path))

		// Ask about deleting the branch
		deleteRemote, _ := ui.PromptYesNo(fmt.Sprintf("Also delete remote branch '%s'?", wt.Branch), false)
		if deleteRemote {
			if err := git.DeleteRemoteBranch("origin", wt.Branch); err != nil {
				ui.Warning(fmt.Sprintf("Could not delete remote branch: %v", err))
			} else {
				ui.Success(fmt.Sprintf("Deleted remote branch: origin/%s", wt.Branch))
			}
		}

		// Delete local branch
		if err := git.DeleteBranch(wt.Branch, false); err != nil {
			ui.Warning(fmt.Sprintf("Could not delete local branch: %v", err))
		} else {
			ui.Success(fmt.Sprintf("Deleted local branch: %s", wt.Branch))
		}

		deletedCount++
	}

	ui.NewLine()
	ui.Successf("Cleanup complete: %d worktree(s) deleted", deletedCount)

	return nil
}

func runWorktreeSync(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}

	ui.Header("Sync Worktrees")

	// Get worktrees
	worktrees, err := git.ListWorktrees()
	if err != nil {
		return err
	}

	// Filter to non-bare worktrees
	var syncable []git.Worktree
	for _, wt := range worktrees {
		if !wt.IsBare && wt.Branch != "" {
			syncable = append(syncable, wt)
		}
	}

	if len(syncable) == 0 {
		ui.Info("No worktrees to sync")
		return nil
	}

	// Build options for multi-select
	options := make([]string, len(syncable))
	for i, wt := range syncable {
		status := ""
		if wt.IsCurrent {
			status = " (current)"
		}
		options[i] = fmt.Sprintf("%s%s", wt.Branch, status)
	}

	// Show multi-select
	selected, err := ui.PromptMultiSelect("Select worktrees to sync", options, nil)
	if err != nil {
		return err
	}

	if len(selected) == 0 {
		ui.Info("No worktrees selected")
		return nil
	}

	ui.NewLine()
	ui.Infof("Syncing %d worktree(s)...", len(selected))
	ui.NewLine()

	successCount := 0
	for _, sel := range selected {
		// Find the worktree
		branchName := strings.TrimSuffix(sel, " (current)")
		var wt *git.Worktree
		for i := range syncable {
			if syncable[i].Branch == branchName {
				wt = &syncable[i]
				break
			}
		}

		if wt == nil {
			continue
		}

		sp := ui.NewSpinner(fmt.Sprintf("Syncing %s", wt.Branch))
		sp.Start()

		// Pull from origin
		err := git.PullInWorktree(wt.Path)
		if err != nil {
			sp.Fail(fmt.Sprintf("Failed to sync %s: %v", wt.Branch, err))
			continue
		}

		sp.Success(fmt.Sprintf("Synced %s", wt.Branch))
		successCount++
	}

	ui.NewLine()
	ui.Successf("Sync complete: %d/%d worktrees updated", successCount, len(selected))

	return nil
}

