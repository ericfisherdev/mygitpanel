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

	// FetchCheckRuns returns all check runs for the given ref (commit SHA or branch).
	FetchCheckRuns(ctx context.Context, repoFullName string, ref string) ([]model.CheckRun, error)
	// FetchCombinedStatus returns the combined commit status for the given ref.
	FetchCombinedStatus(ctx context.Context, repoFullName string, ref string) (*model.CombinedStatus, error)
	// FetchPRDetail returns diff stats and mergeable status for a single PR.
	FetchPRDetail(ctx context.Context, repoFullName string, prNumber int) (*model.PRDetail, error)
	// FetchRequiredStatusChecks returns the list of required status check contexts
	// for the given branch's protection rules. Returns empty slice if unprotected.
	FetchRequiredStatusChecks(ctx context.Context, repoFullName string, branch string) ([]string, error)
}
