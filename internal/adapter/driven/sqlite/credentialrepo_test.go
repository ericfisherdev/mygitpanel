package sqlite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCredentialRepo_SetAndGet(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCredentialRepo(db)
	ctx := context.Background()

	err := repo.Set(ctx, "github", "token", "ghp_abc123")
	require.NoError(t, err)

	val, err := repo.Get(ctx, "github", "token")
	require.NoError(t, err)
	assert.Equal(t, "ghp_abc123", val)
}

func TestCredentialRepo_GetMissing(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCredentialRepo(db)
	ctx := context.Background()

	val, err := repo.Get(ctx, "github", "nonexistent")
	require.NoError(t, err)
	assert.Equal(t, "", val)
}

func TestCredentialRepo_UpsertOverwrites(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCredentialRepo(db)
	ctx := context.Background()

	err := repo.Set(ctx, "github", "token", "old-value")
	require.NoError(t, err)

	err = repo.Set(ctx, "github", "token", "new-value")
	require.NoError(t, err)

	val, err := repo.Get(ctx, "github", "token")
	require.NoError(t, err)
	assert.Equal(t, "new-value", val)
}

func TestCredentialRepo_GetAll(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCredentialRepo(db)
	ctx := context.Background()

	err := repo.Set(ctx, "github", "token", "ghp_abc")
	require.NoError(t, err)

	err = repo.Set(ctx, "github", "username", "testuser")
	require.NoError(t, err)

	creds, err := repo.GetAll(ctx, "github")
	require.NoError(t, err)
	assert.Len(t, creds, 2)
	assert.Equal(t, "ghp_abc", creds["token"])
	assert.Equal(t, "testuser", creds["username"])
}

func TestCredentialRepo_GetAllEmpty(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCredentialRepo(db)
	ctx := context.Background()

	creds, err := repo.GetAll(ctx, "jira")
	require.NoError(t, err)
	assert.Empty(t, creds)
}

func TestCredentialRepo_Delete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCredentialRepo(db)
	ctx := context.Background()

	err := repo.Set(ctx, "github", "token", "ghp_abc")
	require.NoError(t, err)

	err = repo.Delete(ctx, "github", "token")
	require.NoError(t, err)

	val, err := repo.Get(ctx, "github", "token")
	require.NoError(t, err)
	assert.Equal(t, "", val)
}

func TestCredentialRepo_DeleteNonexistent(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCredentialRepo(db)
	ctx := context.Background()

	err := repo.Delete(ctx, "github", "nonexistent")
	assert.NoError(t, err, "deleting nonexistent credential should not error")
}
