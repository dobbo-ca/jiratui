package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/christopherdobbyn/jiratui/internal/models"
)

func TestBuildContextMarkdown(t *testing.T) {
	issue := models.Issue{
		Key:         "TEC-123",
		Summary:     "Fix login bug",
		Description: "Users cannot log in after password reset.",
		Status:      models.Status{Name: "In Progress"},
		Priority:    models.Priority{Name: "High"},
		Type:        models.IssueType{Name: "Bug"},
		Assignee:    &models.User{DisplayName: "Chris"},
		ProjectKey:  "TEC",
		Comments: []models.Comment{
			{
				Author:  models.User{DisplayName: "Jane"},
				Body:    "I can reproduce this on staging.",
				Created: time.Date(2026, 4, 5, 10, 30, 0, 0, time.UTC),
			},
		},
		Attachments: []models.Attachment{
			{Filename: "screenshot.png", Size: 245000},
			{Filename: "logs.txt", Size: 1200},
		},
	}

	md := buildContextMarkdown(issue)

	checks := []string{
		"# Jira Ticket: TEC-123",
		"**Project:** TEC",
		"**Status:** In Progress",
		"**Priority:** High",
		"Fix login bug",
		"Users cannot log in after password reset.",
		"**Jane**",
		"I can reproduce this on staging.",
		"screenshot.png",
		"logs.txt",
		"jt download-attachment TEC TEC-123",
		"jt attach TEC TEC-123",
		"jt update-description TEC TEC-123",
	}
	for _, check := range checks {
		if !strings.Contains(md, check) {
			t.Errorf("context missing %q", check)
		}
	}
}
