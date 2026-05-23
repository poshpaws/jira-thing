package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds user-configurable settings loaded from ~/.config/jira-thing/jira-thing.json.
type Config struct {
	Project    string `json:"project"`
	ToilMarker string `json:"toil_marker"`
	ToilTeam   string `json:"toil_team"`
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
// Returns an empty Config if the file does not exist or cannot be parsed.
func Load() Config {
	path := ConfigPath()
	if path == "" {
		return Config{}
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Config{}
	}
	var cfg Config
	if json.Unmarshal(data, &cfg) != nil {
		return Config{}
	}
	return cfg
}
