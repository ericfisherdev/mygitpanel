package driven

import "context"

// CredentialStore defines the driven port for credential persistence.
// Get returns ("", nil) if the credential does not exist — queries return
// empty values for missing entities rather than an error.
// GetAll returns an empty map if no credentials exist for the service.
type CredentialStore interface {
	Set(ctx context.Context, service, key, value string) error
	Get(ctx context.Context, service, key string) (string, error)
	GetAll(ctx context.Context, service string) (map[string]string, error)
	Delete(ctx context.Context, service, key string) error
}
