package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	colKey       = 0
	tableHeight  = 15
	colWidthKey  = 14
	colWidthStat = 14
	colWidthPri  = 8
	colWidthDate = 12
	colWidthSum  = 50
)

// Ticket represents a Jira issue for display and selection.
type Ticket struct {
	Key      string
	Status   string
	Priority string
	Updated  string
	Summary  string
}

// model is the bubbletea model for the ticket table.
type model struct {
	table    table.Model
	tickets  []Ticket
	selected []Ticket
	quitting bool
}

// ShowTable launches the interactive table TUI. Returns selected tickets.
func ShowTable(tickets []Ticket) ([]Ticket, error) {
	if len(tickets) == 0 {
		fmt.Println("No tickets to display.")
		return nil, nil
	}

	m := newModel(tickets)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("running TUI: %w", err)
	}

	final := result.(model)
	return final.selected, nil
}

func newModel(tickets []Ticket) model {
	columns := []table.Column{
		{Title: "Key", Width: colWidthKey},
		{Title: "Status", Width: colWidthStat},
		{Title: "Priority", Width: colWidthPri},
		{Title: "Updated", Width: colWidthDate},
		{Title: "Summary", Width: colWidthSum},
	}

	rows := make([]table.Row, len(tickets))
	for i, t := range tickets {
		rows[i] = table.Row{t.Key, t.Status, t.Priority, truncDate(t.Updated), t.Summary}
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(tableHeight),
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

	return model{table: t, tickets: tickets}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter", " ":
			m.toggleSelection()
		case "s":
			m.quitting = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *model) toggleSelection() {
	cursor := m.table.Cursor()
	if cursor >= len(m.tickets) {
		return
	}
	ticket := m.tickets[cursor]
	if idx := m.findSelected(ticket.Key); idx >= 0 {
		m.selected = append(m.selected[:idx], m.selected[idx+1:]...)
		m.unmarkRow(cursor)
	} else {
		m.selected = append(m.selected, ticket)
		m.markRow(cursor)
	}
}

func (m model) findSelected(key string) int {
	for i, t := range m.selected {
		if t.Key == key {
			return i
		}
	}
	return -1
}

func (m *model) markRow(idx int) {
	rows := m.table.Rows()
	if idx < len(rows) {
		rows[idx][colKey] = "✓ " + m.tickets[idx].Key
		m.table.SetRows(rows)
	}
}

func (m *model) unmarkRow(idx int) {
	rows := m.table.Rows()
	if idx < len(rows) {
		rows[idx][colKey] = m.tickets[idx].Key
		m.table.SetRows(rows)
	}
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	return m.tableView()
}

func (m model) tableView() string {
	help := "\n  ↑/↓ navigate • enter/space select • s save & quit • q quit\n"
	status := fmt.Sprintf("  %d ticket(s) selected", len(m.selected))
	return "\n" + m.table.View() + "\n" + status + help
}

// truncDate returns the first 10 chars of a date string.
func truncDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// FormatSelected prints the selected tickets as a summary.
func FormatSelected(tickets []Ticket) string {
	if len(tickets) == 0 {
		return "No tickets selected."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Selected %d ticket(s):\n", len(tickets))
	for _, t := range tickets {
		fmt.Fprintf(&b, "  %s  %s\n", t.Key, t.Summary)
	}
	return b.String()
}
