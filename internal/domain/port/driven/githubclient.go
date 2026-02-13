package driven

import (
	"context"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// GitHubClient defines the driven port for fetching data from the GitHub API.
type GitHubClient interface {
	FetchPullRequests(ctx context.Context, repoFullName string) ([]model.PullRequest, error)
	FetchReviews(ctx context.Context, repoFullName string, prNumber int) ([]model.Review, error)
	FetchReviewComments(ctx context.Context, repoFullName string, prNumber int) ([]model.ReviewComment, error)
	FetchIssueComments(ctx context.Context, repoFullName string, prNumber int) ([]model.IssueComment, error)
	// FetchThreadResolution returns a map of review comment ID to its resolved status.
	// This data typically comes from the GitHub GraphQL API.
	FetchThreadResolution(ctx context.Context, repoFullName string, prNumber int) (map[int64]bool, error)
}
