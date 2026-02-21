// Package cockpit implements the live TUI dashboard for managing Claude Code sessions.
package cockpit

// SessionStatus represents the detected status of a Claude session.
type SessionStatus int

const (
	StatusUnknown  SessionStatus = iota
	StatusActive                 // Claude is actively producing output
	StatusIdle                   // Claude prompt is waiting for input
	StatusThinking               // Claude is thinking/processing
)

// String returns the display string for a session status.
func (s SessionStatus) String() string {
	switch s {
	case StatusActive:
		return "active"
	case StatusIdle:
		return "idle"
	case StatusThinking:
		return "thinking"
	default:
		return "unknown"
	}
}

// ClaudeSession represents a discovered tmux session that may have Claude Code running.
type ClaudeSession struct {
	ID          string        // unique identifier (tmux session name)
	TmuxName    string        // tmux session name
	WorkingDir  string        // from tmux #{pane_current_path}
	GitBranch   string        // git branch in that directory
	IsWorktree  bool          // whether the directory is a git worktree
	ProjectName string        // from .drift.yaml or fallback to dir name
	ProjectRoot string        // directory containing .drift.yaml
	Status      SessionStatus // parsed from pane content
	IsAttached  bool          // whether a client is attached
	WindowCount int           // number of tmux windows
	HasClaude   bool          // whether Claude Code is running
	PanePreview string        // last N lines from capture-pane
}

// ProjectGroup represents a group of sessions sharing the same project root.
type ProjectGroup struct {
	Name     string
	RootPath string
	Sessions []*ClaudeSession
}

// DashboardState holds the complete discovery result for a single refresh cycle.
type DashboardState struct {
	Projects    []*ProjectGroup
	Ungrouped   []*ClaudeSession // sessions without a project
	TotalCount  int
	ClaudeCount int
	Error       error
}
