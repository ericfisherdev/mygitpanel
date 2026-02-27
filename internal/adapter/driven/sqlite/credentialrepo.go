package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
)

// ErrEncryptionKeyNotSet aliases the port-level sentinel so callers import only the driven port.
//
// Deprecated: use driven.ErrEncryptionKeyNotSet directly.
var ErrEncryptionKeyNotSet = driven.ErrEncryptionKeyNotSet

// Compile-time interface satisfaction check.
var _ driven.CredentialStore = (*CredentialRepo)(nil)

// CredentialRepo is the SQLite implementation of the CredentialStore port interface.
// Credential values are encrypted with AES-256-GCM before write and decrypted after read.
type CredentialRepo struct {
	db  *DB
	key []byte // 32-byte AES-256 key; nil when encryption is disabled.
}

// NewCredentialRepo creates a new CredentialRepo. key must be exactly 32 bytes for
// AES-256-GCM, or nil to disable credential storage (all operations will return
// ErrEncryptionKeyNotSet). Panics if key is non-nil with wrong length.
func NewCredentialRepo(db *DB, key []byte) *CredentialRepo {
	if key != nil && len(key) != 32 {
		panic(fmt.Errorf("invalid AES-256 key length: got %d, want 32", len(key)))
	}
	return &CredentialRepo{db: db, key: key}
}

// Set stores or replaces the credential for the given service with the provided plaintext value.
func (r *CredentialRepo) Set(ctx context.Context, service, plaintext string) error {
	encrypted, err := r.encrypt(plaintext)
	if err != nil {
		return err
	}

	const query = `INSERT INTO credentials (service, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(service) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`
	_, err = r.db.Writer.ExecContext(ctx, query, service, encrypted)
	if err != nil {
		return fmt.Errorf("set credential %q: %w", service, err)
	}
	return nil
}

// Get retrieves the plaintext credential for the given service.
// Returns ("", nil) if no credential exists for that service.
func (r *CredentialRepo) Get(ctx context.Context, service string) (string, error) {
	if r.key == nil {
		return "", ErrEncryptionKeyNotSet
	}

	const query = `SELECT value FROM credentials WHERE service = ?`
	var encrypted string
	err := r.db.Reader.QueryRowContext(ctx, query, service).Scan(&encrypted)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get credential %q: %w", service, err)
	}

	plaintext, err := r.decrypt(encrypted)
	if err != nil {
		return "", fmt.Errorf("decrypt credential %q: %w", service, err)
	}
	return plaintext, nil
}

// List returns all stored credentials with decrypted values.
func (r *CredentialRepo) List(ctx context.Context) ([]model.Credential, error) {
	if r.key == nil {
		return nil, ErrEncryptionKeyNotSet
	}

	const query = `SELECT id, service, value, updated_at FROM credentials ORDER BY service`
	rows, err := r.db.Reader.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list credentials: %w", err)
	}
	defer rows.Close()

	var creds []model.Credential
	for rows.Next() {
		var cred model.Credential
		var encrypted string
		var updatedAt string
		if err := rows.Scan(&cred.ID, &cred.Service, &encrypted, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan credential: %w", err)
		}

		plaintext, err := r.decrypt(encrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypt credential %q: %w", cred.Service, err)
		}
		cred.Value = plaintext

		cred.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse updated_at for credential %q: %w", cred.Service, err)
		}

		creds = append(creds, cred)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate credentials: %w", err)
	}

	return creds, nil
}

// Delete removes the credential for the given service.
func (r *CredentialRepo) Delete(ctx context.Context, service string) error {
	const query = `DELETE FROM credentials WHERE service = ?`
	_, err := r.db.Writer.ExecContext(ctx, query, service)
	if err != nil {
		return fmt.Errorf("delete credential %q: %w", service, err)
	}
	return nil
}

// encrypt encrypts plaintext using AES-256-GCM via the shared sqlite-package helper.
func (r *CredentialRepo) encrypt(plaintext string) (string, error) {
	return encryptAES(r.key, plaintext)
}

// decrypt decrypts a base64-encoded AES-256-GCM ciphertext via the shared sqlite-package helper.
func (r *CredentialRepo) decrypt(encoded string) (string, error) {
	return decryptAES(r.key, encoded)
}
