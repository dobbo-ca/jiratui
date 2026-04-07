package jira

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
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

// DownloadURL fetches raw bytes from an absolute URL with auth headers.
func (c *Client) DownloadURL(absoluteURL string) ([]byte, error) {
	req, err := http.NewRequest("GET", absoluteURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	auth := base64.StdEncoding.EncodeToString([]byte(c.email + ":" + c.apiToken))
	req.Header.Set("Authorization", "Basic "+auth)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading attachment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download failed (HTTP %d)", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// UploadAttachment uploads a file as an attachment to an issue.
func (c *Client) UploadAttachment(issueKey, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("creating form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return fmt.Errorf("copying file data: %w", err)
	}
	writer.Close()

	url := c.baseURL + "/rest/api/3/issue/" + issueKey + "/attachments"
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(c.email + ":" + c.apiToken))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Atlassian-Token", "no-check")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("uploading attachment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed (HTTP %d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) put(path string, jsonBody []byte) error {
	url := c.baseURL + path

	req, err := http.NewRequest("PUT", url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(c.email + ":" + c.apiToken))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Jira API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) post(path string, jsonBody []byte) error {
	url := c.baseURL + path

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(c.email + ":" + c.apiToken))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Jira API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) deleteReq(path string) error {
	url := c.baseURL + path

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(c.email + ":" + c.apiToken))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Jira API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func decodeJSON[T any](data []byte) (T, error) {
	var result T
	err := json.Unmarshal(data, &result)
	return result, err
}

// adfToMarkdown converts Atlassian Document Format to markdown.
func adfToMarkdown(adf *jiraADF) string {
	return adfToMarkdownWithMedia(adf, nil)
}

// adfToMarkdownWithMedia converts ADF to markdown, resolving media node IDs
// to filenames using the provided attachment map (id → filename).
func adfToMarkdownWithMedia(adf *jiraADF, attachments map[string]string) string {
	if adf == nil {
		return ""
	}

	var blocks []string
	for _, node := range adf.Content {
		block := renderBlock(node, "", attachments)
		if block != "" {
			blocks = append(blocks, block)
		}
	}
	return strings.Join(blocks, "\n\n")
}

// renderBlock converts a top-level ADF block node to markdown.
func renderBlock(node jiraNode, prefix string, attachments map[string]string) string {
	switch node.Type {
	case "paragraph":
		return prefix + renderInline(node.Content, attachments)

	case "heading":
		level := 1
		if l, ok := node.Attrs["level"]; ok {
			if lf, ok := l.(float64); ok {
				level = int(lf)
			}
		}
		hashes := strings.Repeat("#", level)
		return hashes + " " + renderInline(node.Content, attachments)

	case "bulletList":
		return renderList(node.Content, "- ", prefix, attachments)

	case "orderedList":
		return renderList(node.Content, "1. ", prefix, attachments)

	case "listItem":
		var parts []string
		for i, child := range node.Content {
			if i == 0 {
				parts = append(parts, renderBlock(child, prefix, attachments))
			} else {
				parts = append(parts, renderBlock(child, prefix+"  ", attachments))
			}
		}
		return strings.Join(parts, "\n")

	case "codeBlock":
		lang := ""
		if l, ok := node.Attrs["language"]; ok {
			if ls, ok := l.(string); ok {
				lang = ls
			}
		}
		return "```" + lang + "\n" + renderInline(node.Content, attachments) + "\n```"

	case "blockquote":
		var lines []string
		for _, child := range node.Content {
			lines = append(lines, "> "+renderBlock(child, "", attachments))
		}
		return strings.Join(lines, "\n")

	case "rule":
		return "---"

	case "hardBreak":
		return "\n"

	case "table":
		return renderTable(node)

	case "mediaSingle", "mediaGroup":
		// Container for media nodes — render each child
		var parts []string
		for _, child := range node.Content {
			part := renderMediaNode(child, attachments)
			if part != "" {
				parts = append(parts, part)
			}
		}
		return prefix + strings.Join(parts, " ")

	default:
		// Fallback: extract inline text
		return prefix + renderInline(node.Content, attachments)
	}
}

// renderList converts list items with the given bullet prefix.
func renderList(items []jiraNode, bullet, outerPrefix string, attachments map[string]string) string {
	var lines []string
	for _, item := range items {
		if item.Type != "listItem" {
			continue
		}
		var parts []string
		for i, child := range item.Content {
			if i == 0 {
				if child.Type == "paragraph" {
					parts = append(parts, outerPrefix+bullet+renderInline(child.Content, attachments))
				} else {
					parts = append(parts, renderBlock(child, outerPrefix+"  ", attachments))
				}
			} else {
				indent := outerPrefix + strings.Repeat(" ", len(bullet))
				parts = append(parts, renderBlock(child, indent, attachments))
			}
		}
		lines = append(lines, strings.Join(parts, "\n"))
	}
	return strings.Join(lines, "\n")
}

// renderMediaNode converts a media ADF node to a markdown placeholder.
func renderMediaNode(node jiraNode, attachments map[string]string) string {
	if node.Type != "media" {
		return ""
	}
	// Try to resolve filename from attachment ID
	if id, ok := node.Attrs["id"]; ok {
		idStr := fmt.Sprint(id)
		if name, found := attachments[idStr]; found {
			return "[" + name + "]"
		}
	}
	// Fallback: use alt text or generic label
	if alt, ok := node.Attrs["alt"]; ok && fmt.Sprint(alt) != "" {
		return "[" + fmt.Sprint(alt) + "]"
	}
	return "[image]"
}

// renderInline converts a slice of inline ADF nodes to markdown text.
func renderInline(nodes []jiraNode, attachments map[string]string) string {
	var sb strings.Builder
	for _, node := range nodes {
		switch node.Type {
		case "text":
			text := node.Text
			for _, mark := range node.Marks {
				switch mark.Type {
				case "strong":
					text = "**" + text + "**"
				case "em":
					text = "*" + text + "*"
				case "code":
					text = "`" + text + "`"
				case "strike":
					text = "~~" + text + "~~"
				case "link":
					if href, ok := mark.Attrs["href"]; ok {
						text = "[" + text + "](" + fmt.Sprint(href) + ")"
					}
				}
			}
			sb.WriteString(text)
		case "hardBreak":
			sb.WriteString("\n")
		case "mention":
			if name, ok := node.Attrs["text"]; ok {
				sb.WriteString(fmt.Sprint(name))
			}
		case "emoji":
			if shortName, ok := node.Attrs["shortName"]; ok {
				sb.WriteString(fmt.Sprint(shortName))
			}
		case "inlineCard":
			if url, ok := node.Attrs["url"]; ok {
				sb.WriteString(fmt.Sprint(url))
			}
		case "media":
			sb.WriteString(renderMediaNode(node, attachments))
		default:
			sb.WriteString(renderInline(node.Content, attachments))
		}
	}
	return sb.String()
}

// renderTable converts a table ADF node to markdown.
func renderTable(node jiraNode) string {
	var rows [][]string
	for _, row := range node.Content {
		if row.Type != "tableRow" {
			continue
		}
		var cells []string
		for _, cell := range row.Content {
			var parts []string
			for _, child := range cell.Content {
				parts = append(parts, renderBlock(child, "", nil))
			}
			cells = append(cells, strings.Join(parts, " "))
		}
		rows = append(rows, cells)
	}
	if len(rows) == 0 {
		return ""
	}

	var sb strings.Builder
	// Header row
	sb.WriteString("| " + strings.Join(rows[0], " | ") + " |")
	// Separator
	sep := make([]string, len(rows[0]))
	for i := range sep {
		sep[i] = "---"
	}
	sb.WriteString("\n| " + strings.Join(sep, " | ") + " |")
	// Data rows
	for _, row := range rows[1:] {
		sb.WriteString("\n| " + strings.Join(row, " | ") + " |")
	}
	return sb.String()
}

// markdownToADF converts markdown text to Atlassian Document Format.
func markdownToADF(md string) *jiraADF {
	doc := &jiraADF{
		Type:    "doc",
		Version: 1,
		Content: []jiraNode{},
	}

	if md == "" {
		return doc
	}

	lines := strings.Split(md, "\n")
	i := 0
	for i < len(lines) {
		line := lines[i]

		// Horizontal rule
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" || trimmed == "***" || trimmed == "___" {
			doc.Content = append(doc.Content, jiraNode{Type: "rule"})
			i++
			continue
		}

		// Empty line — skip
		if trimmed == "" {
			i++
			continue
		}

		// Heading
		if strings.HasPrefix(line, "#") {
			level := 0
			for level < len(line) && line[level] == '#' {
				level++
			}
			if level > 0 && level < len(line) && line[level] == ' ' {
				text := strings.TrimSpace(line[level+1:])
				doc.Content = append(doc.Content, jiraNode{
					Type:    "heading",
					Attrs:   map[string]any{"level": float64(level)},
					Content: parseInlineMarkdown(text),
				})
				i++
				continue
			}
		}

		// Blockquote: collect consecutive > lines
		if strings.HasPrefix(line, "> ") || line == ">" {
			var bqLines []string
			for i < len(lines) && (strings.HasPrefix(lines[i], "> ") || lines[i] == ">") {
				stripped := strings.TrimPrefix(lines[i], "> ")
				stripped = strings.TrimPrefix(stripped, ">")
				bqLines = append(bqLines, stripped)
				i++
			}
			inner := markdownToADF(strings.Join(bqLines, "\n"))
			doc.Content = append(doc.Content, jiraNode{
				Type:    "blockquote",
				Content: inner.Content,
			})
			continue
		}

		// Bullet list: lines starting with - , * , or +
		if (strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ")) {
			var items []jiraNode
			for i < len(lines) && (strings.HasPrefix(lines[i], "- ") || strings.HasPrefix(lines[i], "* ") || strings.HasPrefix(lines[i], "+ ")) {
				text := lines[i][2:]
				items = append(items, jiraNode{
					Type: "listItem",
					Content: []jiraNode{
						{
							Type:    "paragraph",
							Content: parseInlineMarkdown(text),
						},
					},
				})
				i++
			}
			doc.Content = append(doc.Content, jiraNode{
				Type:    "bulletList",
				Content: items,
			})
			continue
		}

		// Ordered list: lines starting with N.
		if isOrderedListLine(line) {
			var items []jiraNode
			for i < len(lines) && isOrderedListLine(lines[i]) {
				text := stripOrderedPrefix(lines[i])
				items = append(items, jiraNode{
					Type: "listItem",
					Content: []jiraNode{
						{
							Type:    "paragraph",
							Content: parseInlineMarkdown(text),
						},
					},
				})
				i++
			}
			doc.Content = append(doc.Content, jiraNode{
				Type:    "orderedList",
				Content: items,
			})
			continue
		}

		// Default: paragraph
		doc.Content = append(doc.Content, jiraNode{
			Type:    "paragraph",
			Content: parseInlineMarkdown(line),
		})
		i++
	}

	return doc
}

