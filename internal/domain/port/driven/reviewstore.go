package driven

import (
	"context"

	"github.com/efisher/reviewhub/internal/domain/model"
)

// ReviewStore defines the driven port for persisting reviews, review comments,
// and issue comments.
type ReviewStore interface {
	UpsertReview(ctx context.Context, review model.Review) error
	UpsertReviewComment(ctx context.Context, comment model.ReviewComment) error
	UpsertIssueComment(ctx context.Context, comment model.IssueComment) error
	GetReviewsByPR(ctx context.Context, prID int64) ([]model.Review, error)
	GetReviewCommentsByPR(ctx context.Context, prID int64) ([]model.ReviewComment, error)
	GetIssueCommentsByPR(ctx context.Context, prID int64) ([]model.IssueComment, error)
	UpdateCommentResolution(ctx context.Context, commentID int64, isResolved bool) error
	// DeleteReviewsByPR removes all reviews, review comments, and issue comments
	// associated with the given PR. Used for cleanup when a PR is removed.
	DeleteReviewsByPR(ctx context.Context, prID int64) error
}
