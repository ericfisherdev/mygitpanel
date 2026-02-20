package sqlite

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

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

// NewCredentialRepo creates a new CredentialRepo. key must be 32 bytes for AES-256-GCM,
// or nil to disable credential storage (all operations will return ErrEncryptionKeyNotSet).
func NewCredentialRepo(db *DB, key []byte) *CredentialRepo {
	return &CredentialRepo{db: db, key: key}
}

// Set stores or replaces the credential for the given service with the provided plaintext value.
func (r *CredentialRepo) Set(ctx context.Context, service, plaintext string) error {
	encrypted, err := r.encrypt(plaintext)
	if err != nil {
		return err
	}

	const query = `INSERT OR REPLACE INTO credentials (service, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)`
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

// encrypt encrypts plaintext using AES-256-GCM and returns a base64-encoded string
// containing the nonce (12 bytes) prepended to the ciphertext.
func (r *CredentialRepo) encrypt(plaintext string) (string, error) {
	if r.key == nil {
		return "", ErrEncryptionKeyNotSet
	}

	block, err := aes.NewCipher(r.key)
	if err != nil {
		return "", fmt.Errorf("aes.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("cipher.NewGCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("rand nonce: %w", err)
	}

	// Seal appends the ciphertext to nonce, producing: nonce || ciphertext || tag.
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts a base64-encoded AES-256-GCM ciphertext.
func (r *CredentialRepo) decrypt(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	block, err := aes.NewCipher(r.key)
	if err != nil {
		return "", fmt.Errorf("aes.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("cipher.NewGCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("gcm.Open: %w", err)
	}

	return string(plaintext), nil
}
