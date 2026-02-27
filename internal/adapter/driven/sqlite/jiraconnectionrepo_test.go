package sqlite

import (
	"context"
	"testing"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test fixture constants for JiraConnectionRepo tests.
const (
	testJiraToken    = "secret-api-token"
	testJiraRepoName = "org/repo"
)

func newTestJiraConn(name, url string) model.JiraConnection {
	return model.JiraConnection{
		DisplayName: name,
		BaseURL:     url,
		Email:       "user@example.com",
		Token:       testJiraToken,
	}
}

func TestJiraConnectionRepo_CreateAndList(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJiraConnectionRepo(db, testKey())
	ctx := context.Background()

	conn := newTestJiraConn("My Jira", "https://myco.atlassian.net")
	id, err := repo.Create(ctx, conn)
	require.NoError(t, err)
	assert.Greater(t, id, int64(0))

	conns, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, conns, 1)

	got := conns[0]
	assert.Equal(t, id, got.ID)
	assert.Equal(t, "My Jira", got.DisplayName)
	assert.Equal(t, "https://myco.atlassian.net", got.BaseURL)
	assert.Equal(t, "user@example.com", got.Email)
	assert.Equal(t, "secret-api-token", got.Token, "token should be decrypted on read")
	assert.False(t, got.IsDefault)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
}

func TestJiraConnectionRepo_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJiraConnectionRepo(db, testKey())
	ctx := context.Background()

	got, err := repo.GetByID(ctx, 9999)
	require.NoError(t, err)
	assert.Equal(t, int64(0), got.ID, "should return zero-value connection for unknown ID")
}

func TestJiraConnectionRepo_GetByID_Found(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJiraConnectionRepo(db, testKey())
	ctx := context.Background()

	id, err := repo.Create(ctx, newTestJiraConn("Test", "https://test.atlassian.net"))
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, id, got.ID)
	assert.Equal(t, "Test", got.DisplayName)
	assert.Equal(t, testJiraToken, got.Token)
}

func TestJiraConnectionRepo_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJiraConnectionRepo(db, testKey())
	ctx := context.Background()

	id, err := repo.Create(ctx, newTestJiraConn("Original", "https://orig.atlassian.net"))
	require.NoError(t, err)

	err = repo.Update(ctx, model.JiraConnection{
		ID:          id,
		DisplayName: "Updated",
		BaseURL:     "https://updated.atlassian.net",
		Email:       "new@example.com",
		Token:       "new-token",
	})
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "Updated", got.DisplayName)
	assert.Equal(t, "https://updated.atlassian.net", got.BaseURL)
	assert.Equal(t, "new@example.com", got.Email)
	assert.Equal(t, "new-token", got.Token)
}

func TestJiraConnectionRepo_GetForRepo_DefaultFallback(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJiraConnectionRepo(db, testKey())
	ctx := context.Background()

	// Insert a repo so FK is satisfied.
	_, err := db.Writer.ExecContext(ctx, `INSERT INTO repositories (full_name, owner, name) VALUES ('org/repo', 'org', 'repo')`)
	require.NoError(t, err)

	// Create a default connection.
	conn := newTestJiraConn("Default Jira", "https://default.atlassian.net")
	conn.IsDefault = true
	id, err := repo.Create(ctx, conn)
	require.NoError(t, err)

	// No explicit mapping -- should fall back to default.
	got, err := repo.GetForRepo(ctx, testJiraRepoName)
	require.NoError(t, err)
	assert.Equal(t, id, got.ID)
	assert.Equal(t, "Default Jira", got.DisplayName)
}

func TestJiraConnectionRepo_GetForRepo_ExplicitMapping(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJiraConnectionRepo(db, testKey())
	ctx := context.Background()

	// Insert a repo so FK is satisfied.
	_, err := db.Writer.ExecContext(ctx, `INSERT INTO repositories (full_name, owner, name) VALUES ('org/repo', 'org', 'repo')`)
	require.NoError(t, err)

	// Create default connection.
	defaultConn := newTestJiraConn("Default", "https://default.atlassian.net")
	defaultConn.IsDefault = true
	_, err = repo.Create(ctx, defaultConn)
	require.NoError(t, err)

	// Create non-default connection.
	specificConn := newTestJiraConn("Specific", "https://specific.atlassian.net")
	specificID, err := repo.Create(ctx, specificConn)
	require.NoError(t, err)

	// Map repo to specific connection.
	err = repo.SetRepoMapping(ctx, testJiraRepoName, specificID)
	require.NoError(t, err)

	got, err := repo.GetForRepo(ctx, testJiraRepoName)
	require.NoError(t, err)
	assert.Equal(t, specificID, got.ID)
	assert.Equal(t, "Specific", got.DisplayName)
}

