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

// TableActionKind identifies a quick action triggered from the ticket table,
// as opposed to the normal multi-select-then-quit flow.
type TableActionKind string

const (
	ActionNone       TableActionKind = ""
	ActionView       TableActionKind = "view"
	ActionOpen       TableActionKind = "open"
	ActionTransition TableActionKind = "transition"
	ActionCopyKey    TableActionKind = "copy_key"
	ActionCopyURL    TableActionKind = "copy_url"
)

// TableResult is the outcome of ShowTableWithQuickActions: either a normal
// selection (Action == ActionNone) or a quick action against the ticket under
// the cursor when the key was pressed.
type TableResult struct {
	Selected []Ticket
	Action   TableActionKind
	Key      string
}

// TicketFetcher reloads the tickets shown in the table, e.g. by re-running the
// search that originally produced them. Used for the ctrl+r refresh action.
type TicketFetcher func() ([]Ticket, error)

// refreshMsg carries the result of an async refresh triggered by ctrl+r.
type refreshMsg struct {
	tickets []Ticket
	err     error
}

// model is the bubbletea model for the ticket table.
type model struct {
	table               table.Model
	tickets             []Ticket
	selected            []Ticket
	quitting            bool
	quickActionsEnabled bool
	quickAction         TableActionKind
	quickKey            string
	fetcher             TicketFetcher
	refreshing          bool
	refreshErr          error
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

// ShowTableWithQuickActions launches the interactive table TUI with extra
// single-key actions available on the ticket under the cursor: v (view), o
// (open in browser), m (transition), c (copy URL), ctrl+k (copy key). If
// fetcher is non-nil, ctrl+r re-runs it and reloads the table in place.
// The caller performs the actual action after the TUI exits, based on the
// returned TableResult.
func ShowTableWithQuickActions(tickets []Ticket, fetcher TicketFetcher) (TableResult, error) {
	if len(tickets) == 0 {
		fmt.Println("No tickets to display.")
		return TableResult{}, nil
	}

	m := newModel(tickets)
	m.quickActionsEnabled = true
	m.fetcher = fetcher
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return TableResult{}, fmt.Errorf("running TUI: %w", err)
	}

	final := result.(model)
	return TableResult{Selected: final.selected, Action: final.quickAction, Key: final.quickKey}, nil
}

func newModel(tickets []Ticket) model {
	return model{table: newTicketTable(tickets), tickets: tickets}
}

func newTicketTable(tickets []Ticket) table.Model {
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
	return t
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case refreshMsg:
		return m.handleRefresh(msg), nil
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
		case "v", "o", "m", "c", "ctrl+k":
			if action, ok := quickActionFor(msg.String()); m.quickActionsEnabled && ok {
				m.recordQuickAction(action)
				return m, tea.Quit
			}
		case "ctrl+r":
			if m.quickActionsEnabled && m.fetcher != nil && !m.refreshing {
				m.refreshing = true
				return m, m.refreshCmd()
			}
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// refreshCmd asynchronously re-runs the fetcher and reports the result as a refreshMsg.
func (m model) refreshCmd() tea.Cmd {
	fetcher := m.fetcher
	return func() tea.Msg {
		tickets, err := fetcher()
		return refreshMsg{tickets: tickets, err: err}
	}
}

// handleRefresh applies the outcome of a ctrl+r refresh, rebuilding the table
// on success or recording the error for display otherwise.
func (m model) handleRefresh(msg refreshMsg) model {
	m.refreshing = false
	if msg.err != nil {
		m.refreshErr = msg.err
		return m
	}
	m.refreshErr = nil
	m.tickets = msg.tickets
	m.selected = nil
	m.table = newTicketTable(msg.tickets)
	return m
}

// quickActionFor maps a quick-action keypress to its TableActionKind.
func quickActionFor(key string) (TableActionKind, bool) {
	switch key {
	case "v":
		return ActionView, true
	case "o":
		return ActionOpen, true
	case "m":
		return ActionTransition, true
	case "c":
		return ActionCopyURL, true
	case "ctrl+k":
		return ActionCopyKey, true
	}
	return ActionNone, false
}

// recordQuickAction marks the ticket under the cursor as the target of a quick action.
func (m *model) recordQuickAction(action TableActionKind) {
	cursor := m.table.Cursor()
	if cursor < len(m.tickets) {
		m.quickKey = m.tickets[cursor].Key
	}
	m.quickAction = action
	m.quitting = true
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
	help := "\n  ↑/↓/j/k navigate • g/G top/bottom • enter/space select • s save & quit"
	if m.quickActionsEnabled {
		help += " • v view • o open • m transition • c copy url • ctrl+k copy key"
		if m.fetcher != nil {
			help += " • ctrl+r refresh"
		}
	}
	help += " • q quit\n"
	status := fmt.Sprintf("  %d ticket(s) selected", len(m.selected))
	if m.refreshing {
		status += "  (refreshing...)"
	} else if m.refreshErr != nil {
		status += fmt.Sprintf("  (refresh failed: %v)", m.refreshErr)
	}
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
