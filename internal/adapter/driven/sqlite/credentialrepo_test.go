package sqlite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testKey returns a 32-byte AES-256 key for testing.
func testKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return key
}

func TestCredentialRepo_SetAndGet(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCredentialRepo(db, testKey())
	ctx := context.Background()

	err := repo.Set(ctx, "github_token", "ghp_supersecret")
	require.NoError(t, err)

	got, err := repo.Get(ctx, "github_token")
	require.NoError(t, err)
	assert.Equal(t, "ghp_supersecret", got)
}

func TestCredentialRepo_Get_UnknownService(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCredentialRepo(db, testKey())
	ctx := context.Background()

	got, err := repo.Get(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestCredentialRepo_Set_Overwrite(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCredentialRepo(db, testKey())
	ctx := context.Background()

	require.NoError(t, repo.Set(ctx, "github_token", "old_token"))
	require.NoError(t, repo.Set(ctx, "github_token", "new_token"))

	got, err := repo.Get(ctx, "github_token")
	require.NoError(t, err)
	assert.Equal(t, "new_token", got)
}

func TestCredentialRepo_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCredentialRepo(db, testKey())
	ctx := context.Background()

	require.NoError(t, repo.Set(ctx, "github_token", "ghp_abc"))
	require.NoError(t, repo.Set(ctx, "jira_token", "jira_xyz"))

	creds, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, creds, 2)

	services := []string{creds[0].Service, creds[1].Service}
	assert.Contains(t, services, "github_token")
	assert.Contains(t, services, "jira_token")

	for _, c := range creds {
		assert.NotEmpty(t, c.Service)
		assert.NotEmpty(t, c.Value)
		assert.False(t, c.UpdatedAt.IsZero())
	}
}

func TestCredentialRepo_Delete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCredentialRepo(db, testKey())
	ctx := context.Background()

	require.NoError(t, repo.Set(ctx, "github_token", "ghp_secret"))

	err := repo.Delete(ctx, "github_token")
	require.NoError(t, err)

	got, err := repo.Get(ctx, "github_token")
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestCredentialRepo_Set_NilKey(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCredentialRepo(db, nil)
	ctx := context.Background()

	err := repo.Set(ctx, "github_token", "value")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEncryptionKeyNotSet)
}

func TestCredentialRepo_Get_NilKey(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCredentialRepo(db, nil)
	ctx := context.Background()

	_, err := repo.Get(ctx, "github_token")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEncryptionKeyNotSet)
}

func TestCredentialRepo_List_NilKey(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCredentialRepo(db, nil)
	ctx := context.Background()

	_, err := repo.List(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEncryptionKeyNotSet)
}
