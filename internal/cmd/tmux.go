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

var tmuxCmd = &cobra.Command{
	Use:   "tmux",
	Short: "Tmux session management",
	Long: `Manage tmux sessions for your worktrees and projects.

Without arguments, shows an interactive picker to attach to an existing
session or create a new one for the current worktree.

Sessions are automatically named after worktree directories for easy
identification.`,
	Example: `  drift tmux              # Interactive session picker
  drift tmux list         # List all sessions
  drift tmux new          # Create session for current worktree
  drift tmux attach       # Attach to session (interactive)
  drift tmux kill         # Kill a session (interactive)`,
	RunE: runTmuxInteractive,
}

var tmuxListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tmux sessions",
	Long:  `List all active tmux sessions with their status.`,
	RunE:  runTmuxList,
}

var tmuxNewCmd = &cobra.Command{
	Use:   "new [name]",
	Short: "Create a new tmux session",
	Long: `Create a new tmux session. If no name is provided, uses the current
worktree directory name.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTmuxNew,
}

var tmuxAttachCmd = &cobra.Command{
	Use:   "attach [name]",
	Short: "Attach to a tmux session",
	Long:  `Attach to an existing tmux session. If no name is provided, shows an interactive picker.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runTmuxAttach,
}

var tmuxKillCmd = &cobra.Command{
	Use:   "kill [name]",
	Short: "Kill a tmux session",
	Long:  `Kill a tmux session. If no name is provided, shows an interactive picker.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runTmuxKill,
}

var (
	tmuxDetachedFlag bool
)

func init() {
	tmuxNewCmd.Flags().BoolVarP(&tmuxDetachedFlag, "detached", "d", false, "Create session in detached mode")

	tmuxCmd.AddCommand(tmuxListCmd)
	tmuxCmd.AddCommand(tmuxNewCmd)
	tmuxCmd.AddCommand(tmuxAttachCmd)
	tmuxCmd.AddCommand(tmuxKillCmd)
	rootCmd.AddCommand(tmuxCmd)
}

// TmuxSession represents a tmux session.
type TmuxSession struct {
	Name      string
	Windows   int
	Created   string
	Attached  bool
	Directory string
}

// listTmuxSessions returns all active tmux sessions.
func listTmuxSessions() ([]TmuxSession, error) {
	// Check if tmux server is running
	result, err := shell.Run("tmux", "list-sessions", "-F", "#{session_name}|#{session_windows}|#{session_created_string}|#{session_attached}|#{session_path}")
	if err != nil || result.ExitCode != 0 {
		// No sessions or tmux not running
		return []TmuxSession{}, nil
	}

	var sessions []TmuxSession
	for _, line := range strings.Split(result.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) >= 4 {
			session := TmuxSession{
				Name:     parts[0],
				Created:  parts[2],
				Attached: parts[3] == "1",
			}

			// Parse window count
			fmt.Sscanf(parts[1], "%d", &session.Windows)

			// Get directory if available
			if len(parts) >= 5 {
				session.Directory = parts[4]
			}

			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// getWorktreeSessions returns tmux sessions that match worktree names.
func getWorktreeSessions() ([]TmuxSession, error) {
	sessions, err := listTmuxSessions()
	if err != nil {
		return nil, err
	}

	worktrees, err := git.ListWorktrees()
	if err != nil {
		return sessions, nil // Return all sessions if we can't get worktrees
	}

	// Build a map of worktree names
	wtNames := make(map[string]bool)
	for _, wt := range worktrees {
		name := filepath.Base(wt.Path)
		wtNames[name] = true
	}

	// Filter sessions that match worktree names
	var wtSessions []TmuxSession
	for _, s := range sessions {
		if wtNames[s.Name] {
			wtSessions = append(wtSessions, s)
		}
	}

	return wtSessions, nil
}

// getCurrentWorktreeName returns the name for a tmux session based on current directory.
func getCurrentWorktreeName() string {
	// Try to get from config
	cfg, err := config.Load()
	if err == nil && cfg.Project.Name != "" {
		// Get current branch
		branch, err := git.CurrentBranch()
		if err == nil {
			// Sanitize branch name for tmux
			sanitized := strings.ReplaceAll(branch, "/", "-")
			sanitized = strings.ReplaceAll(sanitized, ".", "-")
			return fmt.Sprintf("%s-%s", cfg.Project.Name, sanitized)
		}
	}

	// Fall back to current directory name
	cwd, err := os.Getwd()
	if err != nil {
		return "drift-session"
	}
	return filepath.Base(cwd)
}

func runTmuxInteractive(cmd *cobra.Command, args []string) error {
	// Check if tmux is installed
	if !shell.CommandExists("tmux") {
		ui.Error("tmux is not installed")
		ui.Info("Install with: brew install tmux")
		return fmt.Errorf("tmux not found")
	}

	// Check if we're already in a tmux session
	if os.Getenv("TMUX") != "" {
		ui.Warning("Already inside a tmux session")
		ui.Info("Use 'tmux switch-client' or detach first")
		return nil
	}

	sessions, err := listTmuxSessions()
	if err != nil {
		return err
	}

	if len(sessions) == 0 {
		// No sessions, create one for current worktree
		ui.Info("No tmux sessions found")
		return runTmuxNew(cmd, args)
	}

	// Build selection items
	items := make([]string, 0, len(sessions)+1)
	items = append(items, fmt.Sprintf("+ Create new session (%s)", getCurrentWorktreeName()))

	for _, s := range sessions {
		status := ""
		if s.Attached {
			status = " (attached)"
		}
		items = append(items, fmt.Sprintf("%s [%d windows]%s", s.Name, s.Windows, status))
	}

	// Show interactive picker
	selected, err := ui.PromptSelect("Select tmux session", items)
	if err != nil {
		return err
	}

	// Check if "Create new" was selected
	if strings.HasPrefix(selected, "+ Create new") {
		return runTmuxNew(cmd, args)
	}

	// Extract session name from selection
	sessionName := strings.Split(selected, " ")[0]

	// Attach to session
	ui.Infof("Attaching to session: %s", sessionName)
	return shell.RunInteractive("tmux", "attach-session", "-t", sessionName)
}

func runTmuxList(cmd *cobra.Command, args []string) error {
	// Check if tmux is installed
	if !shell.CommandExists("tmux") {
		ui.Error("tmux is not installed")
		ui.Info("Install with: brew install tmux")
		return fmt.Errorf("tmux not found")
	}

	sessions, err := listTmuxSessions()
	if err != nil {
		return err
	}

	if len(sessions) == 0 {
		ui.Info("No tmux sessions running")
		return nil
	}

	ui.Header("Tmux Sessions")

	// Get worktree names for highlighting
	worktrees, _ := git.ListWorktrees()
	wtNames := make(map[string]bool)
	for _, wt := range worktrees {
		wtNames[filepath.Base(wt.Path)] = true
	}

	for _, s := range sessions {
		status := ui.Dim("detached")
		if s.Attached {
			status = ui.Green("attached")
		}

		name := s.Name
		if wtNames[s.Name] {
			name = ui.Cyan(s.Name) // Highlight worktree sessions
		}

		ui.KeyValue(name, fmt.Sprintf("%d windows, %s", s.Windows, status))
	}

	return nil
}

func runTmuxNew(cmd *cobra.Command, args []string) error {
	// Check if tmux is installed
	if !shell.CommandExists("tmux") {
		ui.Error("tmux is not installed")
		ui.Info("Install with: brew install tmux")
		return fmt.Errorf("tmux not found")
	}

	// Determine session name
	var sessionName string
	if len(args) > 0 {
		sessionName = args[0]
	} else {
		sessionName = getCurrentWorktreeName()
	}

	// Sanitize session name (tmux doesn't like certain characters)
	sessionName = strings.ReplaceAll(sessionName, ":", "-")
	sessionName = strings.ReplaceAll(sessionName, ".", "-")

	// Check if session already exists
	sessions, _ := listTmuxSessions()
	for _, s := range sessions {
		if s.Name == sessionName {
			ui.Warningf("Session '%s' already exists", sessionName)
			ui.Info("Attaching to existing session...")
			return shell.RunInteractive("tmux", "attach-session", "-t", sessionName)
		}
	}

	// Check if we're already in tmux
	inTmux := os.Getenv("TMUX") != ""

	ui.Successf("Creating tmux session: %s", sessionName)

	if inTmux || tmuxDetachedFlag {
		// Create detached session
		result, err := shell.Run("tmux", "new-session", "-d", "-s", sessionName)
		if err != nil || result.ExitCode != 0 {
			return fmt.Errorf("failed to create session: %s", result.Stderr)
		}
		ui.Infof("Session created (detached). Attach with: tmux attach -t %s", sessionName)
		return nil
	}

	// Create and attach to session
	return shell.RunInteractive("tmux", "new-session", "-s", sessionName)
}

func runTmuxAttach(cmd *cobra.Command, args []string) error {
	// Check if tmux is installed
	if !shell.CommandExists("tmux") {
		ui.Error("tmux is not installed")
		ui.Info("Install with: brew install tmux")
		return fmt.Errorf("tmux not found")
	}

	// Check if we're already in a tmux session
	if os.Getenv("TMUX") != "" {
		ui.Warning("Already inside a tmux session")
		ui.Info("Use 'tmux switch-client -t <session>' to switch")
		return nil
	}

	sessions, err := listTmuxSessions()
	if err != nil {
		return err
	}

	if len(sessions) == 0 {
		ui.Info("No tmux sessions running")
		ui.Info("Create one with: drift tmux new")
		return nil
	}

	var sessionName string
	if len(args) > 0 {
		sessionName = args[0]
	} else {
		// Interactive picker
		items := make([]string, len(sessions))
		for i, s := range sessions {
			status := ""
			if s.Attached {
				status = " (attached)"
			}
			items[i] = fmt.Sprintf("%s [%d windows]%s", s.Name, s.Windows, status)
		}

		selected, err := ui.PromptSelect("Select session to attach", items)
		if err != nil {
			return err
		}
		sessionName = strings.Split(selected, " ")[0]
	}

	ui.Infof("Attaching to session: %s", sessionName)
	return shell.RunInteractive("tmux", "attach-session", "-t", sessionName)
}

func runTmuxKill(cmd *cobra.Command, args []string) error {
	// Check if tmux is installed
	if !shell.CommandExists("tmux") {
		ui.Error("tmux is not installed")
		ui.Info("Install with: brew install tmux")
		return fmt.Errorf("tmux not found")
	}

	sessions, err := listTmuxSessions()
	if err != nil {
		return err
	}

	if len(sessions) == 0 {
		ui.Info("No tmux sessions running")
		return nil
	}

	var sessionName string
	if len(args) > 0 {
		sessionName = args[0]
	} else {
		// Interactive picker
		items := make([]string, len(sessions))
		for i, s := range sessions {
			status := ""
			if s.Attached {
				status = " (attached)"
			}
			items[i] = fmt.Sprintf("%s [%d windows]%s", s.Name, s.Windows, status)
		}

		selected, err := ui.PromptSelect("Select session to kill", items)
		if err != nil {
			return err
		}
		sessionName = strings.Split(selected, " ")[0]
	}

	// Confirm
	confirmed, err := ui.PromptYesNo(fmt.Sprintf("Kill session '%s'?", sessionName), false)
	if err != nil || !confirmed {
		ui.Info("Cancelled")
		return nil
	}

	result, err := shell.Run("tmux", "kill-session", "-t", sessionName)
	if err != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to kill session: %s", result.Stderr)
	}

	ui.Successf("Session '%s' killed", sessionName)
	return nil
}
