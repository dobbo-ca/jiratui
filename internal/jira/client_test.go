package jira

import (
	"encoding/json"
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
	if gotAuth[:6] != "Basic " {
		t.Errorf("Authorization header should start with 'Basic ', got %q", gotAuth[:6])
	}
}

func TestADFToMarkdown(t *testing.T) {
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

	got := adfToMarkdown(adf)
	want := "Hello world\n\nSecond paragraph"
	if got != want {
		t.Errorf("adfToMarkdown = %q, want %q", got, want)
	}
}

func TestADFToMarkdownNil(t *testing.T) {
	got := adfToMarkdown(nil)
	if got != "" {
		t.Errorf("adfToMarkdown(nil) = %q, want empty string", got)
	}
}

func TestADFToMarkdownHeadings(t *testing.T) {
	adf := &jiraADF{
		Type: "doc",
		Content: []jiraNode{
			{
				Type:  "heading",
				Attrs: map[string]any{"level": float64(2)},
				Content: []jiraNode{
					{Type: "text", Text: "Context"},
				},
			},
			{
				Type: "paragraph",
				Content: []jiraNode{
					{Type: "text", Text: "Some text"},
				},
			},
		},
	}

	got := adfToMarkdown(adf)
	want := "## Context\n\nSome text"
	if got != want {
		t.Errorf("adfToMarkdown = %q, want %q", got, want)
	}
}

func TestADFToMarkdownBulletList(t *testing.T) {
	adf := &jiraADF{
		Type: "doc",
		Content: []jiraNode{
			{
				Type: "bulletList",
				Content: []jiraNode{
					{
						Type: "listItem",
						Content: []jiraNode{
							{
								Type: "paragraph",
								Content: []jiraNode{
									{Type: "text", Text: "First item"},
								},
							},
						},
					},
					{
						Type: "listItem",
						Content: []jiraNode{
							{
								Type: "paragraph",
								Content: []jiraNode{
									{Type: "text", Text: "Second item"},
								},
							},
						},
					},
				},
			},
		},
	}

	got := adfToMarkdown(adf)
	want := "- First item\n- Second item"
	if got != want {
		t.Errorf("adfToMarkdown = %q, want %q", got, want)
	}
}

func TestADFToMarkdownOrderedList(t *testing.T) {
	adf := &jiraADF{
		Type: "doc",
		Content: []jiraNode{
			{
				Type: "orderedList",
				Attrs: map[string]any{"order": float64(1)},
				Content: []jiraNode{
					{
						Type: "listItem",
						Content: []jiraNode{
							{
								Type: "paragraph",
								Content: []jiraNode{
									{Type: "text", Text: "Step one"},
								},
							},
						},
					},
					{
						Type: "listItem",
						Content: []jiraNode{
							{
								Type: "paragraph",
								Content: []jiraNode{
									{Type: "text", Text: "Step two"},
								},
							},
						},
					},
				},
			},
		},
	}

	got := adfToMarkdown(adf)
	want := "1. Step one\n1. Step two"
	if got != want {
		t.Errorf("adfToMarkdown = %q, want %q", got, want)
	}
}

func TestADFToMarkdownMarks(t *testing.T) {
	adf := &jiraADF{
		Type: "doc",
		Content: []jiraNode{
			{
				Type: "paragraph",
				Content: []jiraNode{
					{Type: "text", Text: "Use "},
					{
						Type: "text",
						Text: "principal_id",
						Marks: []jiraMark{{Type: "code"}},
					},
					{Type: "text", Text: " and "},
					{
						Type: "text",
						Text: "bold text",
						Marks: []jiraMark{{Type: "strong"}},
					},
				},
			},
		},
	}

	got := adfToMarkdown(adf)
	want := "Use `principal_id` and **bold text**"
	if got != want {
		t.Errorf("adfToMarkdown = %q, want %q", got, want)
	}
}

