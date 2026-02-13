package model

import "time"

// IssueComment represents a PR-level general comment (from the GitHub Issues API,
// not the Pull Requests review comments API).
type IssueComment struct {
	ID        int64
	PRID      int64 // Links to PullRequest; stored by PR, not by issue.
	Author    string
	Body      string
	IsBot     bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