func TestJiraConnectionRepo_GetForRepo_NoConnection(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJiraConnectionRepo(db, testKey())
	ctx := context.Background()

	got, err := repo.GetForRepo(ctx, "org/norepo")
	require.NoError(t, err)
	assert.Equal(t, int64(0), got.ID, "should return zero-value when no connection applies")
}

func TestJiraConnectionRepo_SetDefault_AtomicSwitch(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJiraConnectionRepo(db, testKey())
	ctx := context.Background()

	conn1 := newTestJiraConn("Jira A", "https://a.atlassian.net")
	conn1.IsDefault = true
	id1, err := repo.Create(ctx, conn1)
	require.NoError(t, err)

	conn2 := newTestJiraConn("Jira B", "https://b.atlassian.net")
	id2, err := repo.Create(ctx, conn2)
	require.NoError(t, err)

	// Switch default to conn2.
	err = repo.SetDefault(ctx, id2)
	require.NoError(t, err)

	got1, err := repo.GetByID(ctx, id1)
	require.NoError(t, err)
	assert.False(t, got1.IsDefault, "old default should be cleared")

	got2, err := repo.GetByID(ctx, id2)
	require.NoError(t, err)
	assert.True(t, got2.IsDefault, "new default should be set")
}

func TestJiraConnectionRepo_SetDefault_ClearAll(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJiraConnectionRepo(db, testKey())
	ctx := context.Background()

	conn := newTestJiraConn("Jira", "https://jira.atlassian.net")
	conn.IsDefault = true
	id, err := repo.Create(ctx, conn)
	require.NoError(t, err)

	// Clear default by passing 0.
	err = repo.SetDefault(ctx, 0)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, id)
	require.NoError(t, err)
	assert.False(t, got.IsDefault, "default should be cleared")
}

func TestJiraConnectionRepo_Delete_CascadesMapping(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJiraConnectionRepo(db, testKey())
	ctx := context.Background()

	// Insert a repo so FK is satisfied.
	_, err := db.Writer.ExecContext(ctx, `INSERT INTO repositories (full_name, owner, name) VALUES ('org/repo', 'org', 'repo')`)
	require.NoError(t, err)

	id, err := repo.Create(ctx, newTestJiraConn("ToDelete", "https://delete.atlassian.net"))
	require.NoError(t, err)

	err = repo.SetRepoMapping(ctx, testJiraRepoName, id)
	require.NoError(t, err)

	// Delete the connection -- mapping should be cleaned up via ON DELETE SET NULL.
	err = repo.Delete(ctx, id)
	require.NoError(t, err)

	conns, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, conns)

	// Mapping row should still exist but with NULL connection_id (ON DELETE SET NULL).
	var count int
	err = db.Reader.QueryRowContext(ctx, `SELECT COUNT(*) FROM repo_jira_mapping WHERE repo_full_name = 'org/repo' AND jira_connection_id IS NULL`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "mapping row should have NULL connection_id after cascade")
}

func TestJiraConnectionRepo_NilKey(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJiraConnectionRepo(db, nil)
	ctx := context.Background()

	_, err := repo.Create(ctx, newTestJiraConn("X", "https://x.atlassian.net"))
	require.ErrorIs(t, err, ErrEncryptionKeyNotSet)

	_, err = repo.List(ctx)
	require.ErrorIs(t, err, ErrEncryptionKeyNotSet)

	_, err = repo.GetByID(ctx, 1)
	require.ErrorIs(t, err, ErrEncryptionKeyNotSet)

	_, err = repo.GetForRepo(ctx, testJiraRepoName)
	require.ErrorIs(t, err, ErrEncryptionKeyNotSet)
}
