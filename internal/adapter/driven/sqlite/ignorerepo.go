package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
)

// Compile-time interface satisfaction check.
var _ driven.IgnoreStore = (*IgnoreRepo)(nil)

// IgnoreRepo is the SQLite implementation of the IgnoreStore port interface.
type IgnoreRepo struct {
	db *DB
}

// NewIgnoreRepo creates a new IgnoreRepo backed by the given DB.
func NewIgnoreRepo(db *DB) *IgnoreRepo {
	return &IgnoreRepo{db: db}
}

// Ignore adds a PR to the ignore list. The operation is idempotent — ignoring
// an already-ignored PR is a no-op.
func (r *IgnoreRepo) Ignore(ctx context.Context, repoFullName string, prNumber int) error {
	const query = `
		INSERT INTO ignored_prs (repo_full_name, pr_number, ignored_at)
		VALUES (?, ?, ?)
		ON CONFLICT(repo_full_name, pr_number) DO NOTHING
	`

	_, err := r.db.Writer.ExecContext(ctx, query, repoFullName, prNumber, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("ignore PR %s#%d: %w", repoFullName, prNumber, err)
	}

	return nil
}

// Unignore removes a PR from the ignore list. No error is returned if the
// PR is not currently ignored.
func (r *IgnoreRepo) Unignore(ctx context.Context, repoFullName string, prNumber int) error {
	const query = `DELETE FROM ignored_prs WHERE repo_full_name = ? AND pr_number = ?`

	_, err := r.db.Writer.ExecContext(ctx, query, repoFullName, prNumber)
	if err != nil {
		return fmt.Errorf("unignore PR %s#%d: %w", repoFullName, prNumber, err)
	}

	return nil
}

// IsIgnored returns true if the PR is on the ignore list.
func (r *IgnoreRepo) IsIgnored(ctx context.Context, repoFullName string, prNumber int) (bool, error) {
	const query = `SELECT COUNT(*) FROM ignored_prs WHERE repo_full_name = ? AND pr_number = ?`

	var count int

	err := r.db.Reader.QueryRowContext(ctx, query, repoFullName, prNumber).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check ignored PR %s#%d: %w", repoFullName, prNumber, err)
	}

	return count > 0, nil
}

// ListIgnored returns all ignored PRs ordered by most recently ignored first.
func (r *IgnoreRepo) ListIgnored(ctx context.Context) ([]model.IgnoredPR, error) {
	const query = `
		SELECT id, repo_full_name, pr_number, ignored_at
		FROM ignored_prs
		ORDER BY ignored_at DESC
	`

	rows, err := r.db.Reader.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list ignored PRs: %w", err)
	}
	defer rows.Close()

	var ignored []model.IgnoredPR
	for rows.Next() {
		var item model.IgnoredPR
		var ignoredAt string

		if err := rows.Scan(&item.ID, &item.RepoFullName, &item.PRNumber, &ignoredAt); err != nil {
			return nil, fmt.Errorf("scan ignored PR: %w", err)
		}

		item.IgnoredAt, err = parseTime(ignoredAt)
		if err != nil {
			return nil, fmt.Errorf("parse ignored_at: %w", err)
		}

		ignored = append(ignored, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ignored PRs: %w", err)
	}

	return ignored, nil
}
