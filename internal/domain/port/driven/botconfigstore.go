package driven

import (
	"context"

	"github.com/efisher/reviewhub/internal/domain/model"
)

// BotConfigStore defines the driven port for managing bot username configuration.
type BotConfigStore interface {
	Add(ctx context.Context, config model.BotConfig) error
	Remove(ctx context.Context, username string) error
	ListAll(ctx context.Context) ([]model.BotConfig, error)
	// GetUsernames returns only the username strings, ordered alphabetically.
	GetUsernames(ctx context.Context) ([]string, error)
}
