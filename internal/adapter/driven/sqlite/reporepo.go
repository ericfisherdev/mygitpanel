package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
)

// Compile-time interface satisfaction check.
var _ driven.RepoStore = (*RepoRepo)(nil)

// RepoRepo is the SQLite implementation of the RepoStore port interface.
type RepoRepo struct {
	db *DB
}

// NewRepoRepo creates a new RepoRepo backed by the given DB.
func NewRepoRepo(db *DB) *RepoRepo {
	return &RepoRepo{db: db}
}

// Add inserts a new repository. Returns an error if a repository with the same
// full_name already exists.
func (r *RepoRepo) Add(ctx context.Context, repo model.Repository) error {
	const query = `INSERT INTO repositories (full_name, owner, name, added_at) VALUES (?, ?, ?, ?)`

	addedAt := repo.AddedAt
	if addedAt.IsZero() {
		addedAt = time.Now().UTC()
	}

	_, err := r.db.Writer.ExecContext(ctx, query, repo.FullName, repo.Owner, repo.Name, addedAt)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return fmt.Errorf("add repository %s: %w", repo.FullName, driven.ErrRepoAlreadyExists)
		}
		return fmt.Errorf("add repository %s: %w", repo.FullName, err)
	}

	return nil
}

// Remove deletes a repository by full name. Returns an error if the repository
// does not exist. Due to foreign key cascade, all associated pull requests are
// also deleted.
func (r *RepoRepo) Remove(ctx context.Context, fullName string) error {
	const query = `DELETE FROM repositories WHERE full_name = ?`

	result, err := r.db.Writer.ExecContext(ctx, query, fullName)
	if err != nil {
		return fmt.Errorf("remove repository %s: %w", fullName, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("remove repository %s: %w", fullName, driven.ErrRepoNotFound)
	}

	return nil
}

// GetByFullName retrieves a repository by its full name. Returns nil, nil if
// the repository does not exist.
func (r *RepoRepo) GetByFullName(ctx context.Context, fullName string) (*model.Repository, error) {
	const query = `SELECT id, full_name, owner, name, added_at FROM repositories WHERE full_name = ?`

	repo, err := scanRepository(r.db.Reader.QueryRowContext(ctx, query, fullName))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get repository %s: %w", fullName, err)
	}

	return repo, nil
}

// ListAll returns all repositories ordered by full name.
func (r *RepoRepo) ListAll(ctx context.Context) ([]model.Repository, error) {
	const query = `SELECT id, full_name, owner, name, added_at FROM repositories ORDER BY full_name`

	rows, err := r.db.Reader.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
	}
	defer rows.Close()

	var repos []model.Repository
	for rows.Next() {
		repo, err := scanRepository(rows)
		if err != nil {
			return nil, fmt.Errorf("scan repository: %w", err)
		}
		repos = append(repos, *repo)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repositories: %w", err)
	}

	return repos, nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanRepository(s scanner) (*model.Repository, error) {
	var repo model.Repository
	var addedAt string

	err := s.Scan(&repo.ID, &repo.FullName, &repo.Owner, &repo.Name, &addedAt)
	if err != nil {
		return nil, err
	}

	repo.AddedAt, err = parseTime(addedAt)
	if err != nil {
		return nil, fmt.Errorf("parse added_at: %w", err)
	}

	return &repo, nil
}

// parseTime tries multiple SQLite datetime formats.
func parseTime(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.000",
		time.RFC3339,
		time.RFC3339Nano,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized time format: %s", s)
}
