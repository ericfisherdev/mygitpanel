package model

import "time"

// BotConfig holds a configured bot username for filtering bot comments
// from human comments in review intelligence features.
type BotConfig struct {
	ID       int64
	Username string // e.g., "coderabbitai", "github-actions[bot]"
	AddedAt  time.Time
}
