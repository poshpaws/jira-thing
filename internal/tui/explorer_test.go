package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// key builds a keypress message for the named key.
func key(name string) tea.KeyMsg {
	switch name {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)}
}

// explorerFixture has two groups; group "E-2" fails to load.
func explorerFixture() (explorerModel, *int) {
	calls := new(int)
	load := func(g ExplorerGroup) ([]Ticket, error) {
		*calls++
		if g.ID == "E-2" {
			return nil, errors.New("boom")
		}
		return []Ticket{{Key: g.ID + "-a", Summary: "first"}, {Key: g.ID + "-b", Summary: "second"}}, nil
	}
	groups := []ExplorerGroup{{ID: "E-1", Title: "Epic one", Detail: "In Progress"}, {ID: "E-2", Title: "Epic two\nforged"}}
	return newExplorerModel("Epics in PROJ", groups, load, 0), calls
}

// run feeds msgs through Update, running any command each one returns, as
// bubbletea would, and returns the final model.
func run(m explorerModel, msgs ...tea.Msg) explorerModel {
	step := func(msg tea.Msg) {
		next, cmd := m.Update(msg)
		m = next.(explorerModel)
		if cmd == nil {
			return
		}
		if loaded, ok := cmd().(groupLoadedMsg); ok {
			next, _ = m.Update(loaded)
			m = next.(explorerModel)
		}
	}
	if cmd := m.Init(); cmd != nil {
		step(cmd())
	}
	for _, msg := range msgs {
		step(msg)
	}
	return m
}

func TestExplorer_LoadsAndShowsFirstGroup(t *testing.T) {
	m, calls := explorerFixture()
	m = run(m)
	view := m.View()
	if *calls != 1 || !strings.Contains(view, "E-1-a") || !strings.Contains(view, "2 issue(s)") || !strings.Contains(view, "Epics in PROJ") {
		t.Errorf("calls=%d view=\n%s", *calls, view)
	}
}

func TestExplorer_MovingLoadsOnceAndShowsErrors(t *testing.T) {
	m, calls := explorerFixture()
	m = run(m, key("j"), key("k"), key("j"))
	view := m.View()
	// E-1 loads once; E-2 fails, so it is tried again when revisited.
	if *calls != 3 || m.cursor != 1 || !strings.Contains(view, "Couldn't load: boom") {
		t.Errorf("calls=%d cursor=%d view=\n%s", *calls, m.cursor, view)
	}
	if strings.Contains(view, "two\nforged") {
		t.Errorf("group title not kept to one line:\n%s", view)
	}
}

func TestExplorer_QuickActionFromIssues(t *testing.T) {
	m, _ := explorerFixture()
	m = run(m, key("tab"), key("down"), key("o"))
	if !m.done || m.result != (ExplorerResult{Action: ActionOpen, Key: "E-1-b", Group: 0}) {
		t.Errorf("result=%+v done=%v", m.result, m.done)
	}
	if m.View() != "" {
		t.Error("view should be empty once done")
	}
}

func TestExplorer_QuickActionKeysIgnoredInSidebar(t *testing.T) {
	m, _ := explorerFixture()
	m = run(m, key("v"))
	if m.done || m.result.Action != ActionNone {
		t.Errorf("sidebar v should do nothing: %+v", m.result)
	}
}

func TestExplorer_FocusAndHelp(t *testing.T) {
	m, _ := explorerFixture()
	m = run(m, key("enter"))
	if m.focus != focusIssues {
		t.Fatal("enter should focus the issues")
	}
	m = run(m, key("esc"))
	if m.focus != focusGroups || m.done {
		t.Errorf("esc in issues should go back, focus=%v done=%v", m.focus, m.done)
	}
	m = run(m, key("?"))
	if !strings.Contains(m.View(), "copy its URL") {
		t.Errorf("help not shown:\n%s", m.View())
	}
	m = run(m, key("esc"))
	if m.showHelp || m.done {
		t.Errorf("esc should close help only: help=%v done=%v", m.showHelp, m.done)
	}
}

func TestExplorer_ReloadAndQuit(t *testing.T) {
	m, calls := explorerFixture()
	m = run(m, key("r"))
	if *calls != 2 {
		t.Errorf("reload should load again, calls=%d", *calls)
	}
	m = run(m, key("q"))
	if !m.done || m.result.Action != ActionNone {
		t.Errorf("q should quit with no action: %+v", m.result)
	}
	m2, _ := explorerFixture()
	if m2 = run(m2, key("esc")); !m2.done {
		t.Error("esc in the sidebar should quit")
	}
}

func TestExplorer_StartClampedAndEmptyGroup(t *testing.T) {
	load := func(ExplorerGroup) ([]Ticket, error) { return nil, nil }
	m := run(newExplorerModel("x", []ExplorerGroup{{ID: "1", Title: "Only"}}, load, 9))
	if m.cursor != 0 || !strings.Contains(m.View(), "No issues.") {
		t.Errorf("cursor=%d view=\n%s", m.cursor, m.View())
	}
	if m = run(m, key("tab"), key("o")); m.done {
		t.Error("a quick action with no issue under the cursor should do nothing")
	}
}
