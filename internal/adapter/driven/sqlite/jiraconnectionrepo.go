package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
)

// Compile-time interface satisfaction checks.
var _ driven.JiraConnectionStore = (*JiraConnectionRepo)(nil)
var _ driven.JiraRepoMappingStore = (*JiraConnectionRepo)(nil)

// JiraConnectionRepo is the SQLite implementation of the JiraConnectionStore port interface.
// API tokens are encrypted with AES-256-GCM before write and decrypted after read.
type JiraConnectionRepo struct {
	db  *DB
	key []byte // 32-byte AES-256 key; nil when encryption is disabled.
}

// NewJiraConnectionRepo creates a new JiraConnectionRepo. key must be exactly 32 bytes
// for AES-256-GCM, or nil to disable credential storage (all operations will return
// ErrEncryptionKeyNotSet). Panics if key is non-nil with wrong length.
func NewJiraConnectionRepo(db *DB, key []byte) *JiraConnectionRepo {
	if key != nil && len(key) != 32 {
		panic(fmt.Errorf("invalid AES-256 key length: got %d, want 32", len(key)))
	}
	return &JiraConnectionRepo{db: db, key: key}
}

// Create persists a new Jira connection and returns the assigned ID.
// is_default is always inserted as 0; if conn.IsDefault is true the SetDefault logic
// is applied atomically within the same transaction.
func (r *JiraConnectionRepo) Create(ctx context.Context, conn model.JiraConnection) (int64, error) {
	encrypted, err := r.encrypt(conn.Token)
	if err != nil {
		return 0, err
	}

	tx, err := r.db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("create jira connection: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	const query = `INSERT INTO jira_connections (display_name, base_url, email, token, is_default)
		VALUES (?, ?, ?, ?, 0)`
	result, err := tx.ExecContext(ctx, query,
		conn.DisplayName, conn.BaseURL, conn.Email, encrypted,
	)
	if err != nil {
		return 0, fmt.Errorf("create jira connection: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("create jira connection: last insert id: %w", err)
	}

	if conn.IsDefault {
		if err := setDefaultInTx(ctx, tx, id); err != nil {
			return 0, fmt.Errorf("create jira connection: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("create jira connection: commit: %w", err)
	}
	return id, nil
}

// Update replaces all fields of an existing Jira connection.
// is_default is always written as 0; if conn.IsDefault is true the SetDefault logic
// is applied atomically within the same transaction.
// Returns an error if no row with conn.ID exists.
func (r *JiraConnectionRepo) Update(ctx context.Context, conn model.JiraConnection) error {
	encrypted, err := r.encrypt(conn.Token)
	if err != nil {
		return err
	}

	tx, err := r.db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("update jira connection %d: begin tx: %w", conn.ID, err)
	}
	defer tx.Rollback() //nolint:errcheck

	const query = `UPDATE jira_connections
		SET display_name = ?, base_url = ?, email = ?, token = ?, is_default = 0, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`
	result, err := tx.ExecContext(ctx, query,
		conn.DisplayName, conn.BaseURL, conn.Email, encrypted, conn.ID,
	)
	if err != nil {
		return fmt.Errorf("update jira connection %d: %w", conn.ID, err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update jira connection %d: rows affected: %w", conn.ID, err)
	}
	if n == 0 {
		return fmt.Errorf("update jira connection %d: not found", conn.ID)
	}

	if conn.IsDefault {
		if err := setDefaultInTx(ctx, tx, conn.ID); err != nil {
			return fmt.Errorf("update jira connection %d: %w", conn.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("update jira connection %d: commit: %w", conn.ID, err)
	}
	return nil
}

// Delete removes a Jira connection by ID. FK cascade handles repo_jira_mapping cleanup.
func (r *JiraConnectionRepo) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM jira_connections WHERE id = ?`
	_, err := r.db.Writer.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete jira connection %d: %w", id, err)
	}
	return nil
}

