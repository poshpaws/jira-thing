package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds user-configurable settings loaded from ~/.config/jira-thing/jira-thing.json.
type Config struct {
	Project         string `json:"project"`
	ToilMarker      string `json:"toil_marker"`
	ToilTeam        string `json:"toil_team"`
	Editor          string `json:"editor"`
	ConfluenceSpace string `json:"confluence_space"`
	TicketHanger    string `json:"ticket_hanger"`
}

// ConfigPath returns the path to the config file.
var ConfigPath = defaultConfigPath

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "jira-thing", "jira-thing.json")
}

// Load reads the config from ~/.config/jira-thing/jira-thing.json.
// Returns an empty Config if the file does not exist.
// Returns an error if the file exists but contains invalid JSON.
func Load() (Config, error) {
	path := ConfigPath()
	if path == "" {
		return Config{}, nil
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("reading config %s: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return cfg, nil
}
