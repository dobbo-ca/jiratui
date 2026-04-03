package jira

import (
	"fmt"

	"github.com/christopherdobbyn/jiratui/internal/models"
)

// jiraTransitionsResponse is the response from /rest/api/3/issue/{key}/transitions
type jiraTransitionsResponse struct {
	Transitions []jiraTransition `json:"transitions"`
}

type jiraTransition struct {
	ID   string     `json:"id"`
	Name string     `json:"name"`
	To   jiraStatus `json:"to"`
}

// GetTransitions returns the available status transitions for an issue.
func (c *Client) GetTransitions(issueKey string) ([]models.Transition, error) {
	path := "/rest/api/3/issue/" + issueKey + "/transitions"
	data, err := c.get(path)
	if err != nil {
		return nil, fmt.Errorf("fetching transitions for %s: %w", issueKey, err)
	}

	resp, err := decodeJSON[jiraTransitionsResponse](data)
	if err != nil {
		return nil, fmt.Errorf("decoding transitions for %s: %w", issueKey, err)
	}

	transitions := make([]models.Transition, len(resp.Transitions))
	for i, jt := range resp.Transitions {
		transitions[i] = models.Transition{
			ID:   jt.ID,
			Name: jt.Name,
			To: models.Status{
				ID:   jt.To.ID,
				Name: jt.To.Name,
			},
		}
	}
	return transitions, nil
}