// List returns all Jira connections with decrypted tokens, ordered by display name.
func (r *JiraConnectionRepo) List(ctx context.Context) ([]model.JiraConnection, error) {
	if r.key == nil {
		return nil, driven.ErrEncryptionKeyNotSet
	}

	const query = `SELECT id, display_name, base_url, email, token, is_default, created_at, updated_at
		FROM jira_connections ORDER BY display_name`
	rows, err := r.db.Reader.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list jira connections: %w", err)
	}
	defer rows.Close()

	var conns []model.JiraConnection
	for rows.Next() {
		conn, err := r.scanConnection(rows)
		if err != nil {
			return nil, fmt.Errorf("scan jira connection: %w", err)
		}
		conns = append(conns, conn)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jira connections: %w", err)
	}
	return conns, nil
}

// GetByID retrieves a single Jira connection by ID.
// Returns a zero-value JiraConnection (ID==0) and nil error if not found.
func (r *JiraConnectionRepo) GetByID(ctx context.Context, id int64) (model.JiraConnection, error) {
	if r.key == nil {
		return model.JiraConnection{}, driven.ErrEncryptionKeyNotSet
	}

	const query = `SELECT id, display_name, base_url, email, token, is_default, created_at, updated_at
		FROM jira_connections WHERE id = ?`
	conn, err := r.scanConnection(r.db.Reader.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return model.JiraConnection{}, nil
	}
	if err != nil {
		return model.JiraConnection{}, fmt.Errorf("get jira connection %d: %w", id, err)
	}
	return conn, nil
}

// GetForRepo returns the Jira connection associated with the given repository.
// Falls back to the default connection if no explicit mapping exists.
// Returns a zero-value JiraConnection (ID==0) and nil error if no connection applies.
func (r *JiraConnectionRepo) GetForRepo(ctx context.Context, repoFullName string) (model.JiraConnection, error) {
	if r.key == nil {
		return model.JiraConnection{}, driven.ErrEncryptionKeyNotSet
	}

	// Try explicit mapping first, then fall back to default connection.
	const query = `
		SELECT jc.id, jc.display_name, jc.base_url, jc.email, jc.token, jc.is_default, jc.created_at, jc.updated_at
		FROM jira_connections jc
		LEFT JOIN repo_jira_mapping rjm ON rjm.jira_connection_id = jc.id AND rjm.repo_full_name = ?
		WHERE rjm.repo_full_name IS NOT NULL OR jc.is_default = 1
		ORDER BY CASE WHEN rjm.repo_full_name IS NOT NULL THEN 0 ELSE 1 END
		LIMIT 1`

	conn, err := r.scanConnection(r.db.Reader.QueryRowContext(ctx, query, repoFullName))
	if errors.Is(err, sql.ErrNoRows) {
		return model.JiraConnection{}, nil
	}
	if err != nil {
		return model.JiraConnection{}, fmt.Errorf("get jira connection for repo %s: %w", repoFullName, err)
	}
	return conn, nil
}

// GetRepoMappings returns the assigned Jira connection ID for each given repo in a single query.
// Repos with no explicit mapping fall back to the default connection ID (0 if none exists).
func (r *JiraConnectionRepo) GetRepoMappings(ctx context.Context, repoFullNames []string) (map[string]int64, error) {
	if len(repoFullNames) == 0 {
		return map[string]int64{}, nil
	}

	placeholders := strings.Repeat("?,", len(repoFullNames))
	placeholders = placeholders[:len(placeholders)-1]

	args := make([]any, len(repoFullNames))
	for i, name := range repoFullNames {
		args[i] = name
	}

	//nolint:gosec // placeholders contains only comma-separated "?" literals, never user input
	query := fmt.Sprintf(
		`SELECT repo_full_name, jira_connection_id FROM repo_jira_mapping WHERE repo_full_name IN (%s)`,
		placeholders,
	)

	rows, err := r.db.Reader.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get repo mappings: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int64, len(repoFullNames))
	for rows.Next() {
		var repo string
		var connID int64
		if err := rows.Scan(&repo, &connID); err != nil {
			return nil, fmt.Errorf("scan repo mapping: %w", err)
		}
		result[repo] = connID
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repo mappings: %w", err)
	}

	// Fetch the default connection ID once for repos with no explicit mapping.
	var defaultID int64
	for _, name := range repoFullNames {
		if _, ok := result[name]; !ok {
			if defaultID == 0 {
				scanErr := r.db.Reader.QueryRowContext(ctx,
					`SELECT id FROM jira_connections WHERE is_default = 1 LIMIT 1`,
				).Scan(&defaultID)
				if scanErr != nil && !errors.Is(scanErr, sql.ErrNoRows) {
					return nil, fmt.Errorf("get default jira connection: %w", scanErr)
				}
			}
			result[name] = defaultID
		}
	}

	return result, nil
}

