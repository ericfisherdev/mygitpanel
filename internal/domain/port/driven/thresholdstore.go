package driven

import (
	"context"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// ThresholdStore defines the driven port for attention threshold configuration persistence.
type ThresholdStore interface {
	// GetGlobalSettings returns the current global threshold defaults.
	// Returns model.DefaultGlobalSettings() if no settings have been saved.
	GetGlobalSettings(ctx context.Context) (model.GlobalSettings, error)

	// SetGlobalSettings persists the global threshold defaults.
	SetGlobalSettings(ctx context.Context, settings model.GlobalSettings) error

	// GetRepoThreshold returns the per-repository threshold overrides for the
	// given repository. Returns a zero-value RepoThreshold (all nil pointers)
	// when no override exists.
	GetRepoThreshold(ctx context.Context, repoFullName string) (model.RepoThreshold, error)

	// SetRepoThreshold persists per-repository threshold overrides.
	SetRepoThreshold(ctx context.Context, threshold model.RepoThreshold) error

	// DeleteRepoThreshold removes the per-repository override for the given repo,
	// causing it to fall back to global settings.
	DeleteRepoThreshold(ctx context.Context, repoFullName string) error
}
