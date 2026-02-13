package driven

import (
	"context"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// CheckStore defines the driven port for check run persistence.
// Uses full replacement strategy: all check runs for a PR are replaced atomically.
type CheckStore interface {
	// ReplaceCheckRunsForPR deletes all existing check runs for the given PR
	// and inserts the provided runs atomically in a transaction.
	ReplaceCheckRunsForPR(ctx context.Context, prID int64, runs []model.CheckRun) error
	// GetCheckRunsByPR returns all check runs for the given PR, ordered by name.
	GetCheckRunsByPR(ctx context.Context, prID int64) ([]model.CheckRun, error)
}