// isOrderedListLine checks if a line starts with a digit followed by a dot within the first 3 chars.
func isOrderedListLine(line string) bool {
	if len(line) < 3 {
		return false
	}
	dotIdx := strings.Index(line, ".")
	if dotIdx < 1 || dotIdx > 2 {
		return false
	}
	for j := 0; j < dotIdx; j++ {
		if line[j] < '0' || line[j] > '9' {
			return false
		}
	}
	if dotIdx+1 < len(line) && line[dotIdx+1] == ' ' {
		return true
	}
	return false
}

// stripOrderedPrefix removes "N. " from the start of an ordered list line.
func stripOrderedPrefix(line string) string {
	dotIdx := strings.Index(line, ".")
	if dotIdx >= 0 && dotIdx+2 <= len(line) {
		return line[dotIdx+2:]
	}
	return line
}

// parseInlineMarkdown parses inline markdown formatting into ADF text nodes.
func parseInlineMarkdown(text string) []jiraNode {
	if text == "" {
		return []jiraNode{{Type: "text", Text: ""}}
	}

	var nodes []jiraNode
	var current strings.Builder
	runes := []rune(text)
	i := 0

	flushCurrent := func() {
		if current.Len() > 0 {
			nodes = append(nodes, jiraNode{Type: "text", Text: current.String()})
			current.Reset()
		}
	}

	for i < len(runes) {
		// Bold: **text**
		if i+1 < len(runes) && runes[i] == '*' && runes[i+1] == '*' {
			end := findClosing(runes, i+2, "**")
			if end >= 0 {
				flushCurrent()
				inner := string(runes[i+2 : end])
				nodes = append(nodes, jiraNode{
					Type:  "text",
					Text:  inner,
					Marks: []jiraMark{{Type: "strong"}},
				})
				i = end + 2
				continue
			}
		}

		// Strikethrough: ~~text~~
		if i+1 < len(runes) && runes[i] == '~' && runes[i+1] == '~' {
			end := findClosing(runes, i+2, "~~")
			if end >= 0 {
				flushCurrent()
				inner := string(runes[i+2 : end])
				nodes = append(nodes, jiraNode{
					Type:  "text",
					Text:  inner,
					Marks: []jiraMark{{Type: "strike"}},
				})
				i = end + 2
				continue
			}
		}

		// Italic: *text*
		if runes[i] == '*' {
			end := findClosingRune(runes, i+1, '*')
			if end >= 0 {
				flushCurrent()
				inner := string(runes[i+1 : end])
				nodes = append(nodes, jiraNode{
					Type:  "text",
					Text:  inner,
					Marks: []jiraMark{{Type: "em"}},
				})
				i = end + 1
				continue
			}
		}

		// Inline code: `text`
		if runes[i] == '`' {
			end := findClosingRune(runes, i+1, '`')
			if end >= 0 {
				flushCurrent()
				inner := string(runes[i+1 : end])
				nodes = append(nodes, jiraNode{
					Type:  "text",
					Text:  inner,
					Marks: []jiraMark{{Type: "code"}},
				})
				i = end + 1
				continue
			}
		}

		// Link: [text](url)
		if runes[i] == '[' {
			closeBracket := findClosingRune(runes, i+1, ']')
			if closeBracket >= 0 && closeBracket+1 < len(runes) && runes[closeBracket+1] == '(' {
				closeParen := findClosingRune(runes, closeBracket+2, ')')
				if closeParen >= 0 {
					flushCurrent()
					linkText := string(runes[i+1 : closeBracket])
					linkURL := string(runes[closeBracket+2 : closeParen])
					nodes = append(nodes, jiraNode{
						Type: "text",
						Text: linkText,
						Marks: []jiraMark{{
							Type:  "link",
							Attrs: map[string]any{"href": linkURL},
						}},
					})
					i = closeParen + 1
					continue
				}
			}
		}

		current.WriteRune(runes[i])
		i++
	}

	flushCurrent()

	if len(nodes) == 0 {
		return []jiraNode{{Type: "text", Text: ""}}
	}
	return nodes
}

