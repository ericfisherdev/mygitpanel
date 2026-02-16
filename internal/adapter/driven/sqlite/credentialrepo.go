package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
)

// Compile-time interface satisfaction check.
var _ driven.CredentialStore = (*CredentialRepo)(nil)

// CredentialRepo is the SQLite implementation of the CredentialStore port interface.
type CredentialRepo struct {
	db *DB
}

// NewCredentialRepo creates a new CredentialRepo backed by the given DB.
func NewCredentialRepo(db *DB) *CredentialRepo {
	return &CredentialRepo{db: db}
}

// Set inserts or updates a credential. If the (service, key) pair already
// exists, its value and updated_at timestamp are replaced.
func (r *CredentialRepo) Set(ctx context.Context, service, key, value string) error {
	const query = `
		INSERT INTO credentials (service, key, value, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(service, key) DO UPDATE SET
			value = excluded.value,
			updated_at = excluded.updated_at
	`

	_, err := r.db.Writer.ExecContext(ctx, query, service, key, value, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("set credential %s/%s: %w", service, key, err)
	}

	return nil
}

// Get retrieves a single credential value. Returns ("", nil) if the
// credential does not exist.
func (r *CredentialRepo) Get(ctx context.Context, service, key string) (string, error) {
	const query = `SELECT value FROM credentials WHERE service = ? AND key = ?`

	var value string

	err := r.db.Reader.QueryRowContext(ctx, query, service, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get credential %s/%s: %w", service, key, err)
	}

	return value, nil
}

// GetAll retrieves all credential key-value pairs for the given service.
// Returns an empty map if no credentials exist.
func (r *CredentialRepo) GetAll(ctx context.Context, service string) (map[string]string, error) {
	const query = `SELECT key, value FROM credentials WHERE service = ?`

	rows, err := r.db.Reader.QueryContext(ctx, query, service)
	if err != nil {
		return nil, fmt.Errorf("get all credentials for %s: %w", service, err)
	}
	defer rows.Close()

	creds := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scan credential: %w", err)
		}
		creds[k] = v
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate credentials: %w", err)
	}

	return creds, nil
}

// Delete removes a credential by service and key. No error is returned if
// the credential does not exist.
func (r *CredentialRepo) Delete(ctx context.Context, service, key string) error {
	const query = `DELETE FROM credentials WHERE service = ? AND key = ?`

	_, err := r.db.Writer.ExecContext(ctx, query, service, key)
	if err != nil {
		return fmt.Errorf("delete credential %s/%s: %w", service, key, err)
	}

	return nil
}
