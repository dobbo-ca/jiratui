package config

import (
	"path/filepath"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := &Config{
		ActiveProfile: "work",
		Profiles: map[string]Profile{
			"work": {
				URL:      "https://company.atlassian.net",
				Email:    "chris@company.com",
				APIToken: "test-token-123",
			},
		},
	}

	err := Save(cfg, path)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ActiveProfile != "work" {
		t.Errorf("ActiveProfile = %q, want %q", loaded.ActiveProfile, "work")
	}

	p, ok := loaded.Profiles["work"]
	if !ok {
		t.Fatal("profile 'work' not found")
	}
	if p.URL != "https://company.atlassian.net" {
		t.Errorf("URL = %q, want %q", p.URL, "https://company.atlassian.net")
	}
	if p.Email != "chris@company.com" {
		t.Errorf("Email = %q, want %q", p.Email, "chris@company.com")
	}
	if p.APIToken != "test-token-123" {
		t.Errorf("APIToken = %q, want %q", p.APIToken, "test-token-123")
	}
}

func TestLoadNonExistent(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestDefaultPath(t *testing.T) {
	path := DefaultPath()
	if path == "" {
		t.Fatal("DefaultPath returned empty string")
	}
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("DefaultPath = %q, want filename config.yaml", path)
	}
}

func TestAddProfile(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]Profile{},
	}

	err := cfg.AddProfile("work", Profile{
		URL:      "https://company.atlassian.net",
		Email:    "chris@company.com",
		APIToken: "token-123",
	})
	if err != nil {
		t.Fatalf("AddProfile failed: %v", err)
	}

	if _, ok := cfg.Profiles["work"]; !ok {
		t.Fatal("profile 'work' not found after add")
	}

	// Adding duplicate should fail
	err = cfg.AddProfile("work", Profile{URL: "https://other.atlassian.net"})
	if err == nil {
		t.Fatal("expected error adding duplicate profile, got nil")
	}
}

func TestActiveProfileConfig(t *testing.T) {
	cfg := &Config{
		ActiveProfile: "work",
		Profiles: map[string]Profile{
			"work": {
				URL:      "https://company.atlassian.net",
				Email:    "chris@company.com",
				APIToken: "token-123",
			},
		},
	}

	p, err := cfg.ActiveProfileConfig()
	if err != nil {
		t.Fatalf("ActiveProfileConfig failed: %v", err)
	}
	if p.URL != "https://company.atlassian.net" {
		t.Errorf("URL = %q, want %q", p.URL, "https://company.atlassian.net")
	}

	// Missing active profile
	cfg.ActiveProfile = "nonexistent"
	_, err = cfg.ActiveProfileConfig()
	if err == nil {
		t.Fatal("expected error for missing profile, got nil")
	}

	// Empty active profile
	cfg.ActiveProfile = ""
	_, err = cfg.ActiveProfileConfig()
	if err == nil {
		t.Fatal("expected error for empty active profile, got nil")
	}
}

func TestSavedFiltersPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := &Config{
		ActiveProfile: "work",
		Profiles: map[string]Profile{
			"work": {
				URL:      "https://company.atlassian.net",
				Email:    "chris@company.com",
				APIToken: "token-123",
				Project:  "TEST",
				Filters: map[string]SavedFilters{
					"TEST": {
						StatusIDs:   []string{"10001", "10002"},
						PriorityIDs: []string{"1"},
						SearchText:  "azure",
					},
				},
			},
		},
	}

	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	p := loaded.Profiles["work"]
	if p.Filters == nil {
		t.Fatal("Filters map should not be nil")
	}
	sf, ok := p.Filters["TEST"]
	if !ok {
		t.Fatal("saved filters for project TEST not found")
	}
	if len(sf.StatusIDs) != 2 || sf.StatusIDs[0] != "10001" {
		t.Errorf("StatusIDs = %v, want [10001, 10002]", sf.StatusIDs)
	}
	if len(sf.PriorityIDs) != 1 || sf.PriorityIDs[0] != "1" {
		t.Errorf("PriorityIDs = %v, want [1]", sf.PriorityIDs)
	}
	if sf.SearchText != "azure" {
		t.Errorf("SearchText = %q, want %q", sf.SearchText, "azure")
	}
}
