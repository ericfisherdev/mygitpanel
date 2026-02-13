package sqlite

import (
	"context"
	"path/filepath"
	"testing"
)

// setupTestDB creates a temporary SQLite database for testing. The database
// is automatically closed and removed when the test completes.
func setupTestDB(t *testing.T) *DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")

	db, err := NewDB(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("create test db: %v", err)
	}

	if err := RunMigrations(db.Writer); err != nil {
		db.Close()
		t.Fatalf("run migrations: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}
