package driven

import (
	"context"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// JiraConnectionStore defines the driven port for Jira connection persistence.
// Tokens are stored encrypted; all methods return decrypted plaintext at the domain boundary.
type JiraConnectionStore interface {
	// Create persists a new Jira connection and returns the assigned ID.
	// The token is encrypted before storage.
	Create(ctx context.Context, conn model.JiraConnection) (int64, error)

	// Update replaces all fields of an existing Jira connection.
	// The token is re-encrypted before storage.
	Update(ctx context.Context, conn model.JiraConnection) error

	// Delete removes a Jira connection by ID. Cascades to repo mappings via FK.
	Delete(ctx context.Context, id int64) error

	// List returns all Jira connections with decrypted tokens, ordered by display name.
	List(ctx context.Context) ([]model.JiraConnection, error)

	// GetByID retrieves a single Jira connection by ID.
	// Returns a zero-value JiraConnection (ID==0) and nil error if not found.
	GetByID(ctx context.Context, id int64) (model.JiraConnection, error)

	// SetDefault marks a connection as the default. Pass id=0 to clear the default.
	// Atomically clears is_default on all other connections before setting the new one.
	SetDefault(ctx context.Context, id int64) error
}
