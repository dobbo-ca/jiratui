package jira

import (
	"fmt"

	"github.com/christopherdobbyn/jiratui/internal/models"
)

// GetPriorities returns all available issue priorities.
func (c *Client) GetPriorities() ([]models.Priority, error) {
	path := "/rest/api/3/priority"
	data, err := c.get(path)
	if err != nil {
		return nil, fmt.Errorf("fetching priorities: %w", err)
	}

	jiraPriorities, err := decodeJSON[[]jiraPriority](data)
	if err != nil {
		return nil, fmt.Errorf("decoding priorities: %w", err)
	}

	priorities := make([]models.Priority, len(jiraPriorities))
	for i, jp := range jiraPriorities {
		priorities[i] = models.Priority{
			ID:   jp.ID,
			Name: jp.Name,
		}
	}
	return priorities, nil
}
