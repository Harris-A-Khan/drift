package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/cockpit"
	"github.com/undrift/drift/internal/ui"
	"github.com/undrift/drift/pkg/shell"
)

var cockpitCmd = &cobra.Command{
	Use:   "cockpit",
	Short: "Live dashboard for managing Claude Code sessions",
	Long: `Interactive TUI dashboard showing all Claude Code sessions grouped by project.

Discovers tmux sessions, detects Claude Code instances, groups them by
drift project, and shows live pane previews. Navigate, attach, switch,
and kill sessions from one place.

Requires tmux to be installed.`,
	RunE: runCockpit,
}

func init() {
	rootCmd.AddCommand(cockpitCmd)
}

func runCockpit(cmd *cobra.Command, args []string) error {
	// Check if tmux is installed
	if !shell.CommandExists("tmux") {
		ui.Error("tmux is not installed")
		ui.Info("Install with: brew install tmux")
		return fmt.Errorf("tmux not found")
	}

	inTmux := os.Getenv("TMUX") != ""

	model := cockpit.NewModel(inTmux)
	p := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("cockpit error: %w", err)
	}

	m, ok := finalModel.(cockpit.Model)
	if !ok {
		return nil
	}

	// Handle new session request
	if m.WantsNewSession() {
		return runTmuxNew(cmd, nil)
	}

	// Handle attach after TUI exit
	target := m.AttachTarget()
	if target == "" {
		return nil
	}

	if inTmux {
		ui.Infof("Switching to session: %s", target)
		result, err := shell.Run("tmux", "switch-client", "-t", target)
		if err != nil || result.ExitCode != 0 {
			return fmt.Errorf("failed to switch to session %s: %s", target, result.Stderr)
		}
		return nil
	}

	ui.Infof("Attaching to session: %s", target)
	return shell.RunInteractive("tmux", "attach-session", "-t", target)
}
