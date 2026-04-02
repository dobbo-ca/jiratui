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
