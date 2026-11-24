package tui

import (
	"fmt"
	"strings"

	"jira-thing/internal/termsafe"
)

func (m model) View() string {
	if m.quitting {
		return ""
	}
	if m.showHelp {
		return m.helpView()
	}
	return m.tableView()
}

func (m model) tableView() string {
	help := "\n  ↑/↓/j/k navigate • g/G top/bottom • enter/space select • s save & quit"
	if m.quickActionsEnabled {
		help += " • v view • o open • m transition • c copy url • ctrl+k copy key"
		if m.fetcher != nil {
			help += " • ctrl+r refresh"
		}
	}
	help += " • ? help • q quit\n"
	status := fmt.Sprintf("  %d ticket(s) selected", len(m.selected))
	if m.refreshing {
		status += "  (refreshing...)"
	} else if m.refreshErr != nil {
		status += fmt.Sprintf("  (refresh failed: %s)", termsafe.Line(m.refreshErr.Error()))
	}
	return "\n" + m.table.View() + "\n" + status + help
}

// helpView lists the table's keys, for the ? overlay.
func (m model) helpView() string {
	lines := []string{
		"↑/↓, j/k          move;  g/G  top/bottom",
		"enter, space      select or unselect a ticket",
		"s                 save the selection and quit",
	}
	if m.quickActionsEnabled {
		lines = append(lines,
			"v                 view the ticket",
			"o                 open it in the browser",
			"m                 move it to another state",
			"c                 copy its URL;  ctrl+k  copy its key")
		if m.fetcher != nil {
			lines = append(lines, "ctrl+r            refresh the list")
		}
	}
	lines = append(lines, "?                 show or hide this help", "q, esc            quit")
	return "\n" + focusedBorder.Padding(0, 1).Render(strings.Join(lines, "\n")) + "\n  ? or esc to close\n"
}
