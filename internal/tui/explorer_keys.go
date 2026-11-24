package tui

import tea "github.com/charmbracelet/bubbletea"

// handleKey routes a keypress: keys that work anywhere first, then the
// focused pane's own keys.
func (m explorerModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.done = true
		return m, tea.Quit
	case "?":
		m.showHelp = !m.showHelp
		return m, nil
	case "tab", "shift+tab":
		return m.toggleFocus(), nil
	case "r", "ctrl+r":
		return m.reload()
	}
	if m.showHelp {
		m.showHelp = msg.String() != "esc"
		return m, nil
	}
	if m.focus == focusGroups {
		return m.groupKey(msg)
	}
	return m.issueKey(msg)
}

// toggleFocus switches between the sidebar and the issue table.
func (m explorerModel) toggleFocus() explorerModel {
	if m.focus == focusGroups {
		m.focus = focusIssues
		m.issues.Focus()
		return m
	}
	m.focus = focusGroups
	m.issues.Blur()
	return m
}

// groupKey handles keys while the sidebar has focus.
func (m explorerModel) groupKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		return m.moveTo(m.cursor - 1)
	case "down", "j":
		return m.moveTo(m.cursor + 1)
	case "g", "home":
		return m.moveTo(0)
	case "G", "end":
		return m.moveTo(len(m.groups) - 1)
	case "enter", "right", "l":
		return m.toggleFocus(), nil
	case "esc":
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

// issueKey handles keys while the issue table has focus: quick actions end
// the explorer with a result, esc goes back to the sidebar, and the rest
// move the table cursor.
func (m explorerModel) issueKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "left", "h":
		return m.toggleFocus(), nil
	}
	if action, ok := quickActionFor(msg.String()); ok {
		return m.quickAction(action)
	}
	var cmd tea.Cmd
	m.issues, cmd = m.issues.Update(msg)
	return m, cmd
}

// quickAction ends the explorer with action on the ticket under the cursor;
// with no ticket there, it does nothing.
func (m explorerModel) quickAction(action TableActionKind) (tea.Model, tea.Cmd) {
	tickets := m.cache[m.cursor]
	row := m.issues.Cursor()
	if row < 0 || row >= len(tickets) {
		return m, nil
	}
	m.result = ExplorerResult{Action: action, Key: tickets[row].Key, Group: m.cursor}
	m.done = true
	return m, tea.Quit
}
