package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SavedFilters stores the user's last-used filter selections per project.
type SavedFilters struct {
	TypeIDs      []string `yaml:"type_ids,omitempty"`
	StatusIDs    []string `yaml:"status_ids,omitempty"`
	PriorityIDs  []string `yaml:"priority_ids,omitempty"`
	AssigneeIDs  []string `yaml:"assignee_ids,omitempty"`
	LabelIDs     []string `yaml:"label_ids,omitempty"`
	CreatedFrom  string   `yaml:"created_from,omitempty"`
	CreatedUntil string   `yaml:"created_until,omitempty"`
	SearchText   string   `yaml:"search_text,omitempty"`
}

type Profile struct {
	URL      string `yaml:"url"`
	Email    string `yaml:"email"`
	APIToken string `yaml:"api_token"`
	Project  string `yaml:"project,omitempty"`  // last-used project key
	Filters  map[string]SavedFilters `yaml:"filters,omitempty"` // per-project saved filters (key = project key, "" = all)
}

type Config struct {
	ActiveProfile string             `yaml:"active_profile"`
	Profiles      map[string]Profile `yaml:"profiles"`
}

// DefaultPath returns ~/.config/jiratui/config.yaml
func DefaultPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(configDir, "jiratui", "config.yaml")
}

// Load reads and parses a config file from the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]Profile)
	}

	return &cfg, nil
}

// Save writes the config to the given path, creating directories as needed.
func Save(cfg *Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// ActiveProfileConfig returns the currently active profile, or an error if not configured.
func (c *Config) ActiveProfileConfig() (Profile, error) {
	if c.ActiveProfile == "" {
		return Profile{}, fmt.Errorf("no active profile set")
	}
	p, ok := c.Profiles[c.ActiveProfile]
	if !ok {
		return Profile{}, fmt.Errorf("active profile %q not found in config", c.ActiveProfile)
	}
	return p, nil
}

// AddProfile adds a new profile. Returns error if name already exists.
func (c *Config) AddProfile(name string, profile Profile) error {
	if _, exists := c.Profiles[name]; exists {
		return fmt.Errorf("profile %q already exists", name)
	}
	if c.Profiles == nil {
		c.Profiles = make(map[string]Profile)
	}
	c.Profiles[name] = profile
	return nil
}

// Exists returns true if a config file exists at the given path.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
