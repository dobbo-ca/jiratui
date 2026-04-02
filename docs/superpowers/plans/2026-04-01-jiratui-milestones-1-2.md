# jiratui Milestones 1 & 2: Config/Auth + API Client

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the config/auth system and Jira Cloud REST API client so a user can authenticate, manage profiles, and fetch their real Jira issues from the command line.

**Architecture:** CLI app with two subsystems: (1) a config layer that reads/writes YAML profiles to `~/.config/jiratui/config.yaml`, and (2) a Jira REST v3 client that uses those credentials to fetch issues. Both are in `internal/` packages consumed by CLI commands in `main.go`.

**Tech Stack:** Go 1.22+, `gopkg.in/yaml.v3`, `github.com/spf13/cobra` (CLI framework), standard library `net/http` + `encoding/json`.

**Spec:** `docs/superpowers/specs/2026-04-01-jiratui-go-design.md`

---

## File Structure

```
jiratui/
├── main.go                          # Cobra root command, wires subcommands
├── cmd/
│   ├── root.go                      # Root command definition
│   ├── auth.go                      # `auth add`, `auth list`, `auth switch` commands
│   └── issues.go                    # `issues` command (stdout dump for testing)
├── internal/
│   ├── config/
│   │   ├── config.go                # Config struct, Load/Save, profile management
│   │   └── config_test.go           # Tests for config load/save/profile ops
│   ├── jira/
│   │   ├── client.go                # HTTP client, auth, request helpers
│   │   ├── client_test.go           # Tests with httptest server
│   │   ├── issues.go                # Issue search + detail fetching
│   │   ├── issues_test.go           # Tests for issue endpoints
│   │   └── types.go                 # Jira API response types (raw JSON shapes)
│   └── models/
│       └── models.go                # Domain types: Issue, Comment, User, etc.
├── go.mod
└── go.sum
```

---

## Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `cmd/root.go`

- [ ] **Step 1: Initialize Go module**

Run:
```bash
cd /Users/christopherdobbyn/work/dobbo-ca/jiratui
go mod init github.com/christopherdobbyn/jiratui
```

Expected: `go.mod` created with module path.

- [ ] **Step 2: Install Cobra dependency**

Run:
```bash
go get github.com/spf13/cobra@latest
```

- [ ] **Step 3: Create root command**

Create `cmd/root.go`:
```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "jiratui",
	Short: "A terminal UI for Jira Cloud",
	Long:  "jiratui is a fast, lightweight terminal user interface for browsing and interacting with Jira Cloud.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Create main.go**

Create `main.go`:
```go
package main

import "github.com/christopherdobbyn/jiratui/cmd"

func main() {
	cmd.Execute()
}
```

- [ ] **Step 5: Verify it builds and runs**

Run:
```bash
go build -o jiratui . && ./jiratui --help
```

Expected: Help text showing "jiratui is a fast, lightweight terminal user interface..."

- [ ] **Step 6: Commit**

```bash
git add main.go cmd/root.go go.mod go.sum
git commit -m "feat: scaffold project with cobra root command"
```

---

## Task 2: Config Package — Load & Save

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test for config load/save**

Create `internal/config/config_test.go`:
```go
package config

import (
	"os"
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
	// Should end with jiratui/config.yaml
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("DefaultPath = %q, want filename config.yaml", path)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/config/ -v
```
Expected: FAIL — package doesn't exist yet.

- [ ] **Step 3: Install yaml dependency**

Run:
```bash
go get gopkg.in/yaml.v3@latest
```

- [ ] **Step 4: Write the implementation**

Create `internal/config/config.go`:
```go
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Profile struct {
	URL      string `yaml:"url"`
	Email    string `yaml:"email"`
	APIToken string `yaml:"api_token"`
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

// ActiveProfile returns the currently active profile, or an error if not configured.
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
```

- [ ] **Step 5: Run tests to verify they pass**

Run:
```bash
go test ./internal/config/ -v
```
Expected: All 3 tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go go.mod go.sum
git commit -m "feat: add config package with load/save and profile management"
```

---

## Task 3: Config Package — Profile Operations Tests

**Files:**
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Add tests for AddProfile and ActiveProfileConfig**

Append to `internal/config/config_test.go`:
```go
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
```

- [ ] **Step 2: Run tests to verify they pass**

Run:
```bash
go test ./internal/config/ -v
```
Expected: All 5 tests PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/config/config_test.go
git commit -m "test: add profile operations tests"
```

---

## Task 4: Auth CLI Commands

**Files:**
- Create: `cmd/auth.go`

- [ ] **Step 1: Create auth commands**

Create `cmd/auth.go`:
```go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/christopherdobbyn/jiratui/internal/config"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage Jira authentication profiles",
}

var authAddCmd = &cobra.Command{
	Use:   "auth-add",
	Short: "Add a new authentication profile",
	RunE:  runAuthAdd,
}

var authListCmd = &cobra.Command{
	Use:   "auth-list",
	Short: "List all authentication profiles",
	RunE:  runAuthList,
}

var authSwitchCmd = &cobra.Command{
	Use:   "auth-switch [profile-name]",
	Short: "Switch the active profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runAuthSwitch,
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authAddCmd)
	authCmd.AddCommand(authListCmd)
	authCmd.AddCommand(authSwitchCmd)
}

