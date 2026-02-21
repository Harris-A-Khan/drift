package cockpit

import (
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	defaultPollInterval = 3 * time.Second
	previewCaptureLines = 30
)

// Messages for the bubbletea event loop.
type (
	tickMsg        struct{}
	refreshDoneMsg struct{ state DashboardState }
	panePreviewMsg struct {
		sessionID string
		content   string
	}
)

// Model is the bubbletea model for the cockpit dashboard.
type Model struct {
	state         DashboardState
	flatSessions  []*ClaudeSession
	cursor        int
	expandedProjs map[string]bool
	preview       viewport.Model
	keys          KeyMap
	styles        Styles
	width, height int
	showHelp      bool
	confirmKill   bool
	filterClaude  bool
	attachTarget  string
	newSession    bool // flag to create new session after quit
	inTmux        bool
	loading       bool
	refreshing    bool // guards against overlapping refresh cycles
	pollInterval  time.Duration
}

// NewModel creates a new cockpit dashboard model.
func NewModel(inTmux bool) Model {
	vp := viewport.New(0, 0)

	return Model{
		expandedProjs: make(map[string]bool),
		preview:       vp,
		keys:          DefaultKeyMap(),
		styles:        DefaultStyles(),
		inTmux:        inTmux,
		loading:       true,
		pollInterval:  defaultPollInterval,
	}
}

// AttachTarget returns the session to attach to after the TUI exits.
func (m Model) AttachTarget() string {
	return m.attachTarget
}

// WantsNewSession returns whether the user requested a new session.
func (m Model) WantsNewSession() bool {
	return m.newSession
}

// Init starts the initial refresh. The tick timer starts after the first refresh completes.
func (m Model) Init() tea.Cmd {
	m.refreshing = true
	return refreshCmd()
}

// Update handles all messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.preview.Width = m.rightWidth()
		m.preview.Height = m.contentHeight() - 2 // leave room for title
		return m, nil

	case tickMsg:
		// Only start a refresh if one isn't already running
		if m.refreshing {
			return m, tickCmd(m.pollInterval)
		}
		m.refreshing = true
		return m, refreshCmd()

	case refreshDoneMsg:
		m.state = msg.state
		m.loading = false
		m.refreshing = false

		// Auto-expand all projects on first load
		if len(m.expandedProjs) == 0 {
			for _, proj := range m.state.Projects {
				m.expandedProjs[proj.RootPath] = true
			}
		}

		m.rebuildFlatSessions()

		// Clamp cursor
		if m.cursor >= len(m.flatSessions) {
			m.cursor = max(0, len(m.flatSessions)-1)
		}

		// Capture preview for selected session, then restart the poll timer
		cmds := []tea.Cmd{tickCmd(m.pollInterval)}
		if selected := m.selectedSession(); selected != nil {
			cmds = append(cmds, capturePaneCmd(selected.TmuxName))
		}
		return m, tea.Batch(cmds...)

	case panePreviewMsg:
		// Only update if still viewing this session
		if selected := m.selectedSession(); selected != nil && selected.TmuxName == msg.sessionID {
			m.preview.SetContent(msg.content)
			m.preview.GotoBottom()
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// handleKey processes keyboard input.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Kill confirmation mode
	if m.confirmKill {
		switch msg.String() {
		case "y":
			m.confirmKill = false
			if selected := m.selectedSession(); selected != nil {
				_ = KillSession(selected.TmuxName)
				return m, refreshCmd()
			}
			return m, nil
		case "n", "esc":
			m.confirmKill = false
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "j", "down":
		if m.cursor < len(m.flatSessions)-1 {
			m.cursor++
			if selected := m.selectedSession(); selected != nil {
				return m, capturePaneCmd(selected.TmuxName)
			}
		}
		return m, nil

	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			if selected := m.selectedSession(); selected != nil {
				return m, capturePaneCmd(selected.TmuxName)
			}
		}
		return m, nil

	case "enter":
		if selected := m.selectedSession(); selected != nil {
			m.attachTarget = selected.TmuxName
			return m, tea.Quit
		}
		return m, nil

	case "n":
		m.newSession = true
		return m, tea.Quit

	case "x":
		if selected := m.selectedSession(); selected != nil {
			m.confirmKill = true
		}
		return m, nil

	case "c":
		m.filterClaude = !m.filterClaude
		m.rebuildFlatSessions()
		if m.cursor >= len(m.flatSessions) {
			m.cursor = max(0, len(m.flatSessions)-1)
		}
		return m, nil

	case "tab":
		m.toggleExpandAtCursor()
		m.rebuildFlatSessions()
		return m, nil

	case "?":
		m.showHelp = !m.showHelp
		return m, nil
	}

	return m, nil
}

// View renders the full dashboard.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if m.confirmKill {
		return m.renderConfirmOverlay()
	}

	header := m.renderHeader()
	footer := m.renderFooter()

	left := m.renderProjectTree()
	right := m.renderPreview()

	content := lipgloss.JoinHorizontal(lipgloss.Top,
		m.styles.PanelLeft.Width(m.leftWidth()).Height(m.contentHeight()).Render(left),
		right,
	)

	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

// selectedSession returns the currently selected session or nil.
func (m Model) selectedSession() *ClaudeSession {
	if m.cursor < 0 || m.cursor >= len(m.flatSessions) {
		return nil
	}
	return m.flatSessions[m.cursor]
}

// rebuildFlatSessions creates a flat list of navigable sessions from the grouped state.
func (m *Model) rebuildFlatSessions() {
	m.flatSessions = nil

	for _, proj := range m.state.Projects {
		expanded := m.expandedProjs[proj.RootPath]
		if !expanded {
			continue
		}
		for _, session := range proj.Sessions {
			if m.filterClaude && !session.HasClaude {
				continue
			}
			m.flatSessions = append(m.flatSessions, session)
		}
	}

	for _, session := range m.state.Ungrouped {
		if m.filterClaude && !session.HasClaude {
			continue
		}
		m.flatSessions = append(m.flatSessions, session)
	}
}

// toggleExpandAtCursor toggles the project group that the cursor is currently in.
func (m *Model) toggleExpandAtCursor() {
	selected := m.selectedSession()
	if selected == nil {
		// If no session selected, toggle the first project
		if len(m.state.Projects) > 0 {
			root := m.state.Projects[0].RootPath
			m.expandedProjs[root] = !m.expandedProjs[root]
		}
		return
	}

	// Find which project this session belongs to
	for _, proj := range m.state.Projects {
		for _, s := range proj.Sessions {
			if s.TmuxName == selected.TmuxName {
				m.expandedProjs[proj.RootPath] = !m.expandedProjs[proj.RootPath]
				return
			}
		}
	}
}

// Commands

func tickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func refreshCmd() tea.Cmd {
	return func() tea.Msg {
		state := DiscoverAll()
		return refreshDoneMsg{state: state}
	}
}

func capturePaneCmd(sessionName string) tea.Cmd {
	return func() tea.Msg {
		content := CapturePane(sessionName, previewCaptureLines)
		return panePreviewMsg{
			sessionID: sessionName,
			content:   content,
		}
	}
}
