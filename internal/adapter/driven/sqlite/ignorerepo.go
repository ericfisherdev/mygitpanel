package sqlite

import (
	"context"
	"fmt"

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

// Ignore marks a PR as ignored. Idempotent — silently succeeds if already ignored.
func (r *IgnoreRepo) Ignore(ctx context.Context, prID int64) error {
	const query = `INSERT OR IGNORE INTO ignored_prs (pr_id) VALUES (?)`
	_, err := r.db.Writer.ExecContext(ctx, query, prID)
	if err != nil {
		return fmt.Errorf("ignore PR %d: %w", prID, err)
	}
	return nil
}

// Unignore removes a PR from the ignore list. No-op if the PR is not ignored.
func (r *IgnoreRepo) Unignore(ctx context.Context, prID int64) error {
	const query = `DELETE FROM ignored_prs WHERE pr_id = ?`
	_, err := r.db.Writer.ExecContext(ctx, query, prID)
	if err != nil {
		return fmt.Errorf("unignore PR %d: %w", prID, err)
	}
	return nil
}

// IsIgnored returns whether the given PR is currently ignored.
func (r *IgnoreRepo) IsIgnored(ctx context.Context, prID int64) (bool, error) {
	const query = `SELECT COUNT(*) FROM ignored_prs WHERE pr_id = ?`
	var count int
	if err := r.db.Reader.QueryRowContext(ctx, query, prID).Scan(&count); err != nil {
		return false, fmt.Errorf("check ignored PR %d: %w", prID, err)
	}
	return count > 0, nil
}

// ListIgnored returns all ignored PRs ordered by ignored_at DESC.
func (r *IgnoreRepo) ListIgnored(ctx context.Context) ([]driven.IgnoredPR, error) {
	const query = `SELECT pr_id, ignored_at FROM ignored_prs ORDER BY ignored_at DESC`
	rows, err := r.db.Reader.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list ignored PRs: %w", err)
	}
	defer rows.Close()

	var result []driven.IgnoredPR
	for rows.Next() {
		var item driven.IgnoredPR
		var ignoredAt string
		if err := rows.Scan(&item.PRID, &ignoredAt); err != nil {
			return nil, fmt.Errorf("scan ignored PR: %w", err)
		}
		item.IgnoredAt, err = parseTime(ignoredAt)
		if err != nil {
			return nil, fmt.Errorf("parse ignored_at for PR %d: %w", item.PRID, err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ignored PRs: %w", err)
	}
	return result, nil
}

// ListIgnoredIDs returns a set of ignored PR IDs for O(1) lookup in the application layer.
func (r *IgnoreRepo) ListIgnoredIDs(ctx context.Context) (map[int64]struct{}, error) {
	const query = `SELECT pr_id FROM ignored_prs ORDER BY ignored_at DESC`
	rows, err := r.db.Reader.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list ignored PR IDs: %w", err)
	}
	defer rows.Close()

	result := make(map[int64]struct{})
	for rows.Next() {
		var prID int64
		if err := rows.Scan(&prID); err != nil {
			return nil, fmt.Errorf("scan ignored PR ID: %w", err)
		}
		result[prID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ignored PR IDs: %w", err)
	}
	return result, nil
}