func prompt(reader *bufio.Reader, label string) string {
	fmt.Printf("%s: ", label)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func runAuthAdd(cmd *cobra.Command, args []string) error {
	cfgPath := config.DefaultPath()

	var cfg *config.Config
	if config.Exists(cfgPath) {
		var err error
		cfg, err = config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("loading existing config: %w", err)
		}
	} else {
		cfg = &config.Config{
			Profiles: make(map[string]config.Profile),
		}
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Add a new Jira Cloud profile")
	fmt.Println("---")
	fmt.Println("You'll need an API token from: https://id.atlassian.com/manage-profile/security/api-tokens")
	fmt.Println()

	name := prompt(reader, "Profile name (e.g. work, personal)")
	url := prompt(reader, "Jira URL (e.g. https://company.atlassian.net)")
	email := prompt(reader, "Email")
	token := prompt(reader, "API token")

	// Normalize URL: strip trailing slash
	url = strings.TrimRight(url, "/")

	profile := config.Profile{
		URL:      url,
		Email:    email,
		APIToken: token,
	}

	if err := cfg.AddProfile(name, profile); err != nil {
		return err
	}

	// If this is the first profile, set it as active
	if len(cfg.Profiles) == 1 {
		cfg.ActiveProfile = name
	}

	if err := config.Save(cfg, cfgPath); err != nil {
		return err
	}

	fmt.Printf("\nProfile %q saved to %s\n", name, cfgPath)
	if cfg.ActiveProfile == name {
		fmt.Printf("Set as active profile.\n")
	}
	return nil
}

func runAuthList(cmd *cobra.Command, args []string) error {
	cfgPath := config.DefaultPath()

	if !config.Exists(cfgPath) {
		fmt.Println("No config file found. Run `jiratui auth add` to create one.")
		return nil
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	if len(cfg.Profiles) == 0 {
		fmt.Println("No profiles configured. Run `jiratui auth add` to add one.")
		return nil
	}

	fmt.Println("Profiles:")
	for name, p := range cfg.Profiles {
		active := " "
		if name == cfg.ActiveProfile {
			active = "*"
		}
		fmt.Printf("  %s %s — %s (%s)\n", active, name, p.URL, p.Email)
	}
	return nil
}

func runAuthSwitch(cmd *cobra.Command, args []string) error {
	cfgPath := config.DefaultPath()

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	name := args[0]
	if _, ok := cfg.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}

	cfg.ActiveProfile = name

	if err := config.Save(cfg, cfgPath); err != nil {
		return err
	}

	fmt.Printf("Switched active profile to %q\n", name)
	return nil
}
```

- [ ] **Step 2: Build and verify help text**

Run:
```bash
go build -o jiratui . && ./jiratui auth --help
```

Expected: Help text showing `auth-add`, `auth-list`, `auth-switch` subcommands.

- [ ] **Step 3: Commit**

```bash
git add cmd/auth.go
git commit -m "feat: add auth CLI commands (add, list, switch)"
```

---

## Task 5: First-Run Detection

**Files:**
- Modify: `cmd/root.go`

- [ ] **Step 1: Add first-run check to root command**

Replace `cmd/root.go` with:
```go
package cmd

import (
	"fmt"
	"os"

	"github.com/christopherdobbyn/jiratui/internal/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "jiratui",
	Short: "A terminal UI for Jira Cloud",
	Long:  "jiratui is a fast, lightweight terminal user interface for browsing and interacting with Jira Cloud.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip first-run check for auth commands (they handle it themselves)
		if cmd.Parent() != nil && cmd.Parent().Use == "auth" {
			return nil
		}
		// Also skip for the auth command group itself
		if cmd.Use == "auth" {
			return nil
		}

		cfgPath := config.DefaultPath()
		if !config.Exists(cfgPath) {
			fmt.Println("Welcome to jiratui!")
			fmt.Println()
			fmt.Println("No configuration found. Let's set up your first Jira Cloud profile.")
			fmt.Println()
			return runAuthAdd(cmd, nil)
		}
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Build and verify**

Run:
```bash
go build -o jiratui .
```
Expected: Builds successfully.

- [ ] **Step 3: Commit**

```bash
git add cmd/root.go
git commit -m "feat: add first-run detection that triggers auth setup"
```

---

## Task 6: Domain Models

**Files:**
- Create: `internal/models/models.go`

- [ ] **Step 1: Create domain types**

Create `internal/models/models.go`:
```go
package models

import "time"

type User struct {
	AccountID   string
	DisplayName string
	Email       string
	AvatarURL   string
}

type Priority struct {
	ID   string
	Name string // "Highest", "High", "Medium", "Low", "Lowest"
}

type Status struct {
	ID   string
	Name string // "To Do", "In Progress", "In Review", "Done"
}

type IssueType struct {
	ID   string
	Name string // "Bug", "Task", "Story", "Epic"
}

type Comment struct {
	ID      string
	Author  User
	Body    string
	Created time.Time
	Updated time.Time
}

type IssueLink struct {
	ID           string
	Type         string // "blocks", "is blocked by", "relates to"
	InwardIssue  *IssueSummary
	OutwardIssue *IssueSummary
}

type IssueSummary struct {
	Key     string
	Summary string
	Status  Status
}

type Issue struct {
	Key         string
	Summary     string
	Description string
	Status      Status
	Priority    Priority
	Type        IssueType
	Assignee    *User
	Reporter    *User
	Labels      []string
	Created     time.Time
	Updated     time.Time
	DueDate     *time.Time
	Parent      *IssueSummary
	Sprint      string
	Subtasks    []IssueSummary
	Links       []IssueLink
	Comments    []Comment
	ProjectKey  string
	ProjectName string
	BrowseURL   string
}
```

- [ ] **Step 2: Verify it compiles**

Run:
```bash
go build ./internal/models/
```
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add internal/models/models.go
git commit -m "feat: add domain models for Issue, Comment, User, etc."
```

---

## Task 7: Jira API Types (raw JSON shapes)

**Files:**
- Create: `internal/jira/types.go`

- [ ] **Step 1: Create Jira API response types**

These map directly to Jira's v3 REST API JSON structure. They're separate from domain models — the client maps these to `models.*` types.

Create `internal/jira/types.go`:
```go
package jira

// jiraSearchResponse is the response from /rest/api/3/search
type jiraSearchResponse struct {
	StartAt    int          `json:"startAt"`
	MaxResults int          `json:"maxResults"`
	Total      int          `json:"total"`
	Issues     []jiraIssue  `json:"issues"`
}

type jiraIssue struct {
	Key    string      `json:"key"`
	Self   string      `json:"self"`
	Fields jiraFields  `json:"fields"`
}

type jiraFields struct {
	Summary     string           `json:"summary"`
	Description *jiraADF         `json:"description"`
	Status      jiraStatus       `json:"status"`
	Priority    *jiraPriority    `json:"priority"`
	IssueType   jiraIssueType    `json:"issuetype"`
	Assignee    *jiraUser        `json:"assignee"`
	Reporter    *jiraUser        `json:"reporter"`
	Labels      []string         `json:"labels"`
	Created     string           `json:"created"`
	Updated     string           `json:"updated"`
	DueDate     *string          `json:"duedate"`
	Parent      *jiraParent      `json:"parent"`
	Sprint      *jiraSprint      `json:"sprint"`
	Subtasks    []jiraIssue      `json:"subtasks"`
	IssueLinks  []jiraIssueLink  `json:"issuelinks"`
	Comment     *jiraCommentPage `json:"comment"`
	Project     jiraProject      `json:"project"`
}

// jiraADF represents Atlassian Document Format. We extract plain text from it.
type jiraADF struct {
	Type    string     `json:"type"`
	Content []jiraNode `json:"content"`
}

type jiraNode struct {
	Type    string     `json:"type"`
	Text    string     `json:"text,omitempty"`
	Content []jiraNode `json:"content,omitempty"`
}

type jiraStatus struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type jiraPriority struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type jiraIssueType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type jiraUser struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress"`
	AvatarURLs  map[string]string `json:"avatarUrls"`
}

type jiraParent struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string     `json:"summary"`
		Status  jiraStatus `json:"status"`
	} `json:"fields"`
}

