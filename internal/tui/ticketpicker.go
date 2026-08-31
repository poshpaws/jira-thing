package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	pickerTableHeight = 15
	manualEntryLabel  = "✎  Type a ticket key manually…"
)

// PickTicketResult is the outcome of PickTicket: a chosen ticket key, a request
// to type a key manually (Manual), or Cancelled.
type PickTicketResult struct {
	Key       string
	Manual    bool
	Cancelled bool
}

// pickerModel is the bubbletea model for PickTicket.
type pickerModel struct {
	table   table.Model
	tickets []Ticket
	result  PickTicketResult
	done    bool
}

// PickTicket shows tickets (typically the user's open tasks) in a single-select
// TUI, with a "type a key manually" option always offered first for tickets not
// in the list.
func PickTicket(tickets []Ticket) (PickTicketResult, error) {
	m := newPickerModel(tickets)
	p := tea.NewProgram(m)
	res, err := p.Run()
	if err != nil {
		return PickTicketResult{}, fmt.Errorf("running TUI: %w", err)
	}
	final := res.(pickerModel)
	return final.result, nil
}

func newPickerModel(tickets []Ticket) pickerModel {
	columns := []table.Column{
		{Title: "Key", Width: colWidthKey},
		{Title: "Status", Width: colWidthStat},
		{Title: "Summary", Width: colWidthSum},
	}
	rows := make([]table.Row, 0, len(tickets)+1)
	rows = append(rows, table.Row{"", "", manualEntryLabel})
	for _, t := range tickets {
		rows = append(rows, table.Row{t.Key, t.Status, t.Summary})
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(pickerTableHeight),
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
	return pickerModel{table: t, tickets: tickets}
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "q", "ctrl+c", "esc":
			m.result.Cancelled = true
			m.done = true
			return m, tea.Quit
		case "enter":
			m.pickCursor()
			m.done = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// pickCursor resolves the row under the cursor into the final result: the
// manual-entry row (index 0) or one of the ticket rows.
func (m *pickerModel) pickCursor() {
	cursor := m.table.Cursor()
	if cursor == 0 {
		m.result.Manual = true
		return
	}
	if idx := cursor - 1; idx < len(m.tickets) {
		m.result.Key = m.tickets[idx].Key
	}
}

func (m pickerModel) View() string {
	if m.done {
		return ""
	}
	heading := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).
		Render("  Pick a ticket")
	help := "\n  ↑/↓/j/k navigate • enter select • q cancel\n"
	return "\n" + heading + "\n\n" + m.table.View() + help
}
