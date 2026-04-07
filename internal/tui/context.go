package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/christopherdobbyn/jiratui/internal/models"
)

func buildContextMarkdown(issue models.Issue) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Jira Ticket: %s\n", issue.Key))
	assignee := "Unassigned"
	if issue.Assignee != nil {
		assignee = issue.Assignee.DisplayName
	}
	b.WriteString(fmt.Sprintf("**Project:** %s | **Status:** %s | **Priority:** %s\n",
		issue.ProjectKey, issue.Status.Name, issue.Priority.Name))
	b.WriteString(fmt.Sprintf("**Type:** %s | **Assignee:** %s\n\n",
		issue.Type.Name, assignee))

	b.WriteString("## Summary\n")
	b.WriteString(issue.Summary + "\n\n")

	if issue.Description != "" {
		b.WriteString("## Description\n")
		b.WriteString(issue.Description + "\n\n")
	}

	if len(issue.Comments) > 0 {
		b.WriteString("## Comments\n")
		for _, c := range issue.Comments {
			ts := c.Created.Format("2006-01-02 15:04")
			b.WriteString(fmt.Sprintf("**%s** (%s):\n", c.Author.DisplayName, ts))
			b.WriteString(fmt.Sprintf("> %s\n\n", c.Body))
		}
	}

	if len(issue.Attachments) > 0 {
		b.WriteString("## Attachments\n")
		for _, a := range issue.Attachments {
			b.WriteString(fmt.Sprintf("- %s (%s)\n", a.Filename, formatBytes(a.Size)))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Available Commands\n")
	b.WriteString("You have access to the `jt` CLI for interacting with this ticket:\n\n")
	b.WriteString(fmt.Sprintf("- Download an attachment:\n  `jt download-attachment %s %s \"filename\" /tmp/`\n\n",
		issue.ProjectKey, issue.Key))
	b.WriteString(fmt.Sprintf("- Attach a file to this ticket:\n  `jt attach %s %s path/to/file`\n\n",
		issue.ProjectKey, issue.Key))
	b.WriteString(fmt.Sprintf("- Update the ticket description:\n  `jt update-description %s %s path/to/file.md`\n",
		issue.ProjectKey, issue.Key))

	return b.String()
}

func writeContextFile(issue models.Issue) (string, error) {
	dir := filepath.Join(os.TempDir(), "jt-claude", issue.Key)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating context dir: %w", err)
	}
	path := filepath.Join(dir, "context.md")
	md := buildContextMarkdown(issue)
	if err := os.WriteFile(path, []byte(md), 0o600); err != nil {
		return "", fmt.Errorf("writing context file: %w", err)
	}
	return path, nil
}

func formatBytes(size int) string {
	switch {
	case size >= 1_000_000:
		return fmt.Sprintf("%.1f MB", float64(size)/1_000_000)
	case size >= 1_000:
		return fmt.Sprintf("%.0f KB", float64(size)/1_000)
	default:
		return fmt.Sprintf("%d B", size)
	}
}
