package model

import "time"

// Credential holds a service credential key-value pair. Service identifies
// the external system ("github", "jira"), and Key identifies the credential
// type within that service ("token", "username").
type Credential struct {
	ID        int64
	Service   string
	Key       string
	Value     string
	UpdatedAt time.Time
}
