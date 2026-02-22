package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/git"
	"github.com/undrift/drift/internal/supabase"
	"github.com/undrift/drift/internal/ui"
)

var (
	branchesAllFlag bool
)

var branchesCmd = &cobra.Command{
	Use:     "branches",
	Aliases: []string{"br"},
	Short:   "Supabase branch lifecycle management",
	Long: `Manage Supabase preview branches — list, pause, resume, create, delete, and clean up.

Pausing inactive preview branches saves costs. Use 'drift branches pause' to
shut down non-essential branches and 'drift branches resume' to bring them back.`,
}

// ── list ────────────────────────────────────────────────────────────────────

var branchesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all Supabase branches with status",
	RunE:  runBranchesList,
}

func runBranchesList(cmd *cobra.Command, args []string) error {
	client := supabase.NewClient()
	branches, err := client.GetBranches()
	if err != nil {
		return err
	}

	if len(branches) == 0 {
		ui.Info("No Supabase branches found")
		return nil
	}

	// Classify and sort
	type entry struct {
		Branch supabase.Branch
		Env    supabase.Environment
	}
	entries := make([]entry, len(branches))
	for i, b := range branches {
		entries[i] = entry{Branch: b, Env: environmentForBranch(&branches[i])}
	}
	sort.Slice(entries, func(i, j int) bool {
		wi, wj := envWeight(entries[i].Env), envWeight(entries[j].Env)
		if wi != wj {
			return wi < wj
		}
		return entries[i].Branch.GitBranch < entries[j].Branch.GitBranch
	})

	// Build table
	table := ui.NewTable([]string{"Git Branch", "Environment", "Status", "Updated"})
	var active, paused int
	for _, e := range entries {
		status := e.Branch.Status
		var statusColors []tablewriter.Colors

		switch strings.ToLower(status) {
		case "active_healthy":
			active++
			statusColors = []tablewriter.Colors{{}, {}, ui.TableColor.Green, {}}
			status = "active"
		case "paused":
			paused++
			statusColors = []tablewriter.Colors{{}, {}, ui.TableColor.Yellow, {}}
		default:
			statusColors = []tablewriter.Colors{{}, {}, {}, {}}
		}

		updated := ""
		if e.Branch.UpdatedAt != "" {
			// Trim to date only
			if idx := strings.IndexByte(e.Branch.UpdatedAt, 'T'); idx > 0 {
				updated = e.Branch.UpdatedAt[:idx]
			} else {
				updated = e.Branch.UpdatedAt
			}
		}

		table.AddColoredRow([]string{
			e.Branch.GitBranch,
			string(e.Env),
			status,
			updated,
		}, statusColors)
	}
	table.Render()

	fmt.Printf("\nTotal: %d branches (%d active, %d paused)\n", len(entries), active, paused)
	return nil
}

// ── pause ───────────────────────────────────────────────────────────────────

var branchesPauseCmd = &cobra.Command{
	Use:   "pause [branch-names...]",
	Short: "Pause active preview branches to save costs",
	Long: `Pause active Supabase preview branches.

Only Feature branches are eligible — Production and Development are never paused.
By default shows an interactive multi-select with all branches pre-selected.`,
	RunE: runBranchesPause,
}

func runBranchesPause(cmd *cobra.Command, args []string) error {
	client := supabase.NewClient()
	branches, err := client.GetBranches()
	if err != nil {
		return err
	}

	// Filter to active Feature branches
	var candidates []supabase.Branch
	for _, b := range branches {
		env := environmentForBranch(&b)
		if env == supabase.EnvFeature && strings.ToLower(b.Status) == "active_healthy" {
			candidates = append(candidates, b)
		}
	}

	if len(candidates) == 0 {
		ui.Info("No active feature branches to pause")
		return nil
	}

	var selected []supabase.Branch

	if len(args) > 0 {
		selected, err = matchBranchesByNames(candidates, args)
		if err != nil {
			return err
		}
	} else if branchesAllFlag || IsYes() {
		selected = candidates
	} else {
		// Interactive multi-select — all pre-selected (common: shut everything down)
		items := branchNames(candidates)
		chosen, err := ui.PromptMultiSelect("Select branches to pause", items, items)
		if err != nil {
			return err
		}
		if len(chosen) == 0 {
			ui.Info("No branches selected")
			return nil
		}
		selected, _ = matchBranchesByNames(candidates, chosen)
	}

	// Confirm
	fmt.Printf("\nBranches to pause (%d):\n", len(selected))
	for _, b := range selected {
		fmt.Printf("  %s %s\n", ui.Yellow("⏸"), b.GitBranch)
	}

	if !IsYes() {
		ok, err := ui.PromptYesNo("Pause these branches?", true)
		if err != nil {
			return err
		}
		if !ok {
			ui.Info("Cancelled")
			return nil
		}
	}

	// Execute
	var succeeded int
	for _, b := range selected {
		sp := ui.NewSpinner(fmt.Sprintf("Pausing %s", b.GitBranch))
		sp.Start()
		if err := client.PauseBranch(b.GitBranch); err != nil {
			sp.Fail(fmt.Sprintf("Failed to pause %s: %s", b.GitBranch, err))
			continue
		}
		sp.Success(fmt.Sprintf("Paused %s", b.GitBranch))
		succeeded++
	}

	ui.Successf("Paused %d/%d branches", succeeded, len(selected))
	return nil
}

