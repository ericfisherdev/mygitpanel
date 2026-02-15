package driven

import (
	"context"
	"errors"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// Sentinel errors returned by RepoStore implementations.
var (
	// ErrRepoNotFound indicates the requested repository does not exist.
	ErrRepoNotFound = errors.New("repository not found")

	// ErrRepoAlreadyExists indicates a repository with the same name already exists.
	ErrRepoAlreadyExists = errors.New("repository already exists")
)

// RepoStore defines the driven port for repository persistence.
// Add returns ErrRepoAlreadyExists if the repository already exists.
// Remove returns ErrRepoNotFound if the repository does not exist.
type RepoStore interface {
	Add(ctx context.Context, repo model.Repository) error
	Remove(ctx context.Context, fullName string) error
	GetByFullName(ctx context.Context, fullName string) (*model.Repository, error)
	ListAll(ctx context.Context) ([]model.Repository, error)
}
