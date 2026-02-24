package driven

import (
	"context"
	"errors"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// Sentinel errors for JiraClient operations.
var (
	// ErrJiraNotFound is returned when the requested Jira issue does not exist.
	ErrJiraNotFound = errors.New("jira issue not found")

	// ErrJiraUnauthorized is returned when Jira credentials are invalid or expired.
	ErrJiraUnauthorized = errors.New("jira unauthorized: invalid credentials")

	// ErrJiraUnavailable is returned when the Jira instance is unreachable.
	ErrJiraUnavailable = errors.New("jira instance unavailable")
)

// JiraClient defines the driven port for interacting with a Jira Cloud instance.
type JiraClient interface {
	// GetIssue retrieves a Jira issue by key (e.g. "PROJ-123").
	// Returns ErrJiraNotFound if the issue does not exist,
	// ErrJiraUnauthorized if credentials are invalid,
	// ErrJiraUnavailable if the Jira instance is unreachable.
	GetIssue(ctx context.Context, key string) (model.JiraIssue, error)

	// AddComment posts a comment on the specified Jira issue.
	// The adapter wraps body in ADF format before sending to the Jira API.
	AddComment(ctx context.Context, key, body string) error

	// Ping validates connectivity and credentials via GET /rest/api/3/myself.
	// Used for credential validation on save.
	Ping(ctx context.Context) error
}
