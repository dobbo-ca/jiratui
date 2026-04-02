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
