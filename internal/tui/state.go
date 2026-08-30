package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	stateTableHeight  = 10
	colWidthStateName = 74
)

// TransitionOption is a single workflow transition offered on a ticket.
// Boards customise their workflow states heavily, so this list is always
// read live from Jira rather than assumed to be a fixed set.
type TransitionOption struct {
	ID   string
	Name string
}

// stateModel is the bubbletea model for picking a workflow transition.
type stateModel struct {
	table     table.Model
	options   []TransitionOption
	ticketKey string
	picked    TransitionOption
	done      bool
	cancelled bool
}

// SelectTransition launches a TUI listing the available transitions for ticketKey
// and returns the one the user picks.
func SelectTransition(ticketKey string, options []TransitionOption) (TransitionOption, bool, error) {
	if len(options) == 0 {
		return TransitionOption{}, true, nil
	}
	m := newStateModel(ticketKey, options)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return TransitionOption{}, false, fmt.Errorf("running TUI: %w", err)
	}
	final := result.(stateModel)
	return final.picked, final.cancelled, nil
}

func newStateModel(ticketKey string, options []TransitionOption) stateModel {
	columns := []table.Column{{Title: "Move to", Width: colWidthStateName}}
	rows := make([]table.Row, len(options))
	for i, o := range options {
		rows[i] = table.Row{o.Name}
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(stateTableHeight),
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
	return stateModel{table: t, options: options, ticketKey: ticketKey}
}

func (m stateModel) Init() tea.Cmd { return nil }

func (m stateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "q", "ctrl+c", "esc":
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		case "enter":
			cursor := m.table.Cursor()
			if cursor < len(m.options) {
				m.picked = m.options[cursor]
				m.done = true
				return m, tea.Quit
			}
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m stateModel) View() string {
	if m.done {
		return ""
	}
	heading := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).
		Render(fmt.Sprintf("  %s — choose new state", m.ticketKey))
	help := "\n  ↑/↓ navigate • enter select • q cancel\n"
	return "\n" + heading + "\n\n" + m.table.View() + help
}
