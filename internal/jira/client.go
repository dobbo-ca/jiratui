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

// jiraMyselfResponse is the response from /rest/api/3/myself
type jiraMyselfResponse struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress"`
	Active      bool   `json:"active"`
}

// VerifyCredentials checks that the configured credentials are valid by
// calling /rest/api/3/myself. Returns the user's display name on success.
func (c *Client) VerifyCredentials() (string, error) {
	data, err := c.get("/rest/api/3/myself")
	if err != nil {
		return "", err
	}

	resp, err := decodeJSON[jiraMyselfResponse](data)
	if err != nil {
		return "", fmt.Errorf("decoding user info: %w", err)
	}

	if !resp.Active {
		return "", fmt.Errorf("account %q is deactivated", resp.DisplayName)
	}

	return resp.DisplayName, nil
}
