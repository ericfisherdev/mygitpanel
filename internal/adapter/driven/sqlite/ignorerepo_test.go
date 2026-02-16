package sqlite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIgnoreRepo_IgnoreAndIsIgnored(t *testing.T) {
	db := setupTestDB(t)
	repo := NewIgnoreRepo(db)
	ctx := context.Background()

	ignored, err := repo.IsIgnored(ctx, "owner/repo", 42)
	require.NoError(t, err)
	assert.False(t, ignored)

	err = repo.Ignore(ctx, "owner/repo", 42)
	require.NoError(t, err)

	ignored, err = repo.IsIgnored(ctx, "owner/repo", 42)
	require.NoError(t, err)
	assert.True(t, ignored)
}

func TestIgnoreRepo_DoubleIgnoreIdempotent(t *testing.T) {
	db := setupTestDB(t)
	repo := NewIgnoreRepo(db)
	ctx := context.Background()

	err := repo.Ignore(ctx, "owner/repo", 42)
	require.NoError(t, err)

	err = repo.Ignore(ctx, "owner/repo", 42)
	assert.NoError(t, err, "ignoring an already-ignored PR should be idempotent")
}

func TestIgnoreRepo_Unignore(t *testing.T) {
	db := setupTestDB(t)
	repo := NewIgnoreRepo(db)
	ctx := context.Background()

	err := repo.Ignore(ctx, "owner/repo", 42)
	require.NoError(t, err)

	err = repo.Unignore(ctx, "owner/repo", 42)
	require.NoError(t, err)

	ignored, err := repo.IsIgnored(ctx, "owner/repo", 42)
	require.NoError(t, err)
	assert.False(t, ignored)
}

func TestIgnoreRepo_UnignoreNonexistent(t *testing.T) {
	db := setupTestDB(t)
	repo := NewIgnoreRepo(db)
	ctx := context.Background()

	err := repo.Unignore(ctx, "owner/repo", 999)
	assert.NoError(t, err, "unignoring a non-ignored PR should not error")
}

func TestIgnoreRepo_ListIgnored(t *testing.T) {
	db := setupTestDB(t)
	repo := NewIgnoreRepo(db)
	ctx := context.Background()

	err := repo.Ignore(ctx, "owner/repo-a", 1)
	require.NoError(t, err)

	err = repo.Ignore(ctx, "owner/repo-b", 2)
	require.NoError(t, err)

	items, err := repo.ListIgnored(ctx)
	require.NoError(t, err)
	require.Len(t, items, 2)

	// Most recently ignored first.
	assert.Equal(t, "owner/repo-b", items[0].RepoFullName)
	assert.Equal(t, 2, items[0].PRNumber)
	assert.Equal(t, "owner/repo-a", items[1].RepoFullName)
	assert.Equal(t, 1, items[1].PRNumber)
}

func TestIgnoreRepo_ListIgnoredEmpty(t *testing.T) {
	db := setupTestDB(t)
	repo := NewIgnoreRepo(db)
	ctx := context.Background()

	items, err := repo.ListIgnored(ctx)
	require.NoError(t, err)
	assert.Empty(t, items)
}
