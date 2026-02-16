package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
)

// Compile-time interface satisfaction check.
var _ driven.RepoSettingsStore = (*RepoSettingsRepo)(nil)

// RepoSettingsRepo is the SQLite implementation of the RepoSettingsStore port interface.
type RepoSettingsRepo struct {
	db *DB
}

// NewRepoSettingsRepo creates a new RepoSettingsRepo backed by the given DB.
func NewRepoSettingsRepo(db *DB) *RepoSettingsRepo {
	return &RepoSettingsRepo{db: db}
}

// GetSettings retrieves per-repository settings. Returns (nil, nil) if no
// settings exist for the repository — callers should apply defaults.
func (r *RepoSettingsRepo) GetSettings(ctx context.Context, repoFullName string) (*model.RepoSettings, error) {
	const query = `
		SELECT repo_full_name, required_review_count, urgency_days
		FROM repo_settings
		WHERE repo_full_name = ?
	`

	var s model.RepoSettings

	err := r.db.Reader.QueryRowContext(ctx, query, repoFullName).Scan(
		&s.RepoFullName, &s.RequiredReviewCount, &s.UrgencyDays,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get settings for %s: %w", repoFullName, err)
	}

	return &s, nil
}

// SetSettings inserts or updates per-repository settings. On conflict the
// review count and urgency days are replaced.
func (r *RepoSettingsRepo) SetSettings(ctx context.Context, settings model.RepoSettings) error {
	const query = `
		INSERT INTO repo_settings (repo_full_name, required_review_count, urgency_days)
		VALUES (?, ?, ?)
		ON CONFLICT(repo_full_name) DO UPDATE SET
			required_review_count = excluded.required_review_count,
			urgency_days = excluded.urgency_days
	`

	_, err := r.db.Writer.ExecContext(ctx, query,
		settings.RepoFullName, settings.RequiredReviewCount, settings.UrgencyDays,
	)
	if err != nil {
		return fmt.Errorf("set settings for %s: %w", settings.RepoFullName, err)
	}

	return nil
}
