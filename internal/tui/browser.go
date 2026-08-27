package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	browserHeight   = 15
	colWidthTitle   = 60
	colWidthID      = 14
	breadcrumbLimit = 5
)

// PageEntry represents a Confluence page in the browser.
type PageEntry struct {
	ID    string
	Title string
}

// PageFetcher loads the child pages of a given parent page ID.
// The caller provides this so the TUI does not depend on the api package.
type PageFetcher func(parentID string) ([]PageEntry, error)

// BrowseResult holds the user's selection from the space browser.
type BrowseResult struct {
	SelectedPage PageEntry
	Cancelled    bool
}

// fetchedMsg carries the result of an async page fetch.
type fetchedMsg struct {
	parentID string
	pages    []PageEntry
	err      error
}

// browserModel is the bubbletea model for the Confluence space browser.
type browserModel struct {
	table     table.Model
	fetcher   PageFetcher
	pages     []PageEntry // current directory listing
	crumbs    []PageEntry // navigation stack (parent pages)
	current   PageEntry   // the page whose children we are viewing
	selected  PageEntry   // final selection
	loading   bool
	err       error
	done      bool
	cancelled bool
}

// BrowseSpace launches the interactive space browser TUI.
// rootPage is the starting page; fetcher loads children on demand.
// Returns the page the user selected as the upload target.
func BrowseSpace(rootPage PageEntry, fetcher PageFetcher) (BrowseResult, error) {
	m := newBrowserModel(rootPage, fetcher)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return BrowseResult{}, fmt.Errorf("running browser TUI: %w", err)
	}
	final := result.(browserModel)
	if final.cancelled || final.err != nil {
		msg := ""
		if final.err != nil {
			msg = final.err.Error()
		}
		if msg != "" {
			return BrowseResult{Cancelled: true}, fmt.Errorf("browse error: %s", msg)
		}
		return BrowseResult{Cancelled: true}, nil
	}
	return BrowseResult{SelectedPage: final.selected}, nil
}

func newBrowserModel(root PageEntry, fetcher PageFetcher) browserModel {
	t := newBrowserTable(nil)
	return browserModel{
		table:   t,
		fetcher: fetcher,
		current: root,
		loading: true,
	}
}

func newBrowserTable(pages []PageEntry) table.Model {
	columns := []table.Column{
		{Title: "Page Title", Width: colWidthTitle},
		{Title: "ID", Width: colWidthID},
	}
	rows := pagesToRows(pages)
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(browserHeight),
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

func pagesToRows(pages []PageEntry) []table.Row {
	rows := make([]table.Row, len(pages))
	for i, p := range pages {
		rows[i] = table.Row{p.Title, p.ID}
	}
	return rows
}

// Init dispatches the first fetch for the root page's children.
func (m browserModel) Init() tea.Cmd {
	return m.fetchChildren(m.current.ID)
}

func (m browserModel) fetchChildren(parentID string) tea.Cmd {
	fetcher := m.fetcher
	return func() tea.Msg {
		pages, err := fetcher(parentID)
		return fetchedMsg{parentID: parentID, pages: pages, err: err}
	}
}

// Update handles keypresses and async fetch results.
func (m browserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case fetchedMsg:
		return m.handleFetched(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m browserModel) handleFetched(msg fetchedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.err = msg.err
		m.done = true
		m.cancelled = true
		return m, tea.Quit
	}
	m.pages = msg.pages
	m.table = newBrowserTable(msg.pages)
	return m, nil
}

func (m browserModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		m.cancelled = true
		m.done = true
		return m, tea.Quit
	case "enter", "right", "l":
		return m.drillDown()
	case "backspace", "left", "h":
		return m.goUp()
	case "s":
		return m.selectCurrent()
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// drillDown navigates into the selected child page.
func (m browserModel) drillDown() (tea.Model, tea.Cmd) {
	cursor := m.table.Cursor()
	if cursor >= len(m.pages) || len(m.pages) == 0 {
		return m, nil
	}
	child := m.pages[cursor]
	m.crumbs = append(m.crumbs, m.current)
	m.current = child
	m.loading = true
	m.pages = nil
	m.table = newBrowserTable(nil)
	return m, m.fetchChildren(child.ID)
}

// goUp navigates back to the parent page.
func (m browserModel) goUp() (tea.Model, tea.Cmd) {
	if len(m.crumbs) == 0 {
		return m, nil
	}
	parent := m.crumbs[len(m.crumbs)-1]
	m.crumbs = m.crumbs[:len(m.crumbs)-1]
	m.current = parent
	m.loading = true
	m.pages = nil
	m.table = newBrowserTable(nil)
	return m, m.fetchChildren(parent.ID)
}

// selectCurrent marks the current page (the one whose children are displayed) as the target.
func (m browserModel) selectCurrent() (tea.Model, tea.Cmd) {
	m.selected = m.current
	m.done = true
	return m, tea.Quit
}

// View renders the browser interface.
func (m browserModel) View() string {
	if m.done {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(breadcrumbView(m.crumbs, m.current))
	sb.WriteString("\n\n")

	if m.loading {
		sb.WriteString("  Loading...")
		return sb.String()
	}
	if len(m.pages) == 0 {
		sb.WriteString("  (no child pages)")
		sb.WriteString(browserHelp())
		return sb.String()
	}
	sb.WriteString(m.table.View())
	sb.WriteString(browserHelp())
	return sb.String()
}

// breadcrumbView builds the breadcrumb trail showing navigation context.
func breadcrumbView(crumbs []PageEntry, current PageEntry) string {
	arrow := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(" › ")
	crumbStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	currentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)

	var parts []string
	start := 0
	if len(crumbs) > breadcrumbLimit {
		start = len(crumbs) - breadcrumbLimit
		parts = append(parts, crumbStyle.Render("…"))
	}
	for _, c := range crumbs[start:] {
		parts = append(parts, crumbStyle.Render(truncateTitle(c.Title)))
	}
	parts = append(parts, currentStyle.Render(truncateTitle(current.Title)))
	return "  " + strings.Join(parts, arrow)
}

// truncateTitle shortens long page titles for the breadcrumb display.
func truncateTitle(s string) string {
	const maxLen = 30
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

func browserHelp() string {
	return "\n  ↑/↓ navigate • enter/→ drill in • backspace/← go up • s select this page • q quit\n"
}
