package driven

import (
	"context"

	"github.com/efisher/reviewhub/internal/domain/model"
)

// RepoStore defines the driven port for repository persistence.
type RepoStore interface {
	Add(ctx context.Context, repo model.Repository) error
	Remove(ctx context.Context, fullName string) error
	GetByFullName(ctx context.Context, fullName string) (*model.Repository, error)
	ListAll(ctx context.Context) ([]model.Repository, error)
}
