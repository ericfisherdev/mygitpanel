package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeRepo(fullName, owner, name string) model.Repository {
	return model.Repository{
		FullName: fullName,
		Owner:    owner,
		Name:     name,
		AddedAt:  time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
	}
}

func TestRepoRepo_Add(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepoRepo(db)
	ctx := context.Background()

	err := repo.Add(ctx, makeRepo("octocat/hello-world", "octocat", "hello-world"))
	require.NoError(t, err)

	got, err := repo.GetByFullName(ctx, "octocat/hello-world")
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, "octocat/hello-world", got.FullName)
	assert.Equal(t, "octocat", got.Owner)
	assert.Equal(t, "hello-world", got.Name)
	assert.False(t, got.AddedAt.IsZero())
}

func TestRepoRepo_Add_Duplicate(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepoRepo(db)
	ctx := context.Background()

	r := makeRepo("octocat/hello-world", "octocat", "hello-world")
	require.NoError(t, repo.Add(ctx, r))

	err := repo.Add(ctx, r)
	assert.Error(t, err, "adding duplicate repository should fail")
}

func TestRepoRepo_Remove(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepoRepo(db)
	ctx := context.Background()

	require.NoError(t, repo.Add(ctx, makeRepo("octocat/hello-world", "octocat", "hello-world")))

	err := repo.Remove(ctx, "octocat/hello-world")
	require.NoError(t, err)

	all, err := repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, all)
}

func TestRepoRepo_Remove_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepoRepo(db)
	ctx := context.Background()

	err := repo.Remove(ctx, "nonexistent/repo")
	assert.Error(t, err, "removing non-existent repo should fail")
}

func TestRepoRepo_ListAll(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepoRepo(db)
	ctx := context.Background()

	require.NoError(t, repo.Add(ctx, makeRepo("charlie/zeta", "charlie", "zeta")))
	require.NoError(t, repo.Add(ctx, makeRepo("alice/alpha", "alice", "alpha")))
	require.NoError(t, repo.Add(ctx, makeRepo("bob/beta", "bob", "beta")))

	all, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, all, 3)

	// Ordered by full_name
	assert.Equal(t, "alice/alpha", all[0].FullName)
	assert.Equal(t, "bob/beta", all[1].FullName)
	assert.Equal(t, "charlie/zeta", all[2].FullName)
}

func TestRepoRepo_GetByFullName_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepoRepo(db)
	ctx := context.Background()

	got, err := repo.GetByFullName(ctx, "nonexistent/repo")
	require.NoError(t, err)
	assert.Nil(t, got, "non-existent repo should return nil without error")
}
