package sqlite

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB provides dual reader/writer database connections with WAL mode enabled.
// The writer connection is limited to a single connection to avoid "database is locked" errors.
// The reader connection pool allows up to 4 concurrent readers.
type DB struct {
	Writer *sql.DB
	Reader *sql.DB
	path   string
}

// NewDB creates a new dual-connection SQLite database with WAL mode, busy timeout,
// synchronous NORMAL, foreign keys enabled, and a 64MB cache.
func NewDB(dbPath string) (*DB, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=cache_size(-64000)",
		dbPath,
	)

	writer, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open writer: %w", err)
	}
	writer.SetMaxOpenConns(1)

	if err := writer.Ping(); err != nil {
		writer.Close()
		return nil, fmt.Errorf("ping writer: %w", err)
	}

	reader, err := sql.Open("sqlite", dsn)
	if err != nil {
		writer.Close()
		return nil, fmt.Errorf("open reader: %w", err)
	}
	reader.SetMaxOpenConns(4)

	if err := reader.Ping(); err != nil {
		reader.Close()
		writer.Close()
		return nil, fmt.Errorf("ping reader: %w", err)
	}

	return &DB{
		Writer: writer,
		Reader: reader,
		path:   dbPath,
	}, nil
}

// Close closes both reader and writer connections. Returns the first error encountered.
func (db *DB) Close() error {
	var firstErr error

	if err := db.Reader.Close(); err != nil {
		firstErr = fmt.Errorf("close reader: %w", err)
	}

	if err := db.Writer.Close(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("close writer: %w", err)
	}

	return firstErr
}
