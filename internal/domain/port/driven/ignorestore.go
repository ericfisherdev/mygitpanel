package driven

import (
	"context"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// IgnoreStore defines the driven port for managing the PR ignore list.
// Ignore is idempotent — ignoring an already-ignored PR is a no-op.
type IgnoreStore interface {
	Ignore(ctx context.Context, repoFullName string, prNumber int) error
	Unignore(ctx context.Context, repoFullName string, prNumber int) error
	IsIgnored(ctx context.Context, repoFullName string, prNumber int) (bool, error)
	ListIgnored(ctx context.Context) ([]model.IgnoredPR, error)
}
