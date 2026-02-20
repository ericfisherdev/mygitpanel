package model

import "time"

// Credential represents an encrypted credential stored by service name.
// The Value field holds the plaintext at the domain boundary — the SQLite adapter
// is responsible for encrypting before write and decrypting after read.
type Credential struct {
	ID        int64
	Service   string
	Value     string
	UpdatedAt time.Time
}
