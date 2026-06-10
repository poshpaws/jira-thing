package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jira-thing.json")
	os.WriteFile(path, []byte(`{"toil_marker":"ECP_TOIL","toil_team":"ECP_SEC_TEAM"}`), 0o644)

	old := ConfigPath
	ConfigPath = func() string { return path }
	defer func() { ConfigPath = old }()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ToilMarker != "ECP_TOIL" {
		t.Errorf("ToilMarker = %q, want ECP_TOIL", cfg.ToilMarker)
	}
	if cfg.ToilTeam != "ECP_SEC_TEAM" {
		t.Errorf("ToilTeam = %q, want ECP_SEC_TEAM", cfg.ToilTeam)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	old := ConfigPath
	ConfigPath = func() string { return "/nonexistent/path.json" }
	defer func() { ConfigPath = old }()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if cfg.ToilMarker != "" || cfg.ToilTeam != "" {
		t.Errorf("expected empty config, got %+v", cfg)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jira-thing.json")
	os.WriteFile(path, []byte(`not json`), 0o644)

	old := ConfigPath
	ConfigPath = func() string { return path }
	defer func() { ConfigPath = old }()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoad_EmptyPath(t *testing.T) {
	old := ConfigPath
	ConfigPath = func() string { return "" }
	defer func() { ConfigPath = old }()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error for empty path, got %v", err)
	}
	if cfg.ToilMarker != "" || cfg.ToilTeam != "" {
		t.Errorf("expected empty config, got %+v", cfg)
	}
}

func TestLoad_ConfluenceFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jira-thing.json")
	os.WriteFile(path, []byte(`{"confluence_space":"ENG","ticket_hanger":"Toil Tracker"}`), 0o644)

	old := ConfigPath
	ConfigPath = func() string { return path }
	defer func() { ConfigPath = old }()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ConfluenceSpace != "ENG" {
		t.Errorf("ConfluenceSpace = %q, want ENG", cfg.ConfluenceSpace)
	}
	if cfg.TicketHanger != "Toil Tracker" {
		t.Errorf("TicketHanger = %q, want Toil Tracker", cfg.TicketHanger)
	}
}