type jiraSprint struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type jiraIssueLink struct {
	ID   string `json:"id"`
	Type struct {
		Name    string `json:"name"`
		Inward  string `json:"inward"`
		Outward string `json:"outward"`
	} `json:"type"`
	InwardIssue  *jiraLinkedIssue `json:"inwardIssue"`
	OutwardIssue *jiraLinkedIssue `json:"outwardIssue"`
}

type jiraLinkedIssue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string     `json:"summary"`
		Status  jiraStatus `json:"status"`
	} `json:"fields"`
}

type jiraCommentPage struct {
	Total    int            `json:"total"`
	Comments []jiraComment  `json:"comments"`
}

type jiraComment struct {
	ID      string   `json:"id"`
	Author  jiraUser `json:"author"`
	Body    *jiraADF `json:"body"`
	Created string   `json:"created"`
	Updated string   `json:"updated"`
}

type jiraProject struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}
```

- [ ] **Step 2: Verify it compiles**

Run:
```bash
go build ./internal/jira/
```
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add internal/jira/types.go
git commit -m "feat: add Jira API response types for v3 REST API"
```

---

## Task 8: Jira Client — Core HTTP + ADF Parsing

**Files:**
- Create: `internal/jira/client.go`
- Create: `internal/jira/client_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/jira/client_test.go`:
```go
package jira

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("https://test.atlassian.net", "user@test.com", "token-123")
	if c.baseURL != "https://test.atlassian.net" {
		t.Errorf("baseURL = %q, want %q", c.baseURL, "https://test.atlassian.net")
	}
}

func TestClientAuthHeader(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "user@test.com", "token-123")
	_, err := c.get("/rest/api/3/myself")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if gotAuth == "" {
		t.Fatal("Authorization header was empty")
	}
	// Basic auth should be base64("user@test.com:token-123")
	if gotAuth[:6] != "Basic " {
		t.Errorf("Authorization header should start with 'Basic ', got %q", gotAuth[:6])
	}
}

func TestExtractTextFromADF(t *testing.T) {
	adf := &jiraADF{
		Type: "doc",
		Content: []jiraNode{
			{
				Type: "paragraph",
				Content: []jiraNode{
					{Type: "text", Text: "Hello "},
					{Type: "text", Text: "world"},
				},
			},
			{
				Type: "paragraph",
				Content: []jiraNode{
					{Type: "text", Text: "Second paragraph"},
				},
			},
		},
	}

	got := extractTextFromADF(adf)
	want := "Hello world\n\nSecond paragraph"
	if got != want {
		t.Errorf("extractTextFromADF = %q, want %q", got, want)
	}
}

func TestExtractTextFromADFNil(t *testing.T) {
	got := extractTextFromADF(nil)
	if got != "" {
		t.Errorf("extractTextFromADF(nil) = %q, want empty string", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/jira/ -v
```
Expected: FAIL — `NewClient` not defined.

