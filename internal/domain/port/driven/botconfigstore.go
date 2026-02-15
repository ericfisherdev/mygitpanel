// Package driven defines secondary port interfaces for external adapters.
package driven

import (
	"context"
	"errors"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// Sentinel errors returned by BotConfigStore implementations.
var (
	// ErrBotNotFound indicates the requested bot configuration does not exist.
	ErrBotNotFound = errors.New("bot config not found")

	// ErrBotAlreadyExists indicates a bot with the same username already exists.
	ErrBotAlreadyExists = errors.New("bot config already exists")
)

// BotConfigStore defines the driven port for managing bot username configuration.
// Add returns ErrBotAlreadyExists if the username already exists.
// Remove returns ErrBotNotFound if the username does not exist.
type BotConfigStore interface {
	Add(ctx context.Context, config model.BotConfig) (model.BotConfig, error)
	Remove(ctx context.Context, username string) error
	ListAll(ctx context.Context) ([]model.BotConfig, error)
	// GetUsernames returns only the username strings, ordered alphabetically.
	GetUsernames(ctx context.Context) ([]string, error)
}
