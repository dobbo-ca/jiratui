package jira

// jiraSearchResponse is the response from /rest/api/3/search/jql
type jiraSearchResponse struct {
	Issues        []jiraIssue `json:"issues"`
	NextPageToken string      `json:"nextPageToken,omitempty"`
	IsLast        bool        `json:"isLast"`
}

type jiraIssue struct {
	Key    string     `json:"key"`
	Self   string     `json:"self"`
	Fields jiraFields `json:"fields"`
}

type jiraFields struct {
	Summary     string           `json:"summary"`
	Description *jiraADF         `json:"description"`
	Status      jiraStatus       `json:"status"`
	Priority    *jiraPriority    `json:"priority"`
	IssueType   jiraIssueType    `json:"issuetype"`
	Assignee    *jiraUser        `json:"assignee"`
	Reporter    *jiraUser        `json:"reporter"`
	Labels      []string         `json:"labels"`
	Created     string           `json:"created"`
	Updated     string           `json:"updated"`
	DueDate     *string          `json:"duedate"`
	Parent      *jiraParent      `json:"parent"`
	Sprint      *jiraSprint      `json:"sprint"`
	Subtasks    []jiraIssue      `json:"subtasks"`
	IssueLinks  []jiraIssueLink  `json:"issuelinks"`
	Comment     *jiraCommentPage `json:"comment"`
	Attachment  []jiraAttachment `json:"attachment"`
	Project     jiraProject      `json:"project"`
}

type jiraAttachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	MimeType string `json:"mimeType"`
	Size     int    `json:"size"`
	Content  string `json:"content"` // download URL
}

// jiraADF represents Atlassian Document Format, converted to markdown for display.
type jiraADF struct {
	Type    string     `json:"type"`
	Version int        `json:"version,omitempty"`
	Content []jiraNode `json:"content"`
}

type jiraNode struct {
	Type    string            `json:"type"`
	Text    string            `json:"text,omitempty"`
	Content []jiraNode        `json:"content,omitempty"`
	Attrs   map[string]any    `json:"attrs,omitempty"`
	Marks   []jiraMark        `json:"marks,omitempty"`
}

type jiraMark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

type jiraStatus struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type jiraPriority struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type jiraIssueType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type jiraUser struct {
	AccountID   string            `json:"accountId"`
	DisplayName string            `json:"displayName"`
	Email       string            `json:"emailAddress"`
	AvatarURLs  map[string]string `json:"avatarUrls"`
}

type jiraParent struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string     `json:"summary"`
		Status  jiraStatus `json:"status"`
	} `json:"fields"`
}

type jiraSprint struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type jiraIssueLink struct {
	ID   string `json:"id"`
	Type struct {
		Name    string `json:"name"`
		Inward  string `json:"inward"`
		Outward string `json:"outward"`
	} `json:"type"`
	InwardIssue  *jiraLinkedIssue `json:"inwardIssue"`
	OutwardIssue *jiraLinkedIssue `json:"outwardIssue"`
}

type jiraLinkedIssue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string     `json:"summary"`
		Status  jiraStatus `json:"status"`
	} `json:"fields"`
}

type jiraCommentPage struct {
	Total    int           `json:"total"`
	Comments []jiraComment `json:"comments"`
}

type jiraComment struct {
	ID      string   `json:"id"`
	Author  jiraUser `json:"author"`
	Body    *jiraADF `json:"body"`
	Created string   `json:"created"`
	Updated string   `json:"updated"`
}

type jiraProject struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type jiraLinkTypesResponse struct {
	IssueLinkTypes []jiraIssueLinkType `json:"issueLinkTypes"`
}

type jiraIssueLinkType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
}

// jiraProjectStatusEntry is one element of the array returned by
// GET /rest/api/3/project/{key}/statuses — one per issue type.
type jiraProjectStatusEntry struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"` // issue type name
	Statuses []jiraStatus `json:"statuses"`
}
