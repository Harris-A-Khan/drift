package cockpit

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/pkg/shell"
)

// cmdTimeout is the max time any single tmux/git command can take before being killed.
const cmdTimeout = 2 * time.Second

// DiscoverAll discovers all tmux sessions and groups them by project.
func DiscoverAll() DashboardState {
	sessions, err := listAllSessions()
	if err != nil {
		return DashboardState{Error: err}
	}

	// Enrich each session with git/project context
	for _, s := range sessions {
		enrichSession(s)
	}

	// Group by project root
	state := groupByProject(sessions)
	return state
}

// listAllSessions enumerates all tmux sessions with basic info.
func listAllSessions() ([]*ClaudeSession, error) {
	result, err := shell.RunWithTimeout(cmdTimeout, "tmux", "list-sessions", "-F",
		"#{session_name}|#{session_windows}|#{session_attached}")
	if err != nil || result.ExitCode != 0 {
		return nil, nil // no sessions or tmux not running
	}

	var sessions []*ClaudeSession
	for _, line := range strings.Split(result.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}

		s := &ClaudeSession{
			ID:       parts[0],
			TmuxName: parts[0],
			IsAttached: parts[2] == "1",
		}

		fmt.Sscanf(parts[1], "%d", &s.WindowCount)

		// Get the active pane's working directory
		s.WorkingDir = getPaneWorkingDir(s.TmuxName)

		// Check if Claude Code is running
		s.HasClaude = checkHasClaude(s.TmuxName)

		// Detect status from pane content
		if s.HasClaude {
			s.Status = detectSessionStatus(s.TmuxName)
		}

		sessions = append(sessions, s)
	}

	return sessions, nil
}

// getPaneWorkingDir returns the working directory of the active pane.
func getPaneWorkingDir(sessionName string) string {
	result, err := shell.RunWithTimeout(cmdTimeout, "tmux", "display-message", "-t", sessionName, "-p", "#{pane_current_path}")
	if err != nil || result.ExitCode != 0 {
		return ""
	}
	return strings.TrimSpace(result.Stdout)
}

// checkHasClaude checks if Claude Code is running in any pane of the session.
func checkHasClaude(sessionName string) bool {
	// Check pane commands
	result, err := shell.RunWithTimeout(cmdTimeout, "tmux", "list-panes", "-t", sessionName, "-F", "#{pane_current_command}")
	if err == nil && result.ExitCode == 0 {
		for _, line := range strings.Split(result.Stdout, "\n") {
			cmd := strings.TrimSpace(strings.ToLower(line))
			if cmd == "claude" || strings.Contains(cmd, "claude") {
				return true
			}
		}
	}

	// Check pane titles
	result, err = shell.RunWithTimeout(cmdTimeout, "tmux", "list-panes", "-t", sessionName, "-F", "#{pane_title}")
	if err == nil && result.ExitCode == 0 {
		for _, line := range strings.Split(result.Stdout, "\n") {
			title := strings.TrimSpace(line)
			if strings.Contains(title, "Claude") || strings.Contains(title, "claude") {
				return true
			}
		}
	}

	return false
}

// detectSessionStatus parses the last few lines of pane output to determine status.
func detectSessionStatus(sessionName string) SessionStatus {
	result, err := shell.RunWithTimeout(cmdTimeout, "tmux", "capture-pane", "-t", sessionName, "-p", "-l", "5")
	if err != nil || result.ExitCode != 0 {
		return StatusUnknown
	}

	lines := strings.Split(result.Stdout, "\n")

	// Check lines from bottom up for status indicators
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		lower := strings.ToLower(line)

		// Thinking indicators
		if strings.Contains(lower, "thinking") ||
			strings.Contains(lower, "reasoning") ||
			strings.Contains(line, "...") && !strings.Contains(line, ">") {
			return StatusThinking
		}

		// Idle prompt indicators (Claude shows > or $ at prompt)
		if strings.HasPrefix(line, ">") ||
			strings.HasPrefix(line, "$") ||
			strings.Contains(lower, "waiting for input") ||
			strings.HasSuffix(line, "> ") {
			return StatusIdle
		}

		// If we see content being produced, it's active
		return StatusActive
	}

	return StatusUnknown
}

// enrichSession adds git and project context to a session.
func enrichSession(s *ClaudeSession) {
	if s.WorkingDir == "" {
		return
	}

	// Get git branch
	s.GitBranch = gitBranchFromDir(s.WorkingDir)

	// Check if it's a worktree
	s.IsWorktree = isGitWorktreeDir(s.WorkingDir)

	// Find project config
	resolveProjectContext(s)
}

