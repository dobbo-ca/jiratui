package models

import "time"

type User struct {
	AccountID   string
	DisplayName string
	Email       string
	AvatarURL   string
}

type Priority struct {
	ID   string
	Name string // "Highest", "High", "Medium", "Low", "Lowest"
}

type Status struct {
	ID   string
	Name string // "To Do", "In Progress", "In Review", "Done"
}

type IssueType struct {
	ID   string
	Name string // "Bug", "Task", "Story", "Epic"
}

type Transition struct {
	ID   string
	Name string
	To   Status
}

type Project struct {
	ID   string
	Key  string
	Name string
}

type Comment struct {
	ID      string
	Author  User
	Body    string
	Created time.Time
	Updated time.Time
}

type IssueLink struct {
	ID           string
	Type         string // "blocks", "is blocked by", "relates to"
	InwardIssue  *IssueSummary
	OutwardIssue *IssueSummary
}

type IssueSummary struct {
	Key     string
	Summary string
	Status  Status
}

type Attachment struct {
	ID       string
	Filename string
	MimeType string
	Size     int
	URL      string
}

type Issue struct {
	Key         string
	Summary     string
	Description string
	Status      Status
	Priority    Priority
	Type        IssueType
	Assignee    *User
	Reporter    *User
	Labels      []string
	Created     time.Time
	Updated     time.Time
	DueDate     *time.Time
	Parent      *IssueSummary
	Sprint      string
	Subtasks    []IssueSummary
	Links       []IssueLink
	Attachments []Attachment
	Comments    []Comment
	ProjectKey  string
	ProjectName string
	BrowseURL   string
}
