package cockpit

import "github.com/charmbracelet/lipgloss"

// Color constants matching drift's ui/colors.go palette.
const (
	colorCyan    = lipgloss.Color("#00BCD4")
	colorGreen   = lipgloss.Color("#4CAF50")
	colorYellow  = lipgloss.Color("#FFC107")
	colorRed     = lipgloss.Color("#F44336")
	colorBlue    = lipgloss.Color("#2196F3")
	colorDim     = lipgloss.Color("#666666")
	colorWhite   = lipgloss.Color("#FFFFFF")
	colorBg      = lipgloss.Color("#1a1a2e")
	colorBorder  = lipgloss.Color("#333355")
	colorSelect  = lipgloss.Color("#16213e")
)

// Styles holds all lipgloss styles for the dashboard.
type Styles struct {
	Header        lipgloss.Style
	HeaderTitle   lipgloss.Style
	HeaderStat    lipgloss.Style
	ProjectHeader lipgloss.Style
	SessionRow    lipgloss.Style
	SelectedRow   lipgloss.Style
	StatusActive  lipgloss.Style
	StatusIdle    lipgloss.Style
	StatusThink   lipgloss.Style
	StatusUnknown lipgloss.Style
	ClaudeBadge   lipgloss.Style
	AttachedBadge lipgloss.Style
	DimText       lipgloss.Style
	BranchText    lipgloss.Style
	PreviewBorder lipgloss.Style
	PreviewTitle  lipgloss.Style
	Footer        lipgloss.Style
	FooterKey     lipgloss.Style
	FooterDesc    lipgloss.Style
	Overlay       lipgloss.Style
	OverlayTitle  lipgloss.Style
	EmptyState    lipgloss.Style
	PanelLeft     lipgloss.Style
	PanelRight    lipgloss.Style
}

// DefaultStyles returns the default style set.
func DefaultStyles() Styles {
	return Styles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorCyan).
			Padding(0, 1),
		HeaderTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorCyan),
		HeaderStat: lipgloss.NewStyle().
			Foreground(colorDim),
		ProjectHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBlue),
		SessionRow: lipgloss.NewStyle().
			Padding(0, 1),
		SelectedRow: lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(colorWhite).
			Background(colorSelect),
		StatusActive: lipgloss.NewStyle().
			Foreground(colorGreen),
		StatusIdle: lipgloss.NewStyle().
			Foreground(colorDim),
		StatusThink: lipgloss.NewStyle().
			Foreground(colorYellow),
		StatusUnknown: lipgloss.NewStyle().
			Foreground(colorDim),
		ClaudeBadge: lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true),
		AttachedBadge: lipgloss.NewStyle().
			Foreground(colorGreen).
			Faint(true),
		DimText: lipgloss.NewStyle().
			Foreground(colorDim),
		BranchText: lipgloss.NewStyle().
			Foreground(colorCyan),
		PreviewBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1),
		PreviewTitle: lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true),
		Footer: lipgloss.NewStyle().
			Foreground(colorDim).
			Padding(0, 1),
		FooterKey: lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true),
		FooterDesc: lipgloss.NewStyle().
			Foreground(colorDim),
		Overlay: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorRed).
			Padding(1, 3).
			Align(lipgloss.Center),
		OverlayTitle: lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true),
		EmptyState: lipgloss.NewStyle().
			Foreground(colorDim).
			Align(lipgloss.Center).
			Padding(2, 0),
		PanelLeft: lipgloss.NewStyle().
			Padding(0, 1),
		PanelRight: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1),
	}
}
