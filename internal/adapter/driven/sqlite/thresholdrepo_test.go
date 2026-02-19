package sqlite

import (
	"context"
	"testing"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThresholdRepo_GetGlobalSettings_Defaults(t *testing.T) {
	db := setupTestDB(t)
	repo := NewThresholdRepo(db)
	ctx := context.Background()

	// After migration, global_settings is pre-seeded with defaults.
	settings, err := repo.GetGlobalSettings(ctx)
	require.NoError(t, err)

	defaults := model.DefaultGlobalSettings()
	assert.Equal(t, defaults.ReviewCountThreshold, settings.ReviewCountThreshold)
	assert.Equal(t, defaults.AgeUrgencyDays, settings.AgeUrgencyDays)
	assert.Equal(t, defaults.StaleReviewEnabled, settings.StaleReviewEnabled)
	assert.Equal(t, defaults.CIFailureEnabled, settings.CIFailureEnabled)
}

func TestThresholdRepo_SetAndGetGlobalSettings(t *testing.T) {
	db := setupTestDB(t)
	repo := NewThresholdRepo(db)
	ctx := context.Background()

	want := model.GlobalSettings{
		ReviewCountThreshold: 3,
		AgeUrgencyDays:       14,
		StaleReviewEnabled:   false,
		CIFailureEnabled:     true,
	}

	err := repo.SetGlobalSettings(ctx, want)
	require.NoError(t, err)

	got, err := repo.GetGlobalSettings(ctx)
	require.NoError(t, err)
	assert.Equal(t, want.ReviewCountThreshold, got.ReviewCountThreshold)
	assert.Equal(t, want.AgeUrgencyDays, got.AgeUrgencyDays)
	assert.Equal(t, want.StaleReviewEnabled, got.StaleReviewEnabled)
	assert.Equal(t, want.CIFailureEnabled, got.CIFailureEnabled)
}

func TestThresholdRepo_GetRepoThreshold_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewThresholdRepo(db)
	ctx := context.Background()

	threshold, err := repo.GetRepoThreshold(ctx, "owner/nonexistent")
	require.NoError(t, err)

	// All nil pointers when no override exists.
	assert.Nil(t, threshold.ReviewCount)
	assert.Nil(t, threshold.AgeUrgencyDays)
	assert.Nil(t, threshold.StaleReviewEnabled)
	assert.Nil(t, threshold.CIFailureEnabled)
}

func TestThresholdRepo_SetAndGetRepoThreshold(t *testing.T) {
	db := setupTestDB(t)
	addTestRepo(t, db, "owner/repo")
	repo := NewThresholdRepo(db)
	ctx := context.Background()

	reviewCount := 2
	ageUrgency := 5
	staleEnabled := false
	ciEnabled := true

	want := model.RepoThreshold{
		RepoFullName:       "owner/repo",
		ReviewCount:        &reviewCount,
		AgeUrgencyDays:     &ageUrgency,
		StaleReviewEnabled: &staleEnabled,
		CIFailureEnabled:   &ciEnabled,
	}

	err := repo.SetRepoThreshold(ctx, want)
	require.NoError(t, err)

	got, err := repo.GetRepoThreshold(ctx, "owner/repo")
	require.NoError(t, err)
	require.NotNil(t, got.ReviewCount)
	require.NotNil(t, got.AgeUrgencyDays)
	require.NotNil(t, got.StaleReviewEnabled)
	require.NotNil(t, got.CIFailureEnabled)
	assert.Equal(t, reviewCount, *got.ReviewCount)
	assert.Equal(t, ageUrgency, *got.AgeUrgencyDays)
	assert.Equal(t, staleEnabled, *got.StaleReviewEnabled)
	assert.Equal(t, ciEnabled, *got.CIFailureEnabled)
}

func TestThresholdRepo_SetRepoThreshold_NilFields(t *testing.T) {
	db := setupTestDB(t)
	addTestRepo(t, db, "owner/repo")
	repo := NewThresholdRepo(db)
	ctx := context.Background()

	// Set with only some fields set (nil means use global).
	reviewCount := 2
	want := model.RepoThreshold{
		RepoFullName: "owner/repo",
		ReviewCount:  &reviewCount,
		// AgeUrgencyDays, StaleReviewEnabled, CIFailureEnabled remain nil.
	}

	err := repo.SetRepoThreshold(ctx, want)
	require.NoError(t, err)

	got, err := repo.GetRepoThreshold(ctx, "owner/repo")
	require.NoError(t, err)
	require.NotNil(t, got.ReviewCount)
	assert.Equal(t, reviewCount, *got.ReviewCount)
	assert.Nil(t, got.AgeUrgencyDays)
	assert.Nil(t, got.StaleReviewEnabled)
	assert.Nil(t, got.CIFailureEnabled)
}

func TestThresholdRepo_DeleteRepoThreshold(t *testing.T) {
	db := setupTestDB(t)
	addTestRepo(t, db, "owner/repo")
	repo := NewThresholdRepo(db)
	ctx := context.Background()

	reviewCount := 5
	require.NoError(t, repo.SetRepoThreshold(ctx, model.RepoThreshold{
		RepoFullName: "owner/repo",
		ReviewCount:  &reviewCount,
	}))

	err := repo.DeleteRepoThreshold(ctx, "owner/repo")
	require.NoError(t, err)

	got, err := repo.GetRepoThreshold(ctx, "owner/repo")
	require.NoError(t, err)
	assert.Nil(t, got.ReviewCount)
}