func TestADFToMarkdownLink(t *testing.T) {
	adf := &jiraADF{
		Type: "doc",
		Content: []jiraNode{
			{
				Type: "paragraph",
				Content: []jiraNode{
					{Type: "text", Text: "See "},
					{
						Type: "text",
						Text: "docs",
						Marks: []jiraMark{{
							Type:  "link",
							Attrs: map[string]any{"href": "https://example.com"},
						}},
					},
				},
			},
		},
	}

	got := adfToMarkdown(adf)
	want := "See [docs](https://example.com)"
	if got != want {
		t.Errorf("adfToMarkdown = %q, want %q", got, want)
	}
}

func TestMarkdownToADFParagraph(t *testing.T) {
	adf := markdownToADF("Hello world")
	if len(adf.Content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(adf.Content))
	}
	if adf.Content[0].Type != "paragraph" {
		t.Errorf("expected paragraph, got %q", adf.Content[0].Type)
	}
	if len(adf.Content[0].Content) != 1 || adf.Content[0].Content[0].Text != "Hello world" {
		t.Errorf("unexpected text content: %+v", adf.Content[0].Content)
	}
}

func TestMarkdownToADFHeading(t *testing.T) {
	adf := markdownToADF("## Context")
	if len(adf.Content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(adf.Content))
	}
	node := adf.Content[0]
	if node.Type != "heading" {
		t.Errorf("expected heading, got %q", node.Type)
	}
	if level, ok := node.Attrs["level"].(float64); !ok || level != 2 {
		t.Errorf("expected level 2, got %v", node.Attrs["level"])
	}
	if len(node.Content) != 1 || node.Content[0].Text != "Context" {
		t.Errorf("unexpected heading content: %+v", node.Content)
	}
}

func TestMarkdownToADFBold(t *testing.T) {
	adf := markdownToADF("Use **bold** text")
	if len(adf.Content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(adf.Content))
	}
	nodes := adf.Content[0].Content
	if len(nodes) != 3 {
		t.Fatalf("expected 3 inline nodes, got %d", len(nodes))
	}
	if nodes[0].Text != "Use " {
		t.Errorf("first node text = %q, want %q", nodes[0].Text, "Use ")
	}
	if nodes[1].Text != "bold" || len(nodes[1].Marks) != 1 || nodes[1].Marks[0].Type != "strong" {
		t.Errorf("second node should be bold 'bold', got %+v", nodes[1])
	}
	if nodes[2].Text != " text" {
		t.Errorf("third node text = %q, want %q", nodes[2].Text, " text")
	}
}

func TestMarkdownToADFBulletList(t *testing.T) {
	adf := markdownToADF("- item one\n- item two")
	if len(adf.Content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(adf.Content))
	}
	list := adf.Content[0]
	if list.Type != "bulletList" {
		t.Errorf("expected bulletList, got %q", list.Type)
	}
	if len(list.Content) != 2 {
		t.Fatalf("expected 2 listItems, got %d", len(list.Content))
	}
	for i, item := range list.Content {
		if item.Type != "listItem" {
			t.Errorf("item %d: expected listItem, got %q", i, item.Type)
		}
	}
}

func TestMarkdownToADFOrderedList(t *testing.T) {
	adf := markdownToADF("1. step one\n2. step two")
	if len(adf.Content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(adf.Content))
	}
	list := adf.Content[0]
	if list.Type != "orderedList" {
		t.Errorf("expected orderedList, got %q", list.Type)
	}
	if len(list.Content) != 2 {
		t.Fatalf("expected 2 listItems, got %d", len(list.Content))
	}
}

func TestMarkdownToADFRoundTrip(t *testing.T) {
	original := "## Heading\n\nSome **bold** and `code` text\n\n- item one\n- item two"
	adf := markdownToADF(original)
	result := adfToMarkdown(adf)
	if result != original {
		t.Errorf("round-trip mismatch:\ngot:  %q\nwant: %q", result, original)
	}
}