// ── resume ──────────────────────────────────────────────────────────────────

var branchesResumeCmd = &cobra.Command{
	Use:   "resume [branch-names...]",
	Short: "Resume paused preview branches",
	Long: `Resume paused Supabase preview branches.

Only paused Feature branches are eligible.
By default shows an interactive multi-select with all branches pre-selected.`,
	RunE: runBranchesResume,
}

func runBranchesResume(cmd *cobra.Command, args []string) error {
	client := supabase.NewClient()
	branches, err := client.GetBranches()
	if err != nil {
		return err
	}

	// Filter to paused Feature branches
	var candidates []supabase.Branch
	for _, b := range branches {
		env := environmentForBranch(&b)
		if env == supabase.EnvFeature && strings.ToLower(b.Status) == "paused" {
			candidates = append(candidates, b)
		}
	}

	if len(candidates) == 0 {
		ui.Info("No paused feature branches to resume")
		return nil
	}

	var selected []supabase.Branch

	if len(args) > 0 {
		selected, err = matchBranchesByNames(candidates, args)
		if err != nil {
			return err
		}
	} else if branchesAllFlag || IsYes() {
		selected = candidates
	} else {
		items := branchNames(candidates)
		chosen, err := ui.PromptMultiSelect("Select branches to resume", items, items)
		if err != nil {
			return err
		}
		if len(chosen) == 0 {
			ui.Info("No branches selected")
			return nil
		}
		selected, _ = matchBranchesByNames(candidates, chosen)
	}

	fmt.Printf("\nBranches to resume (%d):\n", len(selected))
	for _, b := range selected {
		fmt.Printf("  %s %s\n", ui.Green("▶"), b.GitBranch)
	}

	if !IsYes() {
		ok, err := ui.PromptYesNo("Resume these branches?", true)
		if err != nil {
			return err
		}
		if !ok {
			ui.Info("Cancelled")
			return nil
		}
	}

	var succeeded int
	for _, b := range selected {
		sp := ui.NewSpinner(fmt.Sprintf("Resuming %s", b.GitBranch))
		sp.Start()
		if err := client.UnpauseBranch(b.GitBranch); err != nil {
			sp.Fail(fmt.Sprintf("Failed to resume %s: %s", b.GitBranch, err))
			continue
		}
		sp.Success(fmt.Sprintf("Resumed %s", b.GitBranch))
		succeeded++
	}

	ui.Successf("Resumed %d/%d branches", succeeded, len(selected))
	return nil
}

// ── create ──────────────────────────────────────────────────────────────────

var branchesCreateCmd = &cobra.Command{
	Use:   "create [branch-name]",
	Short: "Create a new Supabase preview branch",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runBranchesCreate,
}

