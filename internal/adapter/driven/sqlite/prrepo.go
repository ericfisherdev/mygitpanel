package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
)

// Compile-time interface satisfaction check.
var _ driven.PRStore = (*PRRepo)(nil)

// PRRepo is the SQLite implementation of the PRStore port interface.
type PRRepo struct {
	db *DB
}

// NewPRRepo creates a new PRRepo backed by the given DB.
func NewPRRepo(db *DB) *PRRepo {
	return &PRRepo{db: db}
}

// Upsert inserts or replaces a pull request. Labels are serialized as a JSON array
// in the TEXT column.
func (r *PRRepo) Upsert(ctx context.Context, pr model.PullRequest) error {
	const query = `
		INSERT INTO pull_requests (
			number, repo_full_name, title, author, status, is_draft, needs_review,
			url, branch, base_branch, labels, head_sha, opened_at, updated_at, last_activity_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(repo_full_name, number) DO UPDATE SET
			title = excluded.title,
			author = excluded.author,
			status = excluded.status,
			is_draft = excluded.is_draft,
			needs_review = excluded.needs_review,
			url = excluded.url,
			branch = excluded.branch,
			base_branch = excluded.base_branch,
			labels = excluded.labels,
			head_sha = excluded.head_sha,
			opened_at = excluded.opened_at,
			updated_at = excluded.updated_at,
			last_activity_at = excluded.last_activity_at
	`

	labels := pr.Labels
	if labels == nil {
		labels = []string{}
	}
	labelsJSON, err := json.Marshal(labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}

	isDraft := 0
	if pr.IsDraft {
		isDraft = 1
	}

	needsReview := 0
	if pr.NeedsReview {
		needsReview = 1
	}

	_, err = r.db.Writer.ExecContext(ctx, query,
		pr.Number, pr.RepoFullName, pr.Title, pr.Author, string(pr.Status), isDraft, needsReview,
		pr.URL, pr.Branch, pr.BaseBranch, string(labelsJSON), pr.HeadSHA,
		pr.OpenedAt.UTC(), pr.UpdatedAt.UTC(), pr.LastActivityAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("upsert pull request %s#%d: %w", pr.RepoFullName, pr.Number, err)
	}

	return nil
}

// GetByRepository returns all pull requests for the given repository, ordered by number.
func (r *PRRepo) GetByRepository(ctx context.Context, repoFullName string) ([]model.PullRequest, error) {
	const query = `
		SELECT id, number, repo_full_name, title, author, status, is_draft, needs_review,
		       url, branch, base_branch, labels, head_sha, opened_at, updated_at, last_activity_at
		FROM pull_requests
		WHERE repo_full_name = ?
		ORDER BY number
	`

	return r.queryPRs(ctx, query, repoFullName)
}

// GetByStatus returns all pull requests with the given status, ordered by updated_at descending.
func (r *PRRepo) GetByStatus(ctx context.Context, status model.PRStatus) ([]model.PullRequest, error) {
	const query = `
		SELECT id, number, repo_full_name, title, author, status, is_draft, needs_review,
		       url, branch, base_branch, labels, head_sha, opened_at, updated_at, last_activity_at
		FROM pull_requests
		WHERE status = ?
		ORDER BY updated_at DESC
	`

	return r.queryPRs(ctx, query, string(status))
}

// GetByNumber retrieves a single pull request by repository and number.
// Returns nil, nil if the pull request does not exist.
func (r *PRRepo) GetByNumber(ctx context.Context, repoFullName string, number int) (*model.PullRequest, error) {
	const query = `
		SELECT id, number, repo_full_name, title, author, status, is_draft, needs_review,
		       url, branch, base_branch, labels, head_sha, opened_at, updated_at, last_activity_at
		FROM pull_requests
		WHERE repo_full_name = ? AND number = ?
	`

	pr, err := scanPR(r.db.Reader.QueryRowContext(ctx, query, repoFullName, number))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get PR %s#%d: %w", repoFullName, number, err)
	}

	return pr, nil
}

// ListAll returns all pull requests ordered by updated_at descending.
func (r *PRRepo) ListAll(ctx context.Context) ([]model.PullRequest, error) {
	const query = `
		SELECT id, number, repo_full_name, title, author, status, is_draft, needs_review,
		       url, branch, base_branch, labels, head_sha, opened_at, updated_at, last_activity_at
		FROM pull_requests
		ORDER BY updated_at DESC
	`

	return r.queryPRs(ctx, query)
}

// ListNeedingReview returns all pull requests where needs_review is true,
// ordered by updated_at descending.
func (r *PRRepo) ListNeedingReview(ctx context.Context) ([]model.PullRequest, error) {
	const query = `
		SELECT id, number, repo_full_name, title, author, status, is_draft, needs_review,
		       url, branch, base_branch, labels, head_sha, opened_at, updated_at, last_activity_at
		FROM pull_requests
		WHERE needs_review = 1
		ORDER BY updated_at DESC
	`

	return r.queryPRs(ctx, query)
}

// Delete removes a pull request by repository and number. Returns an error if
// the pull request does not exist.
func (r *PRRepo) Delete(ctx context.Context, repoFullName string, number int) error {
	const query = `DELETE FROM pull_requests WHERE repo_full_name = ? AND number = ?`

	result, err := r.db.Writer.ExecContext(ctx, query, repoFullName, number)
	if err != nil {
		return fmt.Errorf("delete PR %s#%d: %w", repoFullName, number, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("pull request %s#%d not found", repoFullName, number)
	}

	return nil
}

func (r *PRRepo) queryPRs(ctx context.Context, query string, args ...any) ([]model.PullRequest, error) {
	rows, err := r.db.Reader.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query pull requests: %w", err)
	}
	defer rows.Close()

	var prs []model.PullRequest
	for rows.Next() {
		pr, err := scanPR(rows)
		if err != nil {
			return nil, fmt.Errorf("scan pull request: %w", err)
		}
		prs = append(prs, *pr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pull requests: %w", err)
	}

	return prs, nil
}

func scanPR(s scanner) (*model.PullRequest, error) {
	var pr model.PullRequest
	var status string
	var isDraft int
	var needsReview int
	var labelsJSON string
	var openedAt, updatedAt, lastActivityAt string

	err := s.Scan(
		&pr.ID, &pr.Number, &pr.RepoFullName, &pr.Title, &pr.Author,
		&status, &isDraft, &needsReview, &pr.URL, &pr.Branch, &pr.BaseBranch,
		&labelsJSON, &pr.HeadSHA, &openedAt, &updatedAt, &lastActivityAt,
	)
	if err != nil {
		return nil, err
	}

	pr.Status = model.PRStatus(status)
	pr.IsDraft = isDraft != 0
	pr.NeedsReview = needsReview != 0

	if err := json.Unmarshal([]byte(labelsJSON), &pr.Labels); err != nil {
		return nil, fmt.Errorf("unmarshal labels: %w", err)
	}

	pr.OpenedAt, err = parseTime(openedAt)
	if err != nil {
		return nil, fmt.Errorf("parse opened_at: %w", err)
	}

	pr.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	pr.LastActivityAt, err = parseTime(lastActivityAt)
	if err != nil {
		return nil, fmt.Errorf("parse last_activity_at: %w", err)
	}

	return &pr, nil
}