- [ ] **Step 3: Write the implementation**

Create `internal/jira/client.go`:
```go
package jira

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/christopherdobbyn/jiratui/internal/models"
)

type Client struct {
	baseURL    string
	email      string
	apiToken   string
	httpClient *http.Client
}

func NewClient(baseURL, email, apiToken string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		email:    email,
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) get(path string) ([]byte, error) {
	url := c.baseURL + path

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(c.email + ":" + c.apiToken))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("authentication failed: check your email and API token")
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("access forbidden: check your permissions")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limited by Jira: try again in a moment")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Jira API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func decodeJSON[T any](data []byte) (T, error) {
	var result T
	err := json.Unmarshal(data, &result)
	return result, err
}

// extractTextFromADF converts Atlassian Document Format to plain text.
func extractTextFromADF(adf *jiraADF) string {
	if adf == nil {
		return ""
	}

	var paragraphs []string
	for _, node := range adf.Content {
		text := extractNodeText(node)
		if text != "" {
			paragraphs = append(paragraphs, text)
		}
	}
	return strings.Join(paragraphs, "\n\n")
}

func extractNodeText(node jiraNode) string {
	if node.Text != "" {
		return node.Text
	}
	var parts []string
	for _, child := range node.Content {
		text := extractNodeText(child)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "")
}

func mapUser(u *jiraUser) *models.User {
	if u == nil {
		return nil
	}
	avatarURL := ""
	if u.AvatarURLs != nil {
		avatarURL = u.AvatarURLs["48x48"]
	}
	return &models.User{
		AccountID:   u.AccountID,
		DisplayName: u.DisplayName,
		Email:       u.Email,
		AvatarURL:   avatarURL,
	}
}

func parseTime(s string) time.Time {
	// Jira uses ISO 8601: "2025-03-15T10:30:00.000+0000"
	formats := []string{
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
	}
	for _, f := range formats {
		t, err := time.Parse(f, s)
		if err == nil {
			return t
		}
	}
	return time.Time{}
}

func parseDateOnly(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return nil
	}
	return &t
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:
```bash
go test ./internal/jira/ -v
```
Expected: All 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/jira/client.go internal/jira/client_test.go
git commit -m "feat: add Jira HTTP client with auth and ADF text extraction"
```

