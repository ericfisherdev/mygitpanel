package model

import "time"

// IgnoredPR records that a specific pull request has been excluded from
// the attention dashboard by the user.
type IgnoredPR struct {
	ID           int64
	RepoFullName string
	PRNumber     int
	IgnoredAt    time.Time
}
