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

	// GetForRepo returns the Jira connection associated with the given repository.
	// Falls back to the default connection if no explicit mapping exists.
	// Returns a zero-value JiraConnection (ID==0) and nil error if no connection applies.
	GetForRepo(ctx context.Context, repoFullName string) (model.JiraConnection, error)

	// GetRepoMappings returns the assigned Jira connection ID for each of the given repositories
	// in a single query. Repos with no explicit mapping use the default connection ID (0 if none).
	GetRepoMappings(ctx context.Context, repoFullNames []string) (map[string]int64, error)

	// SetRepoMapping associates a repository with a Jira connection.
	// Pass connectionID=0 to clear the mapping.
	SetRepoMapping(ctx context.Context, repoFullName string, connectionID int64) error

	// SetDefault marks a connection as the default. Pass id=0 to clear the default.
	// Atomically clears is_default on all other connections before setting the new one.
	SetDefault(ctx context.Context, id int64) error
}