---

## Task 9: Jira Client — Issue Fetching

**Files:**
- Create: `internal/jira/issues.go`
- Create: `internal/jira/issues_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/jira/issues_test.go`:
```go
package jira

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const searchResponseJSON = `{
	"startAt": 0,
	"maxResults": 50,
	"total": 2,
	"issues": [
		{
			"key": "PROJ-1",
			"self": "https://test.atlassian.net/rest/api/3/issue/10001",
			"fields": {
				"summary": "Fix login bug",
				"status": {"id": "1", "name": "To Do"},
				"priority": {"id": "2", "name": "High"},
				"issuetype": {"id": "1", "name": "Bug"},
				"assignee": {"accountId": "abc123", "displayName": "Chris", "emailAddress": "chris@test.com"},
				"reporter": {"accountId": "def456", "displayName": "Alice", "emailAddress": "alice@test.com"},
				"labels": ["backend", "auth"],
				"created": "2025-03-15T10:30:00.000+0000",
				"updated": "2025-03-28T14:00:00.000+0000",
				"duedate": "2025-04-01",
				"project": {"key": "PROJ", "name": "My Project"},
				"subtasks": [],
				"issuelinks": []
			}
		},
		{
			"key": "PROJ-2",
			"self": "https://test.atlassian.net/rest/api/3/issue/10002",
			"fields": {
				"summary": "Add rate limiting",
				"status": {"id": "2", "name": "In Progress"},
				"priority": {"id": "3", "name": "Medium"},
				"issuetype": {"id": "2", "name": "Task"},
				"labels": [],
				"created": "2025-03-20T09:00:00.000+0000",
				"updated": "2025-03-25T11:00:00.000+0000",
				"project": {"key": "PROJ", "name": "My Project"},
				"subtasks": [],
				"issuelinks": []
			}
		}
	]
}`

func TestSearchMyIssues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		jql := r.URL.Query().Get("jql")
		if jql != "assignee = currentUser() ORDER BY updated DESC" {
			t.Errorf("unexpected JQL: %s", jql)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(searchResponseJSON))
	}))
	defer server.Close()

	c := NewClient(server.URL, "user@test.com", "token")
	issues, total, err := c.SearchMyIssues(0, 50)
	if err != nil {
		t.Fatalf("SearchMyIssues failed: %v", err)
	}

	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(issues) != 2 {
		t.Fatalf("got %d issues, want 2", len(issues))
	}

	issue := issues[0]
	if issue.Key != "PROJ-1" {
		t.Errorf("Key = %q, want %q", issue.Key, "PROJ-1")
	}
	if issue.Summary != "Fix login bug" {
		t.Errorf("Summary = %q, want %q", issue.Summary, "Fix login bug")
	}
	if issue.Status.Name != "To Do" {
		t.Errorf("Status = %q, want %q", issue.Status.Name, "To Do")
	}
	if issue.Priority.Name != "High" {
		t.Errorf("Priority = %q, want %q", issue.Priority.Name, "High")
	}
	if issue.Assignee == nil {
		t.Fatal("Assignee is nil")
	}
	if issue.Assignee.DisplayName != "Chris" {
		t.Errorf("Assignee = %q, want %q", issue.Assignee.DisplayName, "Chris")
	}
	if len(issue.Labels) != 2 {
		t.Errorf("Labels count = %d, want 2", len(issue.Labels))
	}
	if issue.DueDate == nil {
		t.Fatal("DueDate is nil")
	}
	if issue.ProjectKey != "PROJ" {
		t.Errorf("ProjectKey = %q, want %q", issue.ProjectKey, "PROJ")
	}

	// Second issue has no assignee
	if issues[1].Assignee != nil {
		t.Error("second issue should have nil assignee")
	}
}

const issueDetailJSON = `{
	"key": "PROJ-1",
	"self": "https://test.atlassian.net/rest/api/3/issue/10001",
	"fields": {
		"summary": "Fix login bug",
		"description": {
			"type": "doc",
			"content": [
				{"type": "paragraph", "content": [{"type": "text", "text": "The login is broken."}]}
			]
		},
		"status": {"id": "1", "name": "To Do"},
		"priority": {"id": "2", "name": "High"},
		"issuetype": {"id": "1", "name": "Bug"},
		"assignee": {"accountId": "abc123", "displayName": "Chris"},
		"reporter": {"accountId": "def456", "displayName": "Alice"},
		"labels": ["backend"],
		"created": "2025-03-15T10:30:00.000+0000",
		"updated": "2025-03-28T14:00:00.000+0000",
		"project": {"key": "PROJ", "name": "My Project"},
		"parent": {
			"key": "PROJ-100",
			"fields": {
				"summary": "Auth Overhaul",
				"status": {"id": "2", "name": "In Progress"}
			}
		},
		"sprint": {"id": 14, "name": "Sprint 14"},
		"subtasks": [
			{
				"key": "PROJ-3",
				"fields": {
					"summary": "Add mutex",
					"status": {"id": "3", "name": "Done"},
					"priority": null,
					"issuetype": {"id": "3", "name": "Sub-task"},
					"labels": [],
					"created": "2025-03-16T10:00:00.000+0000",
					"updated": "2025-03-17T10:00:00.000+0000",
					"project": {"key": "PROJ", "name": "My Project"},
					"subtasks": [],
					"issuelinks": []
				}
			}
		],
		"issuelinks": [
			{
				"id": "1001",
				"type": {"name": "Blocks", "inward": "is blocked by", "outward": "blocks"},
				"outwardIssue": {
					"key": "PROJ-101",
					"fields": {
						"summary": "Deploy auth v2",
						"status": {"id": "1", "name": "To Do"}
					}
				}
			}
		],
		"comment": {
			"total": 1,
			"comments": [
				{
					"id": "5001",
					"author": {"accountId": "def456", "displayName": "Alice"},
					"body": {
						"type": "doc",
						"content": [
							{"type": "paragraph", "content": [{"type": "text", "text": "Can we handle the refresh endpoint being down?"}]}
						]
					},
					"created": "2025-03-27T10:00:00.000+0000",
					"updated": "2025-03-27T10:00:00.000+0000"
				}
			]
		}
	}
}`

func TestGetIssue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/PROJ-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(issueDetailJSON))
	}))
	defer server.Close()

	c := NewClient(server.URL, "user@test.com", "token")
	issue, err := c.GetIssue("PROJ-1")
	if err != nil {
		t.Fatalf("GetIssue failed: %v", err)
	}

	if issue.Key != "PROJ-1" {
		t.Errorf("Key = %q, want %q", issue.Key, "PROJ-1")
	}
	if issue.Description != "The login is broken." {
		t.Errorf("Description = %q, want %q", issue.Description, "The login is broken.")
	}
	if issue.Parent == nil {
		t.Fatal("Parent is nil")
	}
	if issue.Parent.Key != "PROJ-100" {
		t.Errorf("Parent.Key = %q, want %q", issue.Parent.Key, "PROJ-100")
	}
	if issue.Sprint != "Sprint 14" {
		t.Errorf("Sprint = %q, want %q", issue.Sprint, "Sprint 14")
	}
	if len(issue.Subtasks) != 1 {
		t.Fatalf("Subtasks count = %d, want 1", len(issue.Subtasks))
	}
	if issue.Subtasks[0].Key != "PROJ-3" {
		t.Errorf("Subtask key = %q, want %q", issue.Subtasks[0].Key, "PROJ-3")
	}
	if len(issue.Links) != 1 {
		t.Fatalf("Links count = %d, want 1", len(issue.Links))
	}
	if issue.Links[0].OutwardIssue.Key != "PROJ-101" {
		t.Errorf("Link outward key = %q, want %q", issue.Links[0].OutwardIssue.Key, "PROJ-101")
	}
	if len(issue.Comments) != 1 {
		t.Fatalf("Comments count = %d, want 1", len(issue.Comments))
	}
	if issue.Comments[0].Body != "Can we handle the refresh endpoint being down?" {
		t.Errorf("Comment body = %q", issue.Comments[0].Body)
	}
}

func TestSearchMyIssuesAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Unauthorized"}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "bad@test.com", "bad-token")
	_, _, err := c.SearchMyIssues(0, 50)
	if err == nil {
		t.Fatal("expected auth error, got nil")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("error = %q, want to contain 'authentication failed'", err.Error())
	}
}
```

