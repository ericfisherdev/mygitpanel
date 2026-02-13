package model

import "time"

// PullRequest represents a GitHub pull request tracked by ReviewHub.
type PullRequest struct {
	ID              int64
	Number          int
	RepoFullName    string
	Title           string
	Author          string
	Status          PRStatus
	IsDraft         bool
	URL             string
	Branch          string
	BaseBranch      string
	NeedsReview     bool
	HeadSHA         string // Current head commit SHA; used for outdated review detection.
	Additions       int
	Deletions       int
	ChangedFiles    int
	MergeableStatus MergeableStatus // Default MergeableUnknown.
	CIStatus        CIStatus        // Default CIStatusUnknown.
	Labels          []string
	OpenedAt        time.Time
	UpdatedAt       time.Time
	LastActivityAt  time.Time

	// Transient fields populated during GitHub fetch, not persisted.
	RequestedReviewers []string
	RequestedTeamSlugs []string
}

// DaysSinceOpened returns the number of days since the PR was opened.
func (pr PullRequest) DaysSinceOpened() int {
	return int(time.Since(pr.OpenedAt).Hours() / 24)
}

// DaysSinceLastActivity returns the number of days since the last activity on the PR.
func (pr PullRequest) DaysSinceLastActivity() int {
	return int(time.Since(pr.LastActivityAt).Hours() / 24)
}

// IsStale returns true if the PR has had no activity for the given number of days.
func (pr PullRequest) IsStale(thresholdDays int) bool {
	return pr.DaysSinceLastActivity() >= thresholdDays
}
