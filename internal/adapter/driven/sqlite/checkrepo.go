package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
)

// Compile-time interface satisfaction check.
var _ driven.CheckStore = (*CheckRepo)(nil)

// CheckRepo is the SQLite implementation of the CheckStore port interface.
type CheckRepo struct {
	db *DB
}

// NewCheckRepo creates a new CheckRepo backed by the given DB.
func NewCheckRepo(db *DB) *CheckRepo {
	return &CheckRepo{db: db}
}

// ReplaceCheckRunsForPR atomically replaces all check runs for a PR.
// It deletes existing runs and inserts the provided runs in a single transaction.
func (r *CheckRepo) ReplaceCheckRunsForPR(ctx context.Context, prID int64, runs []model.CheckRun) error {
	tx, err := r.db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // Rollback after commit is a no-op.

	const deleteQuery = `DELETE FROM check_runs WHERE pr_id = ?`
	if _, err := tx.ExecContext(ctx, deleteQuery, prID); err != nil {
		return fmt.Errorf("delete check runs for PR %d: %w", prID, err)
	}

	const insertQuery = `
		INSERT INTO check_runs (id, pr_id, name, status, conclusion, is_required, details_url, started_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	for _, run := range runs {
		isRequired := 0
		if run.IsRequired {
			isRequired = 1
		}

		var startedAt, completedAt any
		if !run.StartedAt.IsZero() {
			startedAt = run.StartedAt.UTC()
		}
		if !run.CompletedAt.IsZero() {
			completedAt = run.CompletedAt.UTC()
		}

		if _, err := tx.ExecContext(ctx, insertQuery,
			run.ID, prID, run.Name, run.Status, run.Conclusion,
			isRequired, run.DetailsURL, startedAt, completedAt,
		); err != nil {
			return fmt.Errorf("insert check run %d for PR %d: %w", run.ID, prID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit check runs for PR %d: %w", prID, err)
	}

	return nil
}

// GetCheckRunsByPR returns all check runs for the given PR, ordered by name.
func (r *CheckRepo) GetCheckRunsByPR(ctx context.Context, prID int64) ([]model.CheckRun, error) {
	const query = `
		SELECT id, pr_id, name, status, conclusion, is_required, details_url, started_at, completed_at
		FROM check_runs
		WHERE pr_id = ?
		ORDER BY name
	`

	rows, err := r.db.Reader.QueryContext(ctx, query, prID)
	if err != nil {
		return nil, fmt.Errorf("query check runs for PR %d: %w", prID, err)
	}
	defer rows.Close()

	var runs []model.CheckRun
	for rows.Next() {
		run, err := scanCheckRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan check run: %w", err)
		}
		runs = append(runs, *run)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate check runs: %w", err)
	}

	return runs, nil
}

func scanCheckRun(s scanner) (*model.CheckRun, error) {
	var run model.CheckRun
	var isRequired int
	var startedAt, completedAt sql.NullString

	err := s.Scan(
		&run.ID, &run.PRID, &run.Name, &run.Status, &run.Conclusion,
		&isRequired, &run.DetailsURL, &startedAt, &completedAt,
	)
	if err != nil {
		return nil, err
	}

	run.IsRequired = isRequired != 0

	if startedAt.Valid {
		run.StartedAt, err = parseTime(startedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse started_at: %w", err)
		}
	}

	if completedAt.Valid {
		run.CompletedAt, err = parseTime(completedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse completed_at: %w", err)
		}
	}

	return &run, nil
}
