package driven

import (
	"context"

	"github.com/efisher/reviewhub/internal/domain/model"
)

// PRStore defines the driven port for pull request persistence.
type PRStore interface {
	Upsert(ctx context.Context, pr model.PullRequest) error
	GetByRepository(ctx context.Context, repoFullName string) ([]model.PullRequest, error)
	GetByStatus(ctx context.Context, status model.PRStatus) ([]model.PullRequest, error)
	GetByNumber(ctx context.Context, repoFullName string, number int) (*model.PullRequest, error)
	ListAll(ctx context.Context) ([]model.PullRequest, error)
	ListNeedingReview(ctx context.Context) ([]model.PullRequest, error)
	Delete(ctx context.Context, repoFullName string, number int) error
}