func runBranchesCreate(cmd *cobra.Command, args []string) error {
	var branchName string
	if len(args) > 0 {
		branchName = args[0]
	} else {
		name, err := ui.PromptString("Branch name", "")
		if err != nil {
			return err
		}
		branchName = strings.TrimSpace(name)
		if branchName == "" {
			return fmt.Errorf("branch name cannot be empty")
		}
	}

	client := supabase.NewClient()

	// Check if branch already exists
	existing, _ := client.GetBranch(branchName)
	if existing != nil {
		return fmt.Errorf("Supabase branch '%s' already exists (status: %s)", branchName, existing.Status)
	}

	sp := ui.NewSpinner(fmt.Sprintf("Creating branch %s", branchName))
	sp.Start()

	branch, err := client.CreateBranch(branchName)
	if err != nil {
		sp.Fail(fmt.Sprintf("Failed to create branch: %s", err))
		return nil
	}
	sp.Stop()

	ui.Successf("Created branch %s", branchName)
	if branch != nil {
		ui.KeyValue("ID", branch.ID)
		ui.KeyValue("Git Branch", branch.GitBranch)
		ui.KeyValue("Status", branch.Status)
	}
	return nil
}

// ── delete ──────────────────────────────────────────────────────────────────

var branchesDeleteCmd = &cobra.Command{
	Use:   "delete [branch-names...]",
	Short: "Delete Supabase preview branches",
	Long: `Delete Supabase preview branches.

Only Feature branches are eligible — Production and Development cannot be deleted.
By default shows an interactive multi-select with none pre-selected (safe default).`,
	RunE: runBranchesDelete,
}

func runBranchesDelete(cmd *cobra.Command, args []string) error {
	client := supabase.NewClient()
	branches, err := client.GetBranches()
	if err != nil {
		return err
	}

	var candidates []supabase.Branch
	for _, b := range branches {
		if environmentForBranch(&b) == supabase.EnvFeature {
			candidates = append(candidates, b)
		}
	}

	if len(candidates) == 0 {
		ui.Info("No feature branches to delete")
		return nil
	}

	var selected []supabase.Branch

	if len(args) > 0 {
		selected, err = matchBranchesByNames(candidates, args)
		if err != nil {
			return err
		}
	} else if branchesAllFlag {
		selected = candidates
	} else {
		// None pre-selected — destructive action
		items := branchNames(candidates)
		chosen, err := ui.PromptMultiSelect("Select branches to delete", items, nil)
		if err != nil {
			return err
		}
		if len(chosen) == 0 {
			ui.Info("No branches selected")
			return nil
		}
		selected, _ = matchBranchesByNames(candidates, chosen)
	}

	fmt.Printf("\n%s Branches to delete (%d):\n", ui.Red("⚠"), len(selected))
	for _, b := range selected {
		fmt.Printf("  %s %s (%s)\n", ui.Red("✗"), b.GitBranch, b.Status)
	}

	if !IsYes() {
		ok, err := ui.PromptYesNo("Delete these branches? This cannot be undone", false)
		if err != nil {
			return err
		}
		if !ok {
			ui.Info("Cancelled")
			return nil
		}
	}

	var succeeded int
	for _, b := range selected {
		sp := ui.NewSpinner(fmt.Sprintf("Deleting %s", b.GitBranch))
		sp.Start()
		if err := client.DeleteBranch(b.GitBranch); err != nil {
			sp.Fail(fmt.Sprintf("Failed to delete %s: %s", b.GitBranch, err))
			continue
		}
		sp.Success(fmt.Sprintf("Deleted %s", b.GitBranch))
		succeeded++
	}

	ui.Successf("Deleted %d/%d branches", succeeded, len(selected))
	return nil
}

// ── cleanup ─────────────────────────────────────────────────────────────────

var branchesCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Find and delete orphaned Supabase branches",
	Long: `Find Supabase preview branches whose git branch no longer exists on the remote.

Production and Development branches are never considered orphaned.
Shows an interactive multi-select to choose which orphaned branches to delete.`,
	RunE: runBranchesCleanup,
}

