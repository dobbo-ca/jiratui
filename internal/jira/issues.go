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
