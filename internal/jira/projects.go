package jira

import (
	"fmt"

	"github.com/christopherdobbyn/jiratui/internal/models"
)

type jiraProjectFull struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// GetProjects returns all projects the user has access to.
func (c *Client) GetProjects() ([]models.Project, error) {
	path := "/rest/api/3/project?maxResults=100&orderBy=name"
	data, err := c.get(path)
	if err != nil {
		return nil, fmt.Errorf("fetching projects: %w", err)
	}

	jiraProjects, err := decodeJSON[[]jiraProjectFull](data)
	if err != nil {
		return nil, fmt.Errorf("decoding projects: %w", err)
	}

	projects := make([]models.Project, len(jiraProjects))
	for i, jp := range jiraProjects {
		projects[i] = models.Project{
			ID:   jp.ID,
			Key:  jp.Key,
			Name: jp.Name,
		}
	}
	return projects, nil
}
