package driven

import (
	"context"
	"errors"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// ErrEncryptionKeyNotSet is returned by CredentialStore operations when
// MYGITPANEL_SECRET_KEY has not been configured.
var ErrEncryptionKeyNotSet = errors.New("encryption key not configured: set MYGITPANEL_SECRET_KEY")

// CredentialStore defines the driven port for encrypted credential persistence.
// The adapter layer is responsible for encryption/decryption; this interface
// operates on plaintext values at the domain boundary.
type CredentialStore interface {
	// Set stores or replaces the credential for the given service with the
	// provided plaintext value. Returns ErrEncryptionKeyNotSet if the adapter
	// was constructed without an encryption key.
	Set(ctx context.Context, service, plaintext string) error

	// Get retrieves the plaintext credential for the given service.
	// Returns ("", nil) if no credential exists for that service.
	// Returns ErrEncryptionKeyNotSet if the adapter was constructed without an encryption key.
	Get(ctx context.Context, service string) (string, error)

	// List returns all stored credentials. Values are decrypted plaintext.
	// Returns ErrEncryptionKeyNotSet if the adapter was constructed without an encryption key.
	List(ctx context.Context) ([]model.Credential, error)

	// Delete removes the credential for the given service.
	Delete(ctx context.Context, service string) error
}