- [ ] **Step 2: Add missing import to test file**

The `TestSearchMyIssuesAuthError` test uses `strings.Contains`. Add `"strings"` to the imports in `issues_test.go`:

The import block should be:
```go
import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)
```

- [ ] **Step 3: Run tests to verify they fail**

Run:
```bash
go test ./internal/jira/ -v
```
Expected: FAIL — `SearchMyIssues` and `GetIssue` not defined.

- [ ] **Step 4: Write the implementation**

Create `internal/jira/issues.go`:
```go
package jira

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/christopherdobbyn/jiratui/internal/models"
)

// SearchMyIssues fetches issues assigned to the current user.
// Returns the issues, total count, and any error.
func (c *Client) SearchMyIssues(startAt, maxResults int) ([]models.Issue, int, error) {
	jql := "assignee = currentUser() ORDER BY updated DESC"
	return c.searchIssues(jql, startAt, maxResults)
}

// SearchIssues fetches issues matching the given JQL query.
func (c *Client) SearchIssues(jql string, startAt, maxResults int) ([]models.Issue, int, error) {
	return c.searchIssues(jql, startAt, maxResults)
}

func (c *Client) searchIssues(jql string, startAt, maxResults int) ([]models.Issue, int, error) {
	params := url.Values{}
	params.Set("jql", jql)
	params.Set("startAt", strconv.Itoa(startAt))
	params.Set("maxResults", strconv.Itoa(maxResults))
	params.Set("fields", "summary,status,priority,issuetype,assignee,reporter,labels,created,updated,duedate,project,subtasks,issuelinks,parent,sprint")

	path := "/rest/api/3/search?" + params.Encode()
	data, err := c.get(path)
	if err != nil {
		return nil, 0, fmt.Errorf("searching issues: %w", err)
	}

	resp, err := decodeJSON[jiraSearchResponse](data)
	if err != nil {
		return nil, 0, fmt.Errorf("decoding search response: %w", err)
	}

	issues := make([]models.Issue, len(resp.Issues))
	for i, ji := range resp.Issues {
		issues[i] = mapIssue(ji, c.baseURL)
	}

	return issues, resp.Total, nil
}

// GetIssue fetches a single issue with full detail including comments.
func (c *Client) GetIssue(key string) (*models.Issue, error) {
	params := url.Values{}
	params.Set("fields", "summary,description,status,priority,issuetype,assignee,reporter,labels,created,updated,duedate,project,subtasks,issuelinks,parent,sprint,comment")

	path := "/rest/api/3/issue/" + key + "?" + params.Encode()
	data, err := c.get(path)
	if err != nil {
		return nil, fmt.Errorf("fetching issue %s: %w", key, err)
	}

	ji, err := decodeJSON[jiraIssue](data)
	if err != nil {
		return nil, fmt.Errorf("decoding issue %s: %w", key, err)
	}

	issue := mapIssue(ji, c.baseURL)
	return &issue, nil
}

func mapIssue(ji jiraIssue, baseURL string) models.Issue {
	f := ji.Fields

	issue := models.Issue{
		Key:         ji.Key,
		Summary:     f.Summary,
		Description: extractTextFromADF(f.Description),
		Status: models.Status{
			ID:   f.Status.ID,
			Name: f.Status.Name,
		},
		Type: models.IssueType{
			ID:   f.IssueType.ID,
			Name: f.IssueType.Name,
		},
		Assignee:    mapUser(f.Assignee),
		Reporter:    mapUser(f.Reporter),
		Labels:      f.Labels,
		Created:     parseTime(f.Created),
		Updated:     parseTime(f.Updated),
		DueDate:     parseDateOnly(f.DueDate),
		ProjectKey:  f.Project.Key,
		ProjectName: f.Project.Name,
		BrowseURL:   baseURL + "/browse/" + ji.Key,
	}

	if f.Priority != nil {
		issue.Priority = models.Priority{
			ID:   f.Priority.ID,
			Name: f.Priority.Name,
		}
	}

	if f.Parent != nil {
		issue.Parent = &models.IssueSummary{
			Key:     f.Parent.Key,
			Summary: f.Parent.Fields.Summary,
			Status: models.Status{
				ID:   f.Parent.Fields.Status.ID,
				Name: f.Parent.Fields.Status.Name,
			},
		}
	}

	if f.Sprint != nil {
		issue.Sprint = f.Sprint.Name
	}

	if f.Labels == nil {
		issue.Labels = []string{}
	}

	// Map subtasks
	issue.Subtasks = make([]models.IssueSummary, len(f.Subtasks))
	for i, st := range f.Subtasks {
		issue.Subtasks[i] = models.IssueSummary{
			Key:     st.Key,
			Summary: st.Fields.Summary,
			Status: models.Status{
				ID:   st.Fields.Status.ID,
				Name: st.Fields.Status.Name,
			},
		}
	}

	// Map links
	issue.Links = make([]models.IssueLink, len(f.IssueLinks))
	for i, jl := range f.IssueLinks {
		link := models.IssueLink{
			ID: jl.ID,
		}
		if jl.OutwardIssue != nil {
			link.Type = jl.Type.Outward
			link.OutwardIssue = &models.IssueSummary{
				Key:     jl.OutwardIssue.Key,
				Summary: jl.OutwardIssue.Fields.Summary,
				Status: models.Status{
					ID:   jl.OutwardIssue.Fields.Status.ID,
					Name: jl.OutwardIssue.Fields.Status.Name,
				},
			}
		}
		if jl.InwardIssue != nil {
			link.Type = jl.Type.Inward
			link.InwardIssue = &models.IssueSummary{
				Key:     jl.InwardIssue.Key,
				Summary: jl.InwardIssue.Fields.Summary,
				Status: models.Status{
					ID:   jl.InwardIssue.Fields.Status.ID,
					Name: jl.InwardIssue.Fields.Status.Name,
				},
			}
		}
		issue.Links[i] = link
	}

	// Map comments
	if f.Comment != nil {
		issue.Comments = make([]models.Comment, len(f.Comment.Comments))
		for i, jc := range f.Comment.Comments {
			author := mapUser(&jc.Author)
			issue.Comments[i] = models.Comment{
				ID:      jc.ID,
				Author:  *author,
				Body:    extractTextFromADF(jc.Body),
				Created: parseTime(jc.Created),
				Updated: parseTime(jc.Updated),
			}
		}
	}

	return issue
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run:
```bash
go test ./internal/jira/ -v
```
Expected: All 7 tests PASS (4 from client_test.go + 3 from issues_test.go).

- [ ] **Step 6: Commit**

```bash
git add internal/jira/issues.go internal/jira/issues_test.go
git commit -m "feat: add issue search and detail fetching with domain model mapping"
```

---

## Task 10: Issues CLI Command (stdout testing)

**Files:**
- Create: `cmd/issues.go`

- [ ] **Step 1: Create the issues command**

Create `cmd/issues.go`:
```go
package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/christopherdobbyn/jiratui/internal/config"
	"github.com/christopherdobbyn/jiratui/internal/jira"
	"github.com/spf13/cobra"
)