func runBranchesCleanup(cmd *cobra.Command, args []string) error {
	client := supabase.NewClient()

	sp := ui.NewSpinner("Fetching branches")
	sp.Start()

	branches, err := client.GetBranches()
	if err != nil {
		sp.Fail("Failed to fetch Supabase branches")
		return err
	}

	remoteBranches, err := git.ListRemoteBranches("origin")
	if err != nil {
		sp.Fail("Failed to list remote git branches")
		return err
	}
	sp.Stop()

	// Build lookup set for remote branches
	remoteSet := make(map[string]bool, len(remoteBranches))
	for _, rb := range remoteBranches {
		remoteSet[rb] = true
	}

	// Find orphaned Feature branches
	var orphaned []supabase.Branch
	for _, b := range branches {
		if environmentForBranch(&b) != supabase.EnvFeature {
			continue
		}
		if !remoteSet[b.GitBranch] {
			orphaned = append(orphaned, b)
		}
	}

	if len(orphaned) == 0 {
		ui.Success("All branches are synced with remote")
		return nil
	}

	// Show orphaned branches
	fmt.Printf("\nOrphaned Supabase branches (no matching git remote):\n")
	table := ui.NewTable([]string{"Git Branch", "Status", "Updated"})
	for _, b := range orphaned {
		updated := ""
		if b.UpdatedAt != "" {
			if idx := strings.IndexByte(b.UpdatedAt, 'T'); idx > 0 {
				updated = b.UpdatedAt[:idx]
			} else {
				updated = b.UpdatedAt
			}
		}
		table.AddRow([]string{b.GitBranch, b.Status, updated})
	}
	table.Render()
	fmt.Println()

	// Interactive selection — none pre-selected (destructive)
	items := branchNames(orphaned)
	chosen, err := ui.PromptMultiSelect("Select orphaned branches to delete", items, nil)
	if err != nil {
		return err
	}
	if len(chosen) == 0 {
		ui.Info("No branches selected")
		return nil
	}

	selected, _ := matchBranchesByNames(orphaned, chosen)

	if !IsYes() {
		ok, err := ui.PromptYesNo(fmt.Sprintf("Delete %d orphaned branches?", len(selected)), false)
		if err != nil {
			return err
		}
		if !ok {
			ui.Info("Cancelled")
			return nil
		}
	}

	var succeeded int
	for _, b := range selected {
		sp := ui.NewSpinner(fmt.Sprintf("Deleting %s", b.GitBranch))
		sp.Start()
		if err := client.DeleteBranch(b.GitBranch); err != nil {
			sp.Fail(fmt.Sprintf("Failed to delete %s: %s", b.GitBranch, err))
			continue
		}
		sp.Success(fmt.Sprintf("Deleted %s", b.GitBranch))
		succeeded++
	}

	ui.Successf("Cleaned up %d/%d orphaned branches", succeeded, len(selected))
	return nil
}

// ── helpers ─────────────────────────────────────────────────────────────────

// branchNames returns the GitBranch names from a slice of branches.
func branchNames(branches []supabase.Branch) []string {
	names := make([]string, len(branches))
	for i, b := range branches {
		names[i] = b.GitBranch
	}
	return names
}

// matchBranchesByNames matches positional args against GitBranch or Name fields.
// Returns an error listing any names that were not found.
func matchBranchesByNames(candidates []supabase.Branch, names []string) ([]supabase.Branch, error) {
	lookup := make(map[string]*supabase.Branch, len(candidates))
	for i := range candidates {
		lookup[candidates[i].GitBranch] = &candidates[i]
		lookup[candidates[i].Name] = &candidates[i]
	}

	seen := make(map[string]bool)
	var matched []supabase.Branch
	var missing []string

	for _, name := range names {
		b, ok := lookup[name]
		if !ok {
			missing = append(missing, name)
			continue
		}
		if !seen[b.ID] {
			seen[b.ID] = true
			matched = append(matched, *b)
		}
	}

	if len(missing) > 0 {
		return matched, fmt.Errorf("branches not found: %s", strings.Join(missing, ", "))
	}
	return matched, nil
}

func init() {
	// Pause flags
	branchesPauseCmd.Flags().BoolVar(&branchesAllFlag, "all", false, "Pause all active feature branches without prompting")

	// Resume flags — reuse same var since only one subcommand runs at a time
	branchesResumeCmd.Flags().BoolVar(&branchesAllFlag, "all", false, "Resume all paused feature branches without prompting")

	// Delete flags
	branchesDeleteCmd.Flags().BoolVar(&branchesAllFlag, "all", false, "Delete all feature branches without prompting")

	branchesCmd.AddCommand(branchesListCmd)
	branchesCmd.AddCommand(branchesPauseCmd)
	branchesCmd.AddCommand(branchesResumeCmd)
	branchesCmd.AddCommand(branchesCreateCmd)
	branchesCmd.AddCommand(branchesDeleteCmd)
	branchesCmd.AddCommand(branchesCleanupCmd)
	rootCmd.AddCommand(branchesCmd)
}