func TestMarkdownToADFEmpty(t *testing.T) {
	adf := markdownToADF("")
	if adf == nil {
		t.Fatal("expected non-nil ADF")
	}
	if adf.Version != 1 {
		t.Errorf("expected version 1, got %d", adf.Version)
	}
	if len(adf.Content) != 0 {
		t.Errorf("expected 0 content blocks, got %d", len(adf.Content))
	}
}

func TestMarkdownToADFLink(t *testing.T) {
	adf := markdownToADF("See [docs](https://example.com) here")
	if len(adf.Content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(adf.Content))
	}
	nodes := adf.Content[0].Content
	if len(nodes) != 3 {
		t.Fatalf("expected 3 inline nodes, got %d", len(nodes))
	}
	if nodes[0].Text != "See " {
		t.Errorf("first node text = %q, want %q", nodes[0].Text, "See ")
	}
	linkNode := nodes[1]
	if linkNode.Text != "docs" {
		t.Errorf("link text = %q, want %q", linkNode.Text, "docs")
	}
	if len(linkNode.Marks) != 1 || linkNode.Marks[0].Type != "link" {
		t.Errorf("expected link mark, got %+v", linkNode.Marks)
	}
	if href, ok := linkNode.Marks[0].Attrs["href"]; !ok || href != "https://example.com" {
		t.Errorf("expected href https://example.com, got %v", linkNode.Marks[0].Attrs["href"])
	}
	if nodes[2].Text != " here" {
		t.Errorf("third node text = %q, want %q", nodes[2].Text, " here")
	}
}

func TestVerifyCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/myself" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"accountId":"abc123","displayName":"Chris D","emailAddress":"chris@test.com","active":true}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "user@test.com", "token")
	name, err := c.VerifyCredentials()
	if err != nil {
		t.Fatalf("VerifyCredentials failed: %v", err)
	}
	if name != "Chris D" {
		t.Errorf("displayName = %q, want %q", name, "Chris D")
	}
}

func TestVerifyCredentialsDeactivated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"accountId":"abc123","displayName":"Chris D","emailAddress":"chris@test.com","active":false}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "user@test.com", "token")
	_, err := c.VerifyCredentials()
	if err == nil {
		t.Fatal("expected error for deactivated account, got nil")
	}
}

func TestVerifyCredentialsAuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Unauthorized"}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "bad@test.com", "bad-token")
	_, err := c.VerifyCredentials()
	if err == nil {
		t.Fatal("expected auth error, got nil")
	}
}

func TestGetLinkTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issueLinkType" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"issueLinkTypes":[{"id":"10000","name":"Blocks","inward":"is blocked by","outward":"blocks"},{"id":"10001","name":"Relates","inward":"relates to","outward":"relates to"}]}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "user@test.com", "token")
	types, err := c.GetLinkTypes()
	if err != nil {
		t.Fatalf("GetLinkTypes failed: %v", err)
	}
	if len(types) != 2 {
		t.Fatalf("expected 2 link types, got %d", len(types))
	}
	if types[0].Name != "Blocks" || types[0].Inward != "is blocked by" {
		t.Errorf("unexpected first type: %+v", types[0])
	}
}

func TestCreateLink(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/rest/api/3/issueLink" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(201)
	}))
	defer server.Close()

	c := NewClient(server.URL, "user@test.com", "token")
	err := c.CreateLink("TEC-100", "TEC-200", "Blocks", "outward")
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}
	inward := gotBody["inwardIssue"].(map[string]any)
	outward := gotBody["outwardIssue"].(map[string]any)
	if inward["key"] != "TEC-200" || outward["key"] != "TEC-100" {
		t.Errorf("unexpected link direction: inward=%v outward=%v", inward, outward)
	}
}

func TestDeleteLink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" || r.URL.Path != "/rest/api/3/issueLink/12345" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(200)
	}))
	defer server.Close()

	c := NewClient(server.URL, "user@test.com", "token")
	err := c.DeleteLink("12345")
	if err != nil {
		t.Fatalf("DeleteLink failed: %v", err)
	}
}
