package model

import "time"

// Repository represents a GitHub repository watched by ReviewHub.
type Repository struct {
	ID       int64
	FullName string
	Owner    string
	Name     string
	AddedAt  time.Time
}
