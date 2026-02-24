package model

import "time"

// JiraConnection represents a configured Jira Cloud instance with credentials.
// Token is plaintext at the domain boundary; the adapter layer encrypts it for storage.
type JiraConnection struct {
	ID          int64
	DisplayName string
	BaseURL     string
	Email       string
	Token       string
	IsDefault   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// JiraIssue represents a Jira issue with its key metadata for display.
type JiraIssue struct {
	Key         string
	Summary     string
	Description string // Plain text extracted from ADF.
	Status      string
	Priority    string
	Assignee    string
	Comments    []JiraComment
}

// JiraComment represents a single comment on a Jira issue.
type JiraComment struct {
	Author    string
	Body      string // Plain text.
	CreatedAt time.Time
}
