package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	menuTableHeight = 20
	colWidthAction  = 28
	colWidthDesc    = 52
)

// MenuOption is a single selectable entry in the main jira-thing menu.
type MenuOption struct {
	Label string
	Desc  string
}

// menuModel is the bubbletea model for the top-level menu.
type menuModel struct {
	table     table.Model
	options   []MenuOption
	picked    int
	done      bool
	cancelled bool
}

// SelectMenuOption launches the jira-thing home menu and returns the index of
// the option the user picked, or cancelled=true if they quit without picking.
func SelectMenuOption(title string, options []MenuOption) (int, bool, error) {
	m := newMenuModel(title, options)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return 0, false, fmt.Errorf("running TUI: %w", err)
	}
	final := result.(menuModel)
	return final.picked, final.cancelled, nil
}

func newMenuModel(_ string, options []MenuOption) menuModel {
	columns := []table.Column{
		{Title: "Action", Width: colWidthAction},
		{Title: "Description", Width: colWidthDesc},
	}
	rows := make([]table.Row, len(options))
	for i, o := range options {
		rows[i] = table.Row{o.Label, o.Desc}
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(menuTableHeight),
	)
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57"))
	t.SetStyles(s)
	return menuModel{table: t, options: options}
}

func (m menuModel) Init() tea.Cmd { return nil }

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "q", "ctrl+c", "esc":
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		case "enter":
			cursor := m.table.Cursor()
			if cursor < len(m.options) {
				m.picked = cursor
				m.done = true
				return m, tea.Quit
			}
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m menuModel) View() string {
	if m.done {
		return ""
	}
	heading := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).
		Render("  jira-thing menu")
	help := "\n  ↑/↓ navigate • enter select • q quit\n"
	return "\n" + heading + "\n\n" + m.table.View() + help
}

// FormatMenuOptions is a plain-text fallback rendering, useful for tests.
func FormatMenuOptions(options []MenuOption) string {
	var sb strings.Builder
	for i, o := range options {
		fmt.Fprintf(&sb, "%d. %s — %s\n", i+1, o.Label, o.Desc)
	}
	return sb.String()
}
