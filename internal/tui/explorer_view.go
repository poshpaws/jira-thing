package tui

import (
	"fmt"
	"strings"

	"jira-thing/internal/termsafe"

	"github.com/charmbracelet/lipgloss"
)

const (
	// sidebarWidth is the width of the group list, in cells.
	sidebarWidth = 34
	// sidebarRows is how many groups the sidebar shows at once.
	sidebarRows = tableHeight + 2
)

var (
	focusedBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("99"))
	blurredBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

// explorerHelp lists every key, for the ? overlay.
var explorerHelp = []string{
	"tab / shift+tab   switch between the list and the issues",
	"↑/↓, j/k          move;  g/G  first/last",
	"enter, →          open the selected item's issues",
	"esc, ←            back to the list (quits from the list)",
	"v                 view the issue",
	"o                 open it in the browser",
	"m                 move it to another state",
	"c                 copy its URL;  ctrl+k  copy its key",
	"r                 reload the open item's issues",
	"?                 show or hide this help",
	"q                 quit",
}

func (m explorerModel) View() string {
	if m.done {
		return ""
	}
	header := HeadingStyle.Render(termsafe.Line(m.title))
	if m.showHelp {
		return header + "\n\n" + focusedBorder.Padding(0, 1).Render(strings.Join(explorerHelp, "\n")) + "\n  ? or esc to close\n"
	}
	panes := lipgloss.JoinHorizontal(lipgloss.Top, m.sidebarView(), m.issuesView())
	return header + "\n" + panes + "\n" + dimStyle.Render("  tab switch pane • v view • o open • m move • r reload • ? help • q quit") + "\n"
}

// sidebarView renders the visible slice of groups, keeping the cursor in view.
func (m explorerModel) sidebarView() string {
	first := max(0, min(m.cursor-sidebarRows/2, len(m.groups)-sidebarRows))
	var lines []string
	for i := first; i < min(len(m.groups), first+sidebarRows); i++ {
		lines = append(lines, m.groupLine(i))
	}
	return m.border(focusGroups).Width(sidebarWidth).Render(strings.Join(lines, "\n"))
}

// groupLine renders one sidebar entry, cut to fit.
func (m explorerModel) groupLine(i int) string {
	g := m.groups[i]
	text := termsafe.Line(g.Title)
	if g.Detail != "" {
		text += " " + dimStyle.Render(termsafe.Line(g.Detail))
	}
	text = lipgloss.NewStyle().MaxWidth(sidebarWidth - 2).Render(text)
	if i == m.cursor {
		return cursorStyle.Render("› ") + text
	}
	return "  " + text
}

// issuesView renders the open group's heading and its issues, or why there
// are none to show yet.
func (m explorerModel) issuesView() string {
	g := m.groups[m.cursor]
	heading := HeadingStyle.Render(termsafe.Line(g.Title))
	var body string
	switch {
	case m.loading[m.cursor]:
		body = dimStyle.Render("Loading…")
	case m.errs[m.cursor] != nil:
		body = ErrorStyle.Render("Couldn't load: " + termsafe.Line(m.errs[m.cursor].Error()))
	case len(m.cache[m.cursor]) == 0:
		body = dimStyle.Render("No issues.")
	default:
		body = m.issues.View() + "\n" + dimStyle.Render(fmt.Sprintf("%d issue(s)", len(m.cache[m.cursor])))
	}
	return m.border(focusIssues).Render(heading + "\n" + body)
}

// border is the pane's frame, highlighted when it has focus.
func (m explorerModel) border(pane explorerFocus) lipgloss.Style {
	if m.focus == pane {
		return focusedBorder
	}
	return blurredBorder
}
