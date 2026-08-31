package tui

import "testing"

func TestPickCursor_ManualRowIsIndexZero(t *testing.T) {
	m := newPickerModel([]Ticket{{Key: "PROJ-1"}, {Key: "PROJ-2"}})
	m.table.SetCursor(0)

	m.pickCursor()

	if !m.result.Manual {
		t.Error("expected Manual to be true for cursor at row 0")
	}
	if m.result.Key != "" {
		t.Errorf("expected empty key for manual selection, got %q", m.result.Key)
	}
}

func TestPickCursor_TicketRowResolvesKey(t *testing.T) {
	m := newPickerModel([]Ticket{{Key: "PROJ-1"}, {Key: "PROJ-2"}})
	m.table.SetCursor(2) // manual row is 0, so row 2 is tickets[1]

	m.pickCursor()

	if m.result.Manual {
		t.Error("expected Manual to be false for a ticket row")
	}
	if m.result.Key != "PROJ-2" {
		t.Errorf("got key %q, want PROJ-2", m.result.Key)
	}
}

func TestPickTicket_EmptyListStillOffersManualEntry(t *testing.T) {
	m := newPickerModel(nil)
	if len(m.table.Rows()) != 1 {
		t.Fatalf("expected 1 row (manual entry) for empty ticket list, got %d", len(m.table.Rows()))
	}
}
