package driven

import (
	"context"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// RepoSettingsStore defines the driven port for per-repository settings persistence.
// GetSettings returns (nil, nil) if no settings exist for the repository —
// callers should apply defaults when nil is returned.
type RepoSettingsStore interface {
	GetSettings(ctx context.Context, repoFullName string) (*model.RepoSettings, error)
	SetSettings(ctx context.Context, settings model.RepoSettings) error
}
