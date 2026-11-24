package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

// ExplorerGroup is one sidebar entry, such as an epic or a sprint.
type ExplorerGroup struct {
	ID     string // what the loader needs: an epic key or a sprint ID
	Title  string
	Detail string // shown dimmed after the title, e.g. a status or dates
}

// GroupLoader fetches the issues in a group. The explorer calls it in the
// background when the cursor lands on a group it hasn't loaded yet.
type GroupLoader func(group ExplorerGroup) ([]Ticket, error)

// ExplorerResult is how the explorer ended: a quick action on Key (Action is
// ActionNone when the user just quit), and the group that was open, so the
// caller can reopen the explorer in the same place after acting.
type ExplorerResult struct {
	Action TableActionKind
	Key    string
	Group  int
}

// explorerFocus says which pane takes the keys.
type explorerFocus int

const (
	focusGroups explorerFocus = iota
	focusIssues
)

// groupLoadedMsg carries one group's issues back from a background load.
type groupLoadedMsg struct {
	group   int
	tickets []Ticket
	err     error
}

// explorerModel is the bubbletea model for the two-pane explorer.
type explorerModel struct {
	title    string
	groups   []ExplorerGroup
	load     GroupLoader
	cursor   int
	focus    explorerFocus
	cache    map[int][]Ticket
	errs     map[int]error
	loading  map[int]bool
	issues   table.Model
	showHelp bool
	result   ExplorerResult
	done     bool
}

// ShowExplorer runs the explorer over groups, starting on group start, and
// returns the quick action the user chose, if any. Issues are loaded with
// load as each group is first shown.
func ShowExplorer(title string, groups []ExplorerGroup, load GroupLoader, start int) (ExplorerResult, error) {
	if len(groups) == 0 {
		fmt.Println("Nothing to explore.")
		return ExplorerResult{}, nil
	}
	final, err := tea.NewProgram(newExplorerModel(title, groups, load, start), tea.WithAltScreen()).Run()
	if err != nil {
		return ExplorerResult{}, fmt.Errorf("running explorer: %w", err)
	}
	return final.(explorerModel).result, nil
}

// newExplorerModel builds the model with the cursor on start (clamped).
func newExplorerModel(title string, groups []ExplorerGroup, load GroupLoader, start int) explorerModel {
	return explorerModel{
		title: title, groups: groups, load: load, cursor: max(0, min(start, len(groups)-1)),
		cache: map[int][]Ticket{}, errs: map[int]error{}, loading: map[int]bool{},
		issues: newExplorerTable(nil),
	}
}

func (m explorerModel) Init() tea.Cmd { return m.loadCmd(m.cursor) }

func (m explorerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case groupLoadedMsg:
		return m.handleLoaded(msg), nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// loadCmd starts a background load of group i, unless it is cached or
// loading. A group that failed isn't cached, so it is tried again on return.
func (m explorerModel) loadCmd(i int) tea.Cmd {
	if _, ok := m.cache[i]; ok || m.loading[i] {
		return nil
	}
	m.loading[i] = true
	load, group := m.load, m.groups[i]
	return func() tea.Msg {
		tickets, err := load(group)
		return groupLoadedMsg{group: i, tickets: tickets, err: err}
	}
}

// handleLoaded stores a finished load and shows it if it is the open group.
func (m explorerModel) handleLoaded(msg groupLoadedMsg) explorerModel {
	delete(m.loading, msg.group)
	if msg.err != nil {
		m.errs[msg.group] = msg.err
	} else {
		delete(m.errs, msg.group)
		m.cache[msg.group] = msg.tickets
	}
	if msg.group == m.cursor {
		m.issues = newExplorerTable(m.cache[m.cursor])
		m.issues.Blur()
		if m.focus == focusIssues {
			m.issues.Focus()
		}
	}
	return m
}

// moveTo opens group i: shows its cached issues and loads them if needed.
func (m explorerModel) moveTo(i int) (explorerModel, tea.Cmd) {
	if i < 0 || i >= len(m.groups) || i == m.cursor {
		return m, nil
	}
	m.cursor = i
	m.issues = newExplorerTable(m.cache[i])
	return m, m.loadCmd(i)
}

// reload drops the open group's cache and loads it again.
func (m explorerModel) reload() (explorerModel, tea.Cmd) {
	delete(m.cache, m.cursor)
	delete(m.errs, m.cursor)
	m.issues = newExplorerTable(nil)
	return m, m.loadCmd(m.cursor)
}

// newExplorerTable is the issue pane: the ticket table's columns, sized for
// sitting beside the sidebar.
func newExplorerTable(tickets []Ticket) table.Model {
	t := newTicketTable(tickets)
	t.Blur()
	return t
}
