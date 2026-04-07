package jira

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/christopherdobbyn/jiratui/internal/models"
)

// SearchResult holds a page of issues and pagination state.
type SearchResult struct {
	Issues        []models.Issue
	NextPageToken string
	IsLast        bool
}

// SearchMyIssues fetches issues assigned to the current user.
// projectKey optionally filters to a specific project (empty = all projects).
// orderBy is a JQL ORDER BY clause like "key ASC" or "summary DESC".
// If empty, defaults to "updated DESC".
func (c *Client) SearchMyIssues(maxResults int, pageToken, orderBy, projectKey string) (*SearchResult, error) {
	if orderBy == "" {
		orderBy = "updated DESC"
	}
	jql := "assignee = currentUser() AND statusCategory != Done"
	if projectKey != "" {
		jql += " AND project = " + projectKey
	}
	jql += " ORDER BY " + orderBy
	return c.searchIssues(jql, maxResults, pageToken)
}

// SearchIssues fetches issues matching the given JQL query.
func (c *Client) SearchIssues(jql string, maxResults int, pageToken string) (*SearchResult, error) {
	return c.searchIssues(jql, maxResults, pageToken)
}

func (c *Client) searchIssues(jql string, maxResults int, pageToken string) (*SearchResult, error) {
	params := url.Values{}
	params.Set("jql", jql)
	params.Set("maxResults", strconv.Itoa(maxResults))
	params.Set("fields", "summary,status,priority,issuetype,assignee,reporter,labels,created,updated,duedate,project,subtasks,issuelinks,parent,sprint")
	if pageToken != "" {
		params.Set("nextPageToken", pageToken)
	}

	path := "/rest/api/3/search/jql?" + params.Encode()
	data, err := c.get(path)
	if err != nil {
		return nil, fmt.Errorf("searching issues: %w", err)
	}

	resp, err := decodeJSON[jiraSearchResponse](data)
	if err != nil {
		return nil, fmt.Errorf("decoding search response: %w", err)
	}

	issues := make([]models.Issue, len(resp.Issues))
	for i, ji := range resp.Issues {
		issues[i] = mapIssue(ji, c.baseURL)
	}

	return &SearchResult{
		Issues:        issues,
		NextPageToken: resp.NextPageToken,
		IsLast:        resp.IsLast,
	}, nil
}

// GetIssue fetches a single issue with full detail including comments.
func (c *Client) GetIssue(key string) (*models.Issue, error) {
	params := url.Values{}
	params.Set("fields", "summary,description,status,priority,issuetype,assignee,reporter,labels,created,updated,duedate,project,subtasks,issuelinks,parent,sprint,comment,attachment")

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

	// Build attachment lookup for resolving media nodes in ADF
	attachmentMap := make(map[string]string, len(f.Attachment))
	for _, a := range f.Attachment {
		attachmentMap[a.ID] = a.Filename
	}

	issue := models.Issue{
		Key:         ji.Key,
		Summary:     f.Summary,
		Description: adfToMarkdownWithMedia(f.Description, attachmentMap),
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

	// Map attachments
	issue.Attachments = make([]models.Attachment, len(f.Attachment))
	for i, ja := range f.Attachment {
		issue.Attachments[i] = models.Attachment{
			ID:       ja.ID,
			Filename: ja.Filename,
			MimeType: ja.MimeType,
			Size:     ja.Size,
			URL:      ja.Content,
		}
	}

	// Map comments
	if f.Comment != nil {
		issue.Comments = make([]models.Comment, len(f.Comment.Comments))
		for i, jc := range f.Comment.Comments {
			author := mapUser(&jc.Author)
			issue.Comments[i] = models.Comment{
				ID:      jc.ID,
				Author:  *author,
				Body:    adfToMarkdownWithMedia(jc.Body, attachmentMap),
				Created: parseTime(jc.Created),
				Updated: parseTime(jc.Updated),
			}
		}
	}

	return issue
}

// UpdateLabels sets the labels on an issue.
func (c *Client) UpdateLabels(issueKey string, labels []string) error {
	return c.updateField(issueKey, "labels", labels)
}

// UpdateSummary sets the summary (title) on an issue.
func (c *Client) UpdateSummary(issueKey, summary string) error {
	return c.updateField(issueKey, "summary", summary)
}

// UpdateDescription sets the description on an issue using markdown (converted to ADF).
func (c *Client) UpdateDescription(issueKey, markdownBody string) error {
	adf := markdownToADF(markdownBody)
	return c.updateField(issueKey, "description", adf)
}

// UpdatePriority sets the priority on an issue.
func (c *Client) UpdatePriority(issueKey, priorityID string) error {
	return c.updateField(issueKey, "priority", map[string]string{"id": priorityID})
}

// UpdateDueDate sets or clears the due date on an issue. Pass "" to clear.
func (c *Client) UpdateDueDate(issueKey, dueDate string) error {
	if dueDate == "" {
		return c.updateField(issueKey, "duedate", nil)
	}
	return c.updateField(issueKey, "duedate", dueDate)
}

// UpdateParent sets or clears the parent on an issue. Pass "" to clear.
func (c *Client) UpdateParent(issueKey, parentKey string) error {
	if parentKey == "" {
		return c.updateField(issueKey, "parent", nil)
	}
	return c.updateField(issueKey, "parent", map[string]string{"key": parentKey})
}

// UpdateAssignee sets the assignee on an issue. Pass "" to unassign.
func (c *Client) UpdateAssignee(issueKey, accountID string) error {
	if accountID == "" {
		return c.updateField(issueKey, "assignee", nil)
	}
	return c.updateField(issueKey, "assignee", map[string]string{"accountId": accountID})
}

// updateField is a generic helper to update a single field on an issue.
func (c *Client) updateField(issueKey, fieldName string, value interface{}) error {
	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			fieldName: value,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling %s: %w", fieldName, err)
	}
	return c.put("/rest/api/3/issue/"+issueKey, body)
}
