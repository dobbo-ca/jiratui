package jira

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const searchResponseJSON = `{
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
	],
	"nextPageToken": "",
	"isLast": true
}`

func TestSearchMyIssues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/search/jql" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		jql := r.URL.Query().Get("jql")
		if jql != "assignee = currentUser() AND statusCategory != Done ORDER BY updated DESC" {
			t.Errorf("unexpected JQL: %s", jql)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(searchResponseJSON))
	}))
	defer server.Close()

	c := NewClient(server.URL, "user@test.com", "token")
	result, err := c.SearchMyIssues(50, "", "", "")
	if err != nil {
		t.Fatalf("SearchMyIssues failed: %v", err)
	}

	if !result.IsLast {
		t.Error("expected IsLast to be true")
	}
	if len(result.Issues) != 2 {
		t.Fatalf("got %d issues, want 2", len(result.Issues))
	}

	issue := result.Issues[0]
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
	if result.Issues[1].Assignee != nil {
		t.Error("second issue should have nil assignee")
	}
}

func TestSearchMyIssuesPagination(t *testing.T) {
	const page1JSON = `{
		"issues": [{"key": "PROJ-1", "fields": {"summary": "First", "status": {"id": "1", "name": "To Do"}, "issuetype": {"id": "1", "name": "Task"}, "labels": [], "created": "2025-03-15T10:00:00.000+0000", "updated": "2025-03-15T10:00:00.000+0000", "project": {"key": "PROJ", "name": "P"}, "subtasks": [], "issuelinks": []}}],
		"nextPageToken": "page2token",
		"isLast": false
	}`
	const page2JSON = `{
		"issues": [{"key": "PROJ-2", "fields": {"summary": "Second", "status": {"id": "1", "name": "To Do"}, "issuetype": {"id": "1", "name": "Task"}, "labels": [], "created": "2025-03-15T10:00:00.000+0000", "updated": "2025-03-15T10:00:00.000+0000", "project": {"key": "PROJ", "name": "P"}, "subtasks": [], "issuelinks": []}}],
		"nextPageToken": "",
		"isLast": true
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("nextPageToken")
		w.Header().Set("Content-Type", "application/json")
		if token == "page2token" {
			w.Write([]byte(page2JSON))
		} else {
			w.Write([]byte(page1JSON))
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "user@test.com", "token")

	// Page 1
	result, err := c.SearchMyIssues(1, "", "", "")
	if err != nil {
		t.Fatalf("page 1 failed: %v", err)
	}
	if result.IsLast {
		t.Error("page 1 should not be last")
	}
	if result.NextPageToken != "page2token" {
		t.Errorf("NextPageToken = %q, want %q", result.NextPageToken, "page2token")
	}
	if result.Issues[0].Key != "PROJ-1" {
		t.Errorf("page 1 Key = %q, want PROJ-1", result.Issues[0].Key)
	}

	// Page 2
	result, err = c.SearchMyIssues(1, result.NextPageToken, "", "")
	if err != nil {
		t.Fatalf("page 2 failed: %v", err)
	}
	if !result.IsLast {
		t.Error("page 2 should be last")
	}
	if result.Issues[0].Key != "PROJ-2" {
		t.Errorf("page 2 Key = %q, want PROJ-2", result.Issues[0].Key)
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
	_, err := c.SearchMyIssues(50, "", "", "")
	if err == nil {
		t.Fatal("expected auth error, got nil")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("error = %q, want to contain 'authentication failed'", err.Error())
	}
}
