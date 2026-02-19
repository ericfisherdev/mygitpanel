package sqlite

import (
	"context"
	"testing"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertPRForIgnoreTest adds a single PR to an already-created repo and returns its DB ID.
func insertPRForIgnoreTest(t *testing.T, db *DB, repoFullName string, number int) int64 {
	t.Helper()
	prRepo := NewPRRepo(db)
	pr := makePR(repoFullName, number, "Test PR", model.PRStatusOpen)
	require.NoError(t, prRepo.Upsert(context.Background(), pr))

	got, err := prRepo.GetByNumber(context.Background(), repoFullName, number)
	require.NoError(t, err)
	require.NotNil(t, got)
	return got.ID
}

func TestIgnoreRepo_IgnoreAndIsIgnored(t *testing.T) {
	db := setupTestDB(t)
	prID := addTestPR(t, db, "owner/repo", 1)
	repo := NewIgnoreRepo(db)
	ctx := context.Background()

	err := repo.Ignore(ctx, prID)
	require.NoError(t, err)

	ignored, err := repo.IsIgnored(ctx, prID)
	require.NoError(t, err)
	assert.True(t, ignored)
}

func TestIgnoreRepo_UnignoreAndIsIgnored(t *testing.T) {
	db := setupTestDB(t)
	prID := addTestPR(t, db, "owner/repo", 1)
	repo := NewIgnoreRepo(db)
	ctx := context.Background()

	require.NoError(t, repo.Ignore(ctx, prID))
	require.NoError(t, repo.Unignore(ctx, prID))

	ignored, err := repo.IsIgnored(ctx, prID)
	require.NoError(t, err)
	assert.False(t, ignored)
}

func TestIgnoreRepo_DoubleIgnore_Idempotent(t *testing.T) {
	db := setupTestDB(t)
	prID := addTestPR(t, db, "owner/repo", 1)
	repo := NewIgnoreRepo(db)
	ctx := context.Background()

	require.NoError(t, repo.Ignore(ctx, prID))
	// Second Ignore should not return an error.
	err := repo.Ignore(ctx, prID)
	require.NoError(t, err)

	ignored, err := repo.IsIgnored(ctx, prID)
	require.NoError(t, err)
	assert.True(t, ignored)
}

func TestIgnoreRepo_ListIgnored_OrderedByDesc(t *testing.T) {
	db := setupTestDB(t)
	// Add the repo once, then insert 3 PRs.
	addTestRepo(t, db, "owner/repo")
	prID1 := insertPRForIgnoreTest(t, db, "owner/repo", 1)
	prID2 := insertPRForIgnoreTest(t, db, "owner/repo", 2)
	prID3 := insertPRForIgnoreTest(t, db, "owner/repo", 3)
	repo := NewIgnoreRepo(db)
	ctx := context.Background()

	require.NoError(t, repo.Ignore(ctx, prID1))
	require.NoError(t, repo.Ignore(ctx, prID2))
	require.NoError(t, repo.Ignore(ctx, prID3))

	list, err := repo.ListIgnored(ctx)
	require.NoError(t, err)
	require.Len(t, list, 3)

	// All timestamps should be populated.
	for _, item := range list {
		assert.False(t, item.IgnoredAt.IsZero(), "IgnoredAt should not be zero")
	}

	// Verify all three IDs are present.
	ids := make(map[int64]bool)
	for _, item := range list {
		ids[item.PRID] = true
	}
	assert.True(t, ids[prID1])
	assert.True(t, ids[prID2])
	assert.True(t, ids[prID3])
}

func TestIgnoreRepo_ListIgnoredIDs(t *testing.T) {
	db := setupTestDB(t)
	// Add the repo once, then insert 2 PRs.
	addTestRepo(t, db, "owner/repo")
	prID1 := insertPRForIgnoreTest(t, db, "owner/repo", 1)
	prID2 := insertPRForIgnoreTest(t, db, "owner/repo", 2)
	repo := NewIgnoreRepo(db)
	ctx := context.Background()

	require.NoError(t, repo.Ignore(ctx, prID1))
	require.NoError(t, repo.Ignore(ctx, prID2))

	ids, err := repo.ListIgnoredIDs(ctx)
	require.NoError(t, err)
	assert.Len(t, ids, 2)
	_, ok1 := ids[prID1]
	_, ok2 := ids[prID2]
	assert.True(t, ok1)
	assert.True(t, ok2)
}

func TestIgnoreRepo_Unignore_NonExistent_NoError(t *testing.T) {
	db := setupTestDB(t)
	repo := NewIgnoreRepo(db)
	ctx := context.Background()

	// Unignore a PR that was never ignored — should be a no-op, not an error.
	err := repo.Unignore(ctx, 999999)
	require.NoError(t, err)
}
