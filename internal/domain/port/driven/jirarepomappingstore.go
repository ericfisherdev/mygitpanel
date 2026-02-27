package driven

import (
	"context"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// JiraRepoMappingStore defines the driven port for associating repositories
// with Jira connections.
type JiraRepoMappingStore interface {
	// GetForRepo returns the Jira connection associated with the given repository.
	// Falls back to the default connection if no explicit mapping exists.
	// Returns a zero-value JiraConnection (ID==0) and nil error if no connection applies.
	GetForRepo(ctx context.Context, repoFullName string) (model.JiraConnection, error)

	// GetRepoMappings returns the assigned Jira connection ID for each of the given
	// repositories in a single query. Repos with no explicit mapping use the default
	// connection ID (0 if none exists).
	GetRepoMappings(ctx context.Context, repoFullNames []string) (map[string]int64, error)

	// SetRepoMapping associates a repository with a Jira connection.
	// Pass connectionID=0 to clear the mapping.
	SetRepoMapping(ctx context.Context, repoFullName string, connectionID int64) error
}
