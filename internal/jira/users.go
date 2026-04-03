package jira

import (
	"fmt"
	"net/url"

	"github.com/christopherdobbyn/jiratui/internal/models"
)

// GetAssignableUsers returns users assignable to issues in the given project.
func (c *Client) GetAssignableUsers(projectKey string) ([]models.User, error) {
	params := url.Values{}
	params.Set("project", projectKey)
	params.Set("maxResults", "100")

	path := "/rest/api/3/user/assignable/search?" + params.Encode()
	data, err := c.get(path)
	if err != nil {
		return nil, fmt.Errorf("fetching assignable users: %w", err)
	}

	jiraUsers, err := decodeJSON[[]jiraUser](data)
	if err != nil {
		return nil, fmt.Errorf("decoding assignable users: %w", err)
	}

	users := make([]models.User, 0, len(jiraUsers))
	for _, ju := range jiraUsers {
		u := mapUser(&ju)
		if u != nil {
			users = append(users, *u)
		}
	}
	return users, nil
}

// SearchUsers searches for users by display name or email, scoped to a project.
func (c *Client) SearchUsers(query, projectKey string) ([]models.User, error) {
	params := url.Values{}
	params.Set("query", query)
	if projectKey != "" {
		params.Set("project", projectKey)
	}
	params.Set("maxResults", "20")

	path := "/rest/api/3/user/assignable/search?" + params.Encode()
	data, err := c.get(path)
	if err != nil {
		return nil, fmt.Errorf("searching users: %w", err)
	}

	jiraUsers, err := decodeJSON[[]jiraUser](data)
	if err != nil {
		return nil, fmt.Errorf("decoding user search: %w", err)
	}

	users := make([]models.User, 0, len(jiraUsers))
	for _, ju := range jiraUsers {
		u := mapUser(&ju)
		if u != nil {
			users = append(users, *u)
		}
	}
	return users, nil
}