// resolveProjectContext finds the .drift.yaml and extracts project info.
func resolveProjectContext(s *ClaudeSession) {
	// Try to find .drift.yaml from the working directory
	configPath, err := config.FindConfigFileFromDir(s.WorkingDir)
	if err == nil {
		s.ProjectRoot = filepath.Dir(configPath)

		// Load project name from config
		cfg, loadErr := config.LoadFromPath(configPath)
		if loadErr == nil && cfg.Project.Name != "" {
			s.ProjectName = cfg.Project.Name
			return
		}
	}

	// For worktrees, also check the main worktree path
	if s.IsWorktree {
		mainRoot := gitMainWorktreeDir(s.WorkingDir)
		if mainRoot != "" && mainRoot != s.WorkingDir {
			configPath, err = config.FindConfigFileFromDir(mainRoot)
			if err == nil {
				s.ProjectRoot = filepath.Dir(configPath)

				cfg, loadErr := config.LoadFromPath(configPath)
				if loadErr == nil && cfg.Project.Name != "" {
					s.ProjectName = cfg.Project.Name
					return
				}
			}
		}
	}

	// Fall back to git repo root basename
	gitRoot := gitRootFromDir(s.WorkingDir)
	if gitRoot != "" {
		s.ProjectRoot = gitRoot
		s.ProjectName = filepath.Base(gitRoot)
		return
	}

	// Last resort: use the working directory basename
	s.ProjectName = filepath.Base(s.WorkingDir)
}

// groupByProject groups sessions by their project root.
func groupByProject(sessions []*ClaudeSession) DashboardState {
	state := DashboardState{
		TotalCount: len(sessions),
	}

	groups := make(map[string]*ProjectGroup)

	for _, s := range sessions {
		if s.HasClaude {
			state.ClaudeCount++
		}

		if s.ProjectRoot == "" {
			state.Ungrouped = append(state.Ungrouped, s)
			continue
		}

		group, exists := groups[s.ProjectRoot]
		if !exists {
			group = &ProjectGroup{
				Name:     s.ProjectName,
				RootPath: s.ProjectRoot,
			}
			groups[s.ProjectRoot] = group
		}
		group.Sessions = append(group.Sessions, s)
	}

	// Convert map to sorted slice
	for _, group := range groups {
		state.Projects = append(state.Projects, group)
	}
	sort.Slice(state.Projects, func(i, j int) bool {
		return state.Projects[i].Name < state.Projects[j].Name
	})

	return state
}

// CapturePane captures the last N lines of a tmux pane for preview.
func CapturePane(sessionName string, lines int) string {
	result, err := shell.RunWithTimeout(cmdTimeout, "tmux", "capture-pane", "-t", sessionName,
		"-p", "-l", fmt.Sprintf("%d", lines))
	if err != nil || result.ExitCode != 0 {
		return ""
	}
	return result.Stdout
}

// KillSession kills a tmux session by name.
func KillSession(sessionName string) error {
	result, err := shell.RunWithTimeout(cmdTimeout, "tmux", "kill-session", "-t", sessionName)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to kill session: %s", result.Stderr)
	}
	return nil
}

// gitBranchFromDir returns the current git branch for a directory.
func gitBranchFromDir(dir string) string {
	result, err := shell.RunWithTimeout(cmdTimeout, "git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || result.ExitCode != 0 {
		return ""
	}
	return strings.TrimSpace(result.Stdout)
}

// gitRootFromDir returns the git repository root for a directory.
func gitRootFromDir(dir string) string {
	result, err := shell.RunWithTimeout(cmdTimeout, "git", "-C", dir, "rev-parse", "--show-toplevel")
	if err != nil || result.ExitCode != 0 {
		return ""
	}
	return strings.TrimSpace(result.Stdout)
}

// isGitWorktreeDir checks if a directory is a git worktree (not the main working tree).
func isGitWorktreeDir(dir string) bool {
	gitDir, err := shell.RunWithTimeout(cmdTimeout, "git", "-C", dir, "rev-parse", "--git-dir")
	if err != nil || gitDir.ExitCode != 0 {
		return false
	}
	commonDir, err := shell.RunWithTimeout(cmdTimeout, "git", "-C", dir, "rev-parse", "--git-common-dir")
	if err != nil || commonDir.ExitCode != 0 {
		return false
	}

	gd := strings.TrimSpace(gitDir.Stdout)
	cd := strings.TrimSpace(commonDir.Stdout)
	return gd != cd
}

// gitMainWorktreeDir returns the main worktree directory for a git worktree.
func gitMainWorktreeDir(dir string) string {
	result, err := shell.RunWithTimeout(cmdTimeout, "git", "-C", dir, "rev-parse", "--git-common-dir")
	if err != nil || result.ExitCode != 0 {
		return ""
	}
	commonDir := strings.TrimSpace(result.Stdout)
	// git-common-dir returns the .git directory of the main worktree
	// Strip the trailing /.git to get the worktree root
	return filepath.Dir(commonDir)
}
