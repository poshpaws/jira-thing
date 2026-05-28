package tui

import (
	"testing"
)

func TestTruncDate_Long(t *testing.T) {
	if got := truncDate("2026-04-25T10:30:00"); got != "2026-04-25" {
		t.Errorf("truncDate = %q, want 2026-04-25", got)
	}
}

func TestTruncDate_Short(t *testing.T) {
	if got := truncDate("short"); got != "short" {
		t.Errorf("truncDate = %q, want short", got)
	}
}

func TestTruncDate_Empty(t *testing.T) {
	if got := truncDate(""); got != "" {
		t.Errorf("truncDate = %q, want empty", got)
	}
}

func TestTruncDate_Exact10(t *testing.T) {
	if got := truncDate("2026-04-25"); got != "2026-04-25" {
		t.Errorf("truncDate = %q", got)
	}
}

func TestFindSelected(t *testing.T) {
	m := model{
		selected: []Ticket{
			{Key: "PROJ-101"},
			{Key: "PROJ-102"},
		},
	}
	if idx := m.findSelected("PROJ-102"); idx != 1 {
		t.Errorf("findSelected(PROJ-102) = %d, want 1", idx)
	}
	if idx := m.findSelected("PROJ-999"); idx != -1 {
		t.Errorf("findSelected(PROJ-999) = %d, want -1", idx)
	}
}

func TestNewModel_RowCount(t *testing.T) {
	tickets := []Ticket{
		{Key: "PROJ-101", Status: "In Progress", Priority: "High", Updated: "2026-04-25T10:00:00", Summary: "Fix login"},
		{Key: "PROJ-102", Status: "To Do", Priority: "Medium", Updated: "2026-04-23", Summary: "Update docs"},
	}
	m := newModel(tickets)
	rows := m.table.Rows()
	if len(rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rows))
	}
	if rows[0][0] != "PROJ-101" {
		t.Errorf("first row key = %q", rows[0][0])
	}
	if rows[1][3] != "2026-04-23" {
		t.Errorf("second row date = %q", rows[1][3])
	}
}

func TestFormatSelected_Empty(t *testing.T) {
	got := FormatSelected(nil)
	if got != "No tickets selected." {
		t.Errorf("FormatSelected(nil) = %q", got)
	}
}

func TestFormatSelected_WithTickets(t *testing.T) {
	tickets := []Ticket{
		{Key: "PROJ-101", Summary: "Fix login"},
	}
	got := FormatSelected(tickets)
	if got == "No tickets selected." {
		t.Error("expected formatted output, got empty message")
	}
}
