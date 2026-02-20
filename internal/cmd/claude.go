package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/claude"
	"github.com/undrift/drift/internal/ui"
)

var claudeCmd = &cobra.Command{
	Use:   "claude",
	Short: "Claude Code integration",
	Long:  `Manage Claude Code skill commands for drift workflows.`,
}

var claudeInstallForce bool

var claudeInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install drift skills into .claude/commands/",
	Long: `Install embedded drift workflow skills as Claude Code project commands.

Skills are written to .claude/commands/ and become available as /project:<name>
slash commands in Claude Code.

By default, existing files are not overwritten. Use --force to update all files.`,
	RunE: runClaudeInstall,
}

var claudeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed Claude Code skills",
	Long:  `Show all skills in .claude/commands/ and indicate which are drift-shipped.`,
	RunE:  runClaudeList,
}

func init() {
	claudeInstallCmd.Flags().BoolVarP(&claudeInstallForce, "force", "f", false, "Overwrite existing skill files")
	claudeCmd.AddCommand(claudeInstallCmd)
	claudeCmd.AddCommand(claudeListCmd)
	rootCmd.AddCommand(claudeCmd)
}

func runClaudeInstall(cmd *cobra.Command, args []string) error {
	commands, err := claude.ListCommands()
	if err != nil {
		return fmt.Errorf("failed to list embedded commands: %w", err)
	}

	destDir := filepath.Join(".claude", "commands")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", destDir, err)
	}

	var installed, skipped int

	for _, c := range commands {
		destPath := filepath.Join(destDir, c.Filename)

		if !claudeInstallForce {
			if _, err := os.Stat(destPath); err == nil {
				ui.Infof("Skipped: %s (already exists)", c.Name)
				skipped++
				continue
			}
		}

		data, err := claude.ReadCommand(c.Name)
		if err != nil {
			return fmt.Errorf("failed to read embedded command %s: %w", c.Name, err)
		}

		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", destPath, err)
		}

		if claudeInstallForce {
			ui.Infof("Updated: %s", c.Name)
		} else {
			ui.Infof("Installed: %s", c.Name)
		}
		installed++
	}

	ui.NewLine()
	if installed > 0 && skipped > 0 {
		ui.Success(fmt.Sprintf("Installed %d skills, skipped %d", installed, skipped))
	} else if installed > 0 {
		ui.Success(fmt.Sprintf("Installed %d skills", installed))
	} else {
		ui.Info("All skills already installed (use --force to update)")
	}

	ui.Infof("Skills are available as /project:<name> in Claude Code")
	ui.Infof("Consider committing .claude/commands/ to share with your team")

	return nil
}

func runClaudeList(cmd *cobra.Command, args []string) error {
	destDir := filepath.Join(".claude", "commands")

	// Get the list of drift-shipped command names
	embedded, err := claude.ListCommands()
	if err != nil {
		return fmt.Errorf("failed to list embedded commands: %w", err)
	}
	driftNames := make(map[string]bool, len(embedded))
	for _, c := range embedded {
		driftNames[c.Filename] = true
	}

	// Read the installed directory
	entries, err := os.ReadDir(destDir)
	if err != nil {
		if os.IsNotExist(err) {
			ui.Info("No skills installed. Run 'drift claude install' to get started.")
			return nil
		}
		return fmt.Errorf("failed to read %s: %w", destDir, err)
	}

	ui.Header("Installed Claude Code Skills")

	var count int
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		source := "custom"

		if driftNames[entry.Name()] {
			// Check if the file content still has the drift marker
			content, readErr := os.ReadFile(filepath.Join(destDir, entry.Name()))
			if readErr == nil && claude.IsDriftSkill(content) {
				source = "drift"
			}
		}

		label := fmt.Sprintf("/project:%-25s [%s]", name, source)
		fmt.Println("  " + label)
		count++
	}

	if count == 0 {
		ui.Info("No skills found. Run 'drift claude install' to get started.")
	} else {
		ui.NewLine()
		ui.Infof("%d skill(s) installed", count)
	}

	return nil
}
