package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"testing"
)

// setupTestDB creates a named shared in-memory SQLite database for testing.
// Writer and reader connections share the same in-memory database via cache=shared.
// A unique name derived from t.Name() ensures isolation between parallel tests.
func setupTestDB(t *testing.T) *DB {
	t.Helper()

	// Percent-encode the test name so it's a safe SQLite URI filename component
	// and cannot be misinterpreted as query parameters in the "file:%s?..." DSN.
	safeName := url.PathEscape(t.Name())
	// WAL mode is not applicable to in-memory databases; omit journal_mode pragma.
	dsn := fmt.Sprintf(
		"file:%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=cache_size(-64000)",
		safeName,
	)

	writer, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("create test db writer: %v", err)
	}
	writer.SetMaxOpenConns(1)
	if err := writer.PingContext(context.Background()); err != nil {
		_ = writer.Close()
		t.Fatalf("ping test db writer: %v", err)
	}

	reader, err := sql.Open("sqlite", dsn)
	if err != nil {
		_ = writer.Close()
		t.Fatalf("create test db reader: %v", err)
	}
	reader.SetMaxOpenConns(4)
	if err := reader.PingContext(context.Background()); err != nil {
		_ = reader.Close()
		_ = writer.Close()
		t.Fatalf("ping test db reader: %v", err)
	}

	db := &DB{Writer: writer, Reader: reader, path: dsn}

	if err := RunMigrations(db.Writer); err != nil {
		_ = db.Close()
		t.Fatalf("run migrations: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	return db
}
