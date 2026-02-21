package cockpit

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderHeader renders the top title bar with aggregate stats.
func (m Model) renderHeader() string {
	s := m.styles

	title := s.HeaderTitle.Render("DRIFT COCKPIT")

	projectCount := len(m.state.Projects)
	if len(m.state.Ungrouped) > 0 {
		projectCount++
	}

	stats := s.HeaderStat.Render(fmt.Sprintf(
		"  %d projects  |  %d sessions  |  %d claude",
		projectCount, m.state.TotalCount, m.state.ClaudeCount,
	))

	bar := lipgloss.JoinHorizontal(lipgloss.Center, title, stats)

	divider := s.DimText.Render(strings.Repeat("─", m.width))

	return bar + "\n" + divider
}

// renderProjectTree renders the left panel with project groups and sessions.
func (m Model) renderProjectTree() string {
	s := m.styles
	var b strings.Builder

	if m.state.TotalCount == 0 {
		empty := s.EmptyState.Width(m.leftWidth()).Render(
			"No tmux sessions found\n\n" +
				s.DimText.Render("Press ") +
				s.FooterKey.Render("n") +
				s.DimText.Render(" to create one, or use ") +
				s.FooterKey.Render("drift tmux new"))
		return empty
	}

	flatIdx := 0

	// Render project groups
	for _, proj := range m.state.Projects {
		expanded := m.expandedProjs[proj.RootPath]

		arrow := ">"
		if expanded {
			arrow = "v"
		}

		header := fmt.Sprintf("%s %s (%d)", arrow, proj.Name, len(proj.Sessions))
		b.WriteString(s.ProjectHeader.Render(header))
		b.WriteString("\n")

		if expanded {
			for _, session := range proj.Sessions {
				if m.filterClaude && !session.HasClaude {
					flatIdx++
					continue
				}
				b.WriteString(m.renderSessionRow(session, flatIdx))
				b.WriteString("\n")
				flatIdx++
			}
		} else {
			// Skip over collapsed sessions in flat index
			for _, session := range proj.Sessions {
				if m.filterClaude && !session.HasClaude {
					flatIdx++
					continue
				}
				flatIdx++
			}
		}
	}

	// Render ungrouped sessions
	if len(m.state.Ungrouped) > 0 {
		header := fmt.Sprintf("  Ungrouped (%d)", len(m.state.Ungrouped))
		b.WriteString(s.DimText.Render(header))
		b.WriteString("\n")

		for _, session := range m.state.Ungrouped {
			if m.filterClaude && !session.HasClaude {
				flatIdx++
				continue
			}
			b.WriteString(m.renderSessionRow(session, flatIdx))
			b.WriteString("\n")
			flatIdx++
		}
	}

	return b.String()
}

// renderSessionRow renders a single session line.
func (m Model) renderSessionRow(session *ClaudeSession, idx int) string {
	s := m.styles
	isSelected := idx == m.cursor

	// Build the row content
	var parts []string

	// Cursor indicator
	if isSelected {
		parts = append(parts, "*")
	} else {
		parts = append(parts, " ")
	}

	// Session name / branch
	name := session.GitBranch
	if name == "" {
		name = session.TmuxName
	}

	if session.HasClaude {
		parts = append(parts, s.ClaudeBadge.Render(name))
	} else {
		parts = append(parts, s.DimText.Render(name))
	}

	// Status badge
	parts = append(parts, m.renderStatusBadge(session.Status))

	// Attached indicator
	if session.IsAttached {
		parts = append(parts, s.AttachedBadge.Render("attached"))
	}

	row := "  " + strings.Join(parts, "  ")

	if isSelected {
		return s.SelectedRow.Width(m.leftWidth()).Render(row)
	}
	return s.SessionRow.Render(row)
}

// renderStatusBadge returns a colored status indicator.
func (m Model) renderStatusBadge(status SessionStatus) string {
	s := m.styles
	switch status {
	case StatusActive:
		return s.StatusActive.Render("active")
	case StatusIdle:
		return s.StatusIdle.Render("idle")
	case StatusThinking:
		return s.StatusThink.Render("thinking")
	default:
		return s.StatusUnknown.Render("--")
	}
}

// renderPreview renders the right panel with the pane content preview.
func (m Model) renderPreview() string {
	s := m.styles

	if len(m.flatSessions) == 0 {
		return s.PanelRight.
			Width(m.rightWidth()).
			Height(m.contentHeight()).
			Render(s.DimText.Render("No sessions to preview"))
	}

	// Get the selected session
	selected := m.selectedSession()
	if selected == nil {
		return s.PanelRight.
			Width(m.rightWidth()).
			Height(m.contentHeight()).
			Render(s.DimText.Render("No session selected"))
	}

	// Title
	title := s.PreviewTitle.Render(fmt.Sprintf(" %s ", selected.TmuxName))

	// Preview content from viewport
	content := m.preview.View()

	return s.PanelRight.
		Width(m.rightWidth()).
		Height(m.contentHeight()).
		Render(title + "\n" + content)
}

// renderFooter renders the keybinding help bar at the bottom.
func (m Model) renderFooter() string {
	s := m.styles

	if m.confirmKill {
		return s.Footer.Render(
			s.OverlayTitle.Render("Kill session? ") +
				s.FooterKey.Render("y") + s.FooterDesc.Render(" confirm  ") +
				s.FooterKey.Render("n/esc") + s.FooterDesc.Render(" cancel"),
		)
	}

	bindings := []string{
		s.FooterKey.Render("j/k") + " " + s.FooterDesc.Render("navigate"),
		s.FooterKey.Render("enter") + " " + s.FooterDesc.Render("attach"),
		s.FooterKey.Render("n") + " " + s.FooterDesc.Render("new"),
		s.FooterKey.Render("x") + " " + s.FooterDesc.Render("kill"),
		s.FooterKey.Render("c") + " " + s.FooterDesc.Render("claude"),
		s.FooterKey.Render("tab") + " " + s.FooterDesc.Render("expand"),
		s.FooterKey.Render("q") + " " + s.FooterDesc.Render("quit"),
	}

	filterIndicator := ""
	if m.filterClaude {
		filterIndicator = s.ClaudeBadge.Render(" [claude only]")
	}

	divider := s.DimText.Render(strings.Repeat("─", m.width))
	return divider + "\n" + s.Footer.Render(strings.Join(bindings, "  ")) + filterIndicator
}

// renderConfirmOverlay renders a centered kill confirmation dialog.
func (m Model) renderConfirmOverlay() string {
	selected := m.selectedSession()
	if selected == nil {
		return ""
	}

	s := m.styles
	content := s.OverlayTitle.Render("Kill Session") + "\n\n" +
		fmt.Sprintf("Kill session %s?", s.ClaudeBadge.Render(selected.TmuxName)) + "\n\n" +
		s.FooterKey.Render("y") + " confirm  " +
		s.FooterKey.Render("n") + " cancel"

	overlay := s.Overlay.Render(content)

	// Center in the terminal
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay)
}

// leftWidth returns the width of the left panel.
func (m Model) leftWidth() int {
	return int(float64(m.width) * 0.4)
}

// rightWidth returns the width of the right panel.
func (m Model) rightWidth() int {
	return m.width - m.leftWidth() - 4 // account for borders and padding
}

// contentHeight returns the usable content height.
func (m Model) contentHeight() int {
	return m.height - 5 // header + footer + padding
}