var issuesCmd = &cobra.Command{
	Use:   "issues",
	Short: "List issues assigned to you",
	RunE:  runIssues,
}

func init() {
	rootCmd.AddCommand(issuesCmd)
}

func runIssues(cmd *cobra.Command, args []string) error {
	cfgPath := config.DefaultPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	profile, err := cfg.ActiveProfileConfig()
	if err != nil {
		return err
	}

	client := jira.NewClient(profile.URL, profile.Email, profile.APIToken)

	fmt.Printf("Fetching issues from %s...\n\n", profile.URL)

	issues, total, err := client.SearchMyIssues(0, 50)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "KEY\tPRIORITY\tSTATUS\tASSIGNEE\tSUMMARY\n")
	fmt.Fprintf(w, "---\t--------\t------\t--------\t-------\n")

	for _, issue := range issues {
		assignee := "-"
		if issue.Assignee != nil {
			assignee = issue.Assignee.DisplayName
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			issue.Key,
			issue.Priority.Name,
			issue.Status.Name,
			assignee,
			issue.Summary,
		)
	}

	w.Flush()
	fmt.Printf("\nShowing %d of %d issues\n", len(issues), total)

	return nil
}
```

- [ ] **Step 2: Build and verify**

Run:
```bash
go build -o jiratui .
```
Expected: Builds successfully.

- [ ] **Step 3: Manual integration test**

Run (after setting up auth with `./jiratui auth add`):
```bash
./jiratui issues
```
Expected: Table of your real Jira issues printed to stdout. Verify a few ticket keys and summaries match what you see in the Jira web UI.

- [ ] **Step 4: Commit**

```bash
git add cmd/issues.go
git commit -m "feat: add issues CLI command for testing API integration"
```

---

## Task 11: Run All Tests + Final Verification

- [ ] **Step 1: Run full test suite**

Run:
```bash
go test ./... -v
```
Expected: All tests PASS across `internal/config/` and `internal/jira/`.

- [ ] **Step 2: Verify binary builds cleanly**

Run:
```bash
go build -o jiratui . && ls -lh jiratui
```
Expected: Single binary, ~10-15MB.

- [ ] **Step 3: Verify go vet and basic linting**

Run:
```bash
go vet ./...
```
Expected: No issues.

- [ ] **Step 4: Test the full auth → issues flow end-to-end**

Run:
```bash
# If you haven't already added auth:
./jiratui auth add
# Then:
./jiratui auth list
./jiratui issues
```
Expected: Auth flow prompts for credentials, saves config. Auth list shows the profile. Issues command shows your real Jira tickets.

- [ ] **Step 5: Commit any remaining changes**

```bash
git status
# If any unstaged changes, add and commit
```
