package driven

import (
	"context"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// GitHubClient defines the driven port for interacting with the GitHub API.
// Read methods fetch data; write methods mutate PR state (reviews, comments, draft status).
type GitHubClient interface {
	// Read methods

	FetchPullRequests(ctx context.Context, repoFullName string, state string) ([]model.PullRequest, error)
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

	// Write methods

	// CreateReview submits a review on a pull request.
	// event must be one of "APPROVE", "REQUEST_CHANGES", or "COMMENT".
	CreateReview(ctx context.Context, repoFullName string, prNumber int, event string, body string) error
	// CreateIssueComment adds a PR-level comment (via the Issues API).
	CreateIssueComment(ctx context.Context, repoFullName string, prNumber int, body string) error
	// ReplyToReviewComment replies to an existing review comment thread.
	// commentID must be the root comment ID of the thread.
	ReplyToReviewComment(ctx context.Context, repoFullName string, prNumber int, commentID int64, body string) error
	// SetDraftStatus toggles a PR's draft state via GitHub GraphQL mutations.
	// nodeID is the PR's GraphQL node ID (not the numeric PR number).
	// draft=true converts to draft; draft=false marks ready for review.
	SetDraftStatus(ctx context.Context, repoFullName string, prNumber int, nodeID string, draft bool) error
}
