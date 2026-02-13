package sqlite

import (
	"context"
	"fmt"

	"github.com/efisher/reviewhub/internal/domain/model"
	"github.com/efisher/reviewhub/internal/domain/port/driven"
)

// Compile-time interface satisfaction check.
var _ driven.BotConfigStore = (*BotConfigRepo)(nil)

// BotConfigRepo is the SQLite implementation of the BotConfigStore port interface.
type BotConfigRepo struct {
	db *DB
}

// NewBotConfigRepo creates a new BotConfigRepo backed by the given DB.
func NewBotConfigRepo(db *DB) *BotConfigRepo {
	return &BotConfigRepo{db: db}
}

// Add inserts a new bot configuration entry. Returns an error if the username
// already exists (unique constraint violation).
func (r *BotConfigRepo) Add(ctx context.Context, config model.BotConfig) error {
	const query = `INSERT INTO bot_config (username, added_at) VALUES (?, ?)`

	addedAt := config.AddedAt
	if addedAt.IsZero() {
		addedAt = addedAt.UTC()
	}

	_, err := r.db.Writer.ExecContext(ctx, query, config.Username, addedAt.UTC())
	if err != nil {
		return fmt.Errorf("add bot config %q: %w", config.Username, err)
	}

	return nil
}

// Remove deletes a bot configuration entry by username. Returns an error if
// the username does not exist.
func (r *BotConfigRepo) Remove(ctx context.Context, username string) error {
	const query = `DELETE FROM bot_config WHERE username = ?`

	result, err := r.db.Writer.ExecContext(ctx, query, username)
	if err != nil {
		return fmt.Errorf("remove bot config %q: %w", username, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("bot config %q not found", username)
	}

	return nil
}

// ListAll returns all bot configuration entries ordered by username.
func (r *BotConfigRepo) ListAll(ctx context.Context) ([]model.BotConfig, error) {
	const query = `SELECT id, username, added_at FROM bot_config ORDER BY username`

	rows, err := r.db.Reader.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list bot configs: %w", err)
	}
	defer rows.Close()

	var configs []model.BotConfig
	for rows.Next() {
		var config model.BotConfig
		var addedAt string

		if err := rows.Scan(&config.ID, &config.Username, &addedAt); err != nil {
			return nil, fmt.Errorf("scan bot config: %w", err)
		}

		config.AddedAt, err = parseTime(addedAt)
		if err != nil {
			return nil, fmt.Errorf("parse added_at: %w", err)
		}

		configs = append(configs, config)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bot configs: %w", err)
	}

	return configs, nil
}

// GetUsernames returns only the username strings of all bot configurations,
// ordered alphabetically.
func (r *BotConfigRepo) GetUsernames(ctx context.Context) ([]string, error) {
	const query = `SELECT username FROM bot_config ORDER BY username`

	rows, err := r.db.Reader.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get bot usernames: %w", err)
	}
	defer rows.Close()

	var usernames []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, fmt.Errorf("scan username: %w", err)
		}
		usernames = append(usernames, username)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate usernames: %w", err)
	}

	return usernames, nil
}
