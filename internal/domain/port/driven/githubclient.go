package driven

import (
	"context"

	"github.com/efisher/reviewhub/internal/domain/model"
)

// GitHubClient defines the driven port for fetching data from the GitHub API.
type GitHubClient interface {
	FetchPullRequests(ctx context.Context, repoFullName string) ([]model.PullRequest, error)
	FetchReviews(ctx context.Context, repoFullName string, prNumber int) ([]model.Review, error)
	FetchReviewComments(ctx context.Context, repoFullName string, prNumber int) ([]model.ReviewComment, error)
}
