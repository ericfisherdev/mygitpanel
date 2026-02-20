package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
)

// Compile-time interface satisfaction check.
var _ driven.ThresholdStore = (*ThresholdRepo)(nil)

// ThresholdRepo is the SQLite implementation of the ThresholdStore port interface.
type ThresholdRepo struct {
	db *DB
}

// NewThresholdRepo creates a new ThresholdRepo backed by the given DB.
func NewThresholdRepo(db *DB) *ThresholdRepo {
	return &ThresholdRepo{db: db}
}

// GetGlobalSettings returns the current global threshold defaults.
// Falls back to model.DefaultGlobalSettings() for any missing key or if the table is empty.
func (r *ThresholdRepo) GetGlobalSettings(ctx context.Context) (model.GlobalSettings, error) {
	const query = `SELECT key, value FROM global_settings WHERE key IN ('review_count_threshold', 'age_urgency_days', 'stale_review_enabled', 'ci_failure_enabled')`

	rows, err := r.db.Reader.QueryContext(ctx, query)
	if err != nil {
		return model.DefaultGlobalSettings(), fmt.Errorf("query global_settings: %w", err)
	}
	defer rows.Close()

	settings := model.DefaultGlobalSettings()
	found := 0
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return model.DefaultGlobalSettings(), fmt.Errorf("scan global_settings row: %w", err)
		}
		found++
		switch key {
		case "review_count_threshold":
			if v, err := strconv.Atoi(value); err == nil {
				settings.ReviewCountThreshold = v
			}
		case "age_urgency_days":
			if v, err := strconv.Atoi(value); err == nil {
				settings.AgeUrgencyDays = v
			}
		case "stale_review_enabled":
			settings.StaleReviewEnabled = value == "1"
		case "ci_failure_enabled":
			settings.CIFailureEnabled = value == "1"
		}
	}
	if err := rows.Err(); err != nil {
		return model.DefaultGlobalSettings(), fmt.Errorf("iterate global_settings: %w", err)
	}

	// If table is completely empty, return defaults (already set above).
	_ = found
	return settings, nil
}

// SetGlobalSettings persists the global threshold defaults using a transaction.
func (r *ThresholdRepo) SetGlobalSettings(ctx context.Context, settings model.GlobalSettings) error {
	tx, err := r.db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const upsert = `INSERT OR REPLACE INTO global_settings (key, value) VALUES (?, ?)`
	staleVal := "0"
	if settings.StaleReviewEnabled {
		staleVal = "1"
	}
	ciVal := "0"
	if settings.CIFailureEnabled {
		ciVal = "1"
	}

	rows := []struct{ key, value string }{
		{"review_count_threshold", strconv.Itoa(settings.ReviewCountThreshold)},
		{"age_urgency_days", strconv.Itoa(settings.AgeUrgencyDays)},
		{"stale_review_enabled", staleVal},
		{"ci_failure_enabled", ciVal},
	}
	for _, row := range rows {
		if _, err := tx.ExecContext(ctx, upsert, row.key, row.value); err != nil {
			return fmt.Errorf("upsert global_settings %q: %w", row.key, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit global_settings: %w", err)
	}
	return nil
}

// GetRepoThreshold returns the per-repository threshold overrides for the given repository.
// Returns a zero-value RepoThreshold (all nil pointers) when no override exists.
func (r *ThresholdRepo) GetRepoThreshold(ctx context.Context, repoFullName string) (model.RepoThreshold, error) {
	const query = `
		SELECT repo_full_name, review_count, age_urgency_days, stale_review_enabled, ci_failure_enabled
		FROM repo_thresholds
		WHERE repo_full_name = ?
	`

	var result model.RepoThreshold
	var reviewCount, ageUrgencyDays sql.NullInt64
	var staleEnabled, ciEnabled sql.NullInt64

	err := r.db.Reader.QueryRowContext(ctx, query, repoFullName).Scan(
		&result.RepoFullName,
		&reviewCount,
		&ageUrgencyDays,
		&staleEnabled,
		&ciEnabled,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.RepoThreshold{RepoFullName: repoFullName}, nil
	}
	if err != nil {
		return model.RepoThreshold{}, fmt.Errorf("get repo threshold %q: %w", repoFullName, err)
	}

	if reviewCount.Valid {
		v := int(reviewCount.Int64)
		result.ReviewCount = &v
	}
	if ageUrgencyDays.Valid {
		v := int(ageUrgencyDays.Int64)
		result.AgeUrgencyDays = &v
	}
	if staleEnabled.Valid {
		v := staleEnabled.Int64 != 0
		result.StaleReviewEnabled = &v
	}
	if ciEnabled.Valid {
		v := ciEnabled.Int64 != 0
		result.CIFailureEnabled = &v
	}

	return result, nil
}

// SetRepoThreshold persists per-repository threshold overrides.
func (r *ThresholdRepo) SetRepoThreshold(ctx context.Context, threshold model.RepoThreshold) error {
	const query = `
		INSERT OR REPLACE INTO repo_thresholds (repo_full_name, review_count, age_urgency_days, stale_review_enabled, ci_failure_enabled)
		VALUES (?, ?, ?, ?, ?)
	`

	var reviewCount, ageUrgencyDays, staleEnabled, ciEnabled interface{}
	if threshold.ReviewCount != nil {
		reviewCount = *threshold.ReviewCount
	}
	if threshold.AgeUrgencyDays != nil {
		ageUrgencyDays = *threshold.AgeUrgencyDays
	}
	if threshold.StaleReviewEnabled != nil {
		if *threshold.StaleReviewEnabled {
			staleEnabled = 1
		} else {
			staleEnabled = 0
		}
	}
	if threshold.CIFailureEnabled != nil {
		if *threshold.CIFailureEnabled {
			ciEnabled = 1
		} else {
			ciEnabled = 0
		}
	}

	_, err := r.db.Writer.ExecContext(ctx, query,
		threshold.RepoFullName, reviewCount, ageUrgencyDays, staleEnabled, ciEnabled,
	)
	if err != nil {
		return fmt.Errorf("set repo threshold %q: %w", threshold.RepoFullName, err)
	}
	return nil
}

// DeleteRepoThreshold removes the per-repository override for the given repo,
// causing it to fall back to global settings.
func (r *ThresholdRepo) DeleteRepoThreshold(ctx context.Context, repoFullName string) error {
	const query = `DELETE FROM repo_thresholds WHERE repo_full_name = ?`
	_, err := r.db.Writer.ExecContext(ctx, query, repoFullName)
	if err != nil {
		return fmt.Errorf("delete repo threshold %q: %w", repoFullName, err)
	}
	return nil
}
