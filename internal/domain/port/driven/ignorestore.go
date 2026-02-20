package driven

import (
	"context"
	"time"
)

// IgnoredPR is a port-layer type representing a PR that has been ignored by the user.
// It is defined here (not in model/) because it is a persistence concern, not a
// pure domain entity.
type IgnoredPR struct {
	PRID      int64
	IgnoredAt time.Time
}

// IgnoreStore defines the driven port for the PR ignore list.
type IgnoreStore interface {
	// Ignore marks a PR as ignored. Idempotent — silently succeeds if already ignored.
	Ignore(ctx context.Context, prID int64) error

	// Unignore removes a PR from the ignore list. No-op if the PR is not ignored.
	Unignore(ctx context.Context, prID int64) error

	// IsIgnored returns whether the given PR is currently ignored.
	IsIgnored(ctx context.Context, prID int64) (bool, error)

	// ListIgnored returns all ignored PRs ordered by ignored_at DESC.
	ListIgnored(ctx context.Context) ([]IgnoredPR, error)

	// ListIgnoredIDs returns a set of ignored PR IDs for O(1) lookup in the
	// application layer.
	ListIgnoredIDs(ctx context.Context) (map[int64]struct{}, error)
}