// findClosing finds the position of a multi-char closing delimiter in runes starting from pos.
func findClosing(runes []rune, start int, delim string) int {
	dr := []rune(delim)
	for i := start; i <= len(runes)-len(dr); i++ {
		match := true
		for j := 0; j < len(dr); j++ {
			if runes[i+j] != dr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// findClosingRune finds the position of a single-char closing delimiter.
func findClosingRune(runes []rune, start int, ch rune) int {
	for i := start; i < len(runes); i++ {
		if runes[i] == ch {
			return i
		}
	}
	return -1
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

// GetMyself returns the current user's account ID and display name.
func (c *Client) GetMyself() (accountID, displayName string, err error) {
	data, err := c.get("/rest/api/3/myself")
	if err != nil {
		return "", "", err
	}
	resp, err := decodeJSON[jiraMyselfResponse](data)
	if err != nil {
		return "", "", err
	}
	return resp.AccountID, resp.DisplayName, nil
}

// AddComment adds a new comment to an issue.
func (c *Client) AddComment(issueKey, markdownBody string) error {
	adf := markdownToADF(markdownBody)
	payload, err := json.Marshal(map[string]any{"body": adf})
	if err != nil {
		return fmt.Errorf("marshaling comment: %w", err)
	}
	return c.post("/rest/api/3/issue/"+issueKey+"/comment", payload)
}

// UpdateComment updates an existing comment on an issue.
func (c *Client) UpdateComment(issueKey, commentID, markdownBody string) error {
	adf := markdownToADF(markdownBody)
	payload, err := json.Marshal(map[string]any{"body": adf})
	if err != nil {
		return fmt.Errorf("marshaling comment: %w", err)
	}
	return c.put("/rest/api/3/issue/"+issueKey+"/comment/"+commentID, payload)
}

// DeleteComment deletes a comment from an issue.
func (c *Client) DeleteComment(issueKey, commentID string) error {
	return c.deleteReq("/rest/api/3/issue/" + issueKey + "/comment/" + commentID)
}

// GetLinkTypes returns all available issue link types.
func (c *Client) GetLinkTypes() ([]models.LinkType, error) {
	data, err := c.get("/rest/api/3/issueLinkType")
	if err != nil {
		return nil, fmt.Errorf("fetching link types: %w", err)
	}
	resp, err := decodeJSON[jiraLinkTypesResponse](data)
	if err != nil {
		return nil, fmt.Errorf("decoding link types: %w", err)
	}
	types := make([]models.LinkType, len(resp.IssueLinkTypes))
	for i, jlt := range resp.IssueLinkTypes {
		types[i] = models.LinkType{
			ID:      jlt.ID,
			Name:    jlt.Name,
			Inward:  jlt.Inward,
			Outward: jlt.Outward,
		}
	}
	return types, nil
}

// CreateLink creates an issue link between two issues.
func (c *Client) CreateLink(issueKey, targetKey, linkTypeName, direction string) error {
	inward := issueKey
	outward := targetKey
	if direction == "outward" {
		inward = targetKey
		outward = issueKey
	}
	payload := map[string]any{
		"type":         map[string]string{"name": linkTypeName},
		"inwardIssue":  map[string]string{"key": inward},
		"outwardIssue": map[string]string{"key": outward},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling link: %w", err)
	}
	return c.post("/rest/api/3/issueLink", body)
}

// DeleteLink deletes an issue link by its ID.
func (c *Client) DeleteLink(linkID string) error {
	return c.deleteReq("/rest/api/3/issueLink/" + linkID)
}

// DeleteAttachment removes an attachment by its ID.
func (c *Client) DeleteAttachment(attachmentID string) error {
	return c.deleteReq("/rest/api/3/attachment/" + attachmentID)
}

// GetProjectStatusesAndTypes fetches statuses grouped by issue type from
// GET /rest/api/3/project/{key}/statuses. Returns deduplicated statuses
// and issue types.
func (c *Client) GetProjectStatusesAndTypes(projectKey string) ([]models.Status, []models.IssueType, error) {
	path := "/rest/api/3/project/" + projectKey + "/statuses"
	data, err := c.get(path)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching project statuses: %w", err)
	}

	entries, err := decodeJSON[[]jiraProjectStatusEntry](data)
	if err != nil {
		return nil, nil, fmt.Errorf("decoding project statuses: %w", err)
	}

	seenStatus := make(map[string]bool)
	var statuses []models.Status
	issueTypes := make([]models.IssueType, 0, len(entries))

	for _, entry := range entries {
		issueTypes = append(issueTypes, models.IssueType{
			ID:   entry.ID,
			Name: entry.Name,
		})
		for _, s := range entry.Statuses {
			if !seenStatus[s.ID] {
				seenStatus[s.ID] = true
				statuses = append(statuses, models.Status{
					ID:   s.ID,
					Name: s.Name,
				})
			}
		}
	}

	return statuses, issueTypes, nil
}
