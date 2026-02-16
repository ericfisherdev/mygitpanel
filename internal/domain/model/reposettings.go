package model

// RepoSettings holds per-repository attention thresholds used by review
// workflow and attention signal features.
type RepoSettings struct {
	RepoFullName        string
	RequiredReviewCount int
	UrgencyDays         int
}