// SetRepoMapping associates a repository with a Jira connection.
// Pass connectionID=0 to clear the mapping.
func (r *JiraConnectionRepo) SetRepoMapping(ctx context.Context, repoFullName string, connectionID int64) error {
	if connectionID == 0 {
		const query = `DELETE FROM repo_jira_mapping WHERE repo_full_name = ?`
		_, err := r.db.Writer.ExecContext(ctx, query, repoFullName)
		if err != nil {
			return fmt.Errorf("clear repo jira mapping %s: %w", repoFullName, err)
		}
		return nil
	}

	const query = `INSERT INTO repo_jira_mapping (repo_full_name, jira_connection_id) VALUES (?, ?)
		ON CONFLICT(repo_full_name) DO UPDATE SET jira_connection_id = excluded.jira_connection_id`
	_, err := r.db.Writer.ExecContext(ctx, query, repoFullName, connectionID)
	if err != nil {
		return fmt.Errorf("set repo jira mapping %s -> %d: %w", repoFullName, connectionID, err)
	}
	return nil
}

// SetDefault marks a connection as the default. Pass id=0 to clear the default.
// Atomically clears is_default on all other connections before setting the new one.
func (r *JiraConnectionRepo) SetDefault(ctx context.Context, id int64) error {
	tx, err := r.db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if id == 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE jira_connections SET is_default = 0 WHERE is_default = 1`); err != nil {
			return fmt.Errorf("clear defaults: %w", err)
		}
	} else {
		if err := setDefaultInTx(ctx, tx, id); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit set default: %w", err)
	}
	return nil
}

// setDefaultInTx clears is_default on all connections then marks id as default.
// Must be called within an active transaction. id must be > 0.
func setDefaultInTx(ctx context.Context, tx *sql.Tx, id int64) error {
	if _, err := tx.ExecContext(ctx, `UPDATE jira_connections SET is_default = 0 WHERE is_default = 1`); err != nil {
		return fmt.Errorf("clear defaults: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE jira_connections SET is_default = 1 WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("set default %d: %w", id, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("set default %d: rows affected: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("set default %d: no rows affected", id)
	}
	return nil
}

// scanConnection scans a single jira_connections row from the given scanner.
func (r *JiraConnectionRepo) scanConnection(s scanner) (model.JiraConnection, error) {
	var conn model.JiraConnection
	var encrypted string
	var isDefault int
	var createdAt, updatedAt string

	err := s.Scan(
		&conn.ID, &conn.DisplayName, &conn.BaseURL, &conn.Email,
		&encrypted, &isDefault, &createdAt, &updatedAt,
	)
	if err != nil {
		return model.JiraConnection{}, err
	}

	conn.IsDefault = isDefault != 0

	conn.Token, err = r.decrypt(encrypted)
	if err != nil {
		return model.JiraConnection{}, fmt.Errorf("decrypt token for connection %d: %w", conn.ID, err)
	}

	conn.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return model.JiraConnection{}, fmt.Errorf("parse created_at for connection %d: %w", conn.ID, err)
	}

	conn.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return model.JiraConnection{}, fmt.Errorf("parse updated_at for connection %d: %w", conn.ID, err)
	}

	return conn, nil
}

// encrypt encrypts plaintext using AES-256-GCM via the shared sqlite-package helper.
func (r *JiraConnectionRepo) encrypt(plaintext string) (string, error) {
	return encryptAES(r.key, plaintext)
}

// decrypt decrypts a base64-encoded AES-256-GCM ciphertext via the shared sqlite-package helper.
func (r *JiraConnectionRepo) decrypt(encoded string) (string, error) {
	return decryptAES(r.key, encoded)
}
