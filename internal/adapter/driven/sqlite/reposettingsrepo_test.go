package sqlite

import (
	"context"
	"testing"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoSettingsRepo_GetSettingsMissing(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepoSettingsRepo(db)
	ctx := context.Background()

	settings, err := repo.GetSettings(ctx, "owner/nonexistent")
	require.NoError(t, err)
	assert.Nil(t, settings)
}

func TestRepoSettingsRepo_SetAndGetSettings(t *testing.T) {
	db := setupTestDB(t)
	settingsRepo := NewRepoSettingsRepo(db)
	repoRepo := NewRepoRepo(db)
	ctx := context.Background()

	// Create the parent repository first (foreign key constraint).
	err := repoRepo.Add(ctx, model.Repository{FullName: "owner/repo", Owner: "owner", Name: "repo"})
	require.NoError(t, err)

	err = settingsRepo.SetSettings(ctx, model.RepoSettings{
		RepoFullName:        "owner/repo",
		RequiredReviewCount: 3,
		UrgencyDays:         5,
	})
	require.NoError(t, err)

	settings, err := settingsRepo.GetSettings(ctx, "owner/repo")
	require.NoError(t, err)
	require.NotNil(t, settings)
	assert.Equal(t, "owner/repo", settings.RepoFullName)
	assert.Equal(t, 3, settings.RequiredReviewCount)
	assert.Equal(t, 5, settings.UrgencyDays)
}

func TestRepoSettingsRepo_UpsertOverwrites(t *testing.T) {
	db := setupTestDB(t)
	settingsRepo := NewRepoSettingsRepo(db)
	repoRepo := NewRepoRepo(db)
	ctx := context.Background()

	err := repoRepo.Add(ctx, model.Repository{FullName: "owner/repo", Owner: "owner", Name: "repo"})
	require.NoError(t, err)

	err = settingsRepo.SetSettings(ctx, model.RepoSettings{
		RepoFullName:        "owner/repo",
		RequiredReviewCount: 2,
		UrgencyDays:         7,
	})
	require.NoError(t, err)

	err = settingsRepo.SetSettings(ctx, model.RepoSettings{
		RepoFullName:        "owner/repo",
		RequiredReviewCount: 4,
		UrgencyDays:         3,
	})
	require.NoError(t, err)

	settings, err := settingsRepo.GetSettings(ctx, "owner/repo")
	require.NoError(t, err)
	require.NotNil(t, settings)
	assert.Equal(t, 4, settings.RequiredReviewCount)
	assert.Equal(t, 3, settings.UrgencyDays)
}
