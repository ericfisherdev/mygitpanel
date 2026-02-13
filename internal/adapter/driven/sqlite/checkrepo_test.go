package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertTestPR creates a PR and returns its database ID for use in check run tests.
func insertTestPR(t *testing.T, db *DB, repoFullName string, number int) int64 {
	t.Helper()
	addTestRepo(t, db, repoFullName)
	prRepo := NewPRRepo(db)
	ctx := context.Background()

	pr := makePR(repoFullName, number, "Test PR", model.PRStatusOpen)
	require.NoError(t, prRepo.Upsert(ctx, pr))

	got, err := prRepo.GetByNumber(ctx, repoFullName, number)
	require.NoError(t, err)
	require.NotNil(t, got)

	return got.ID
}

func TestCheckRepo_ReplaceAndGet(t *testing.T) {
	db := setupTestDB(t)
	prID := insertTestPR(t, db, "octocat/hello-world", 1)
	checkRepo := NewCheckRepo(db)
	ctx := context.Background()

	started := time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC)
	completed := time.Date(2026, 2, 10, 10, 5, 0, 0, time.UTC)

	runs := []model.CheckRun{
		{
			ID:          1001,
			PRID:        prID,
			Name:        "build",
			Status:      "completed",
			Conclusion:  "success",
			IsRequired:  true,
			DetailsURL:  "https://github.com/octocat/hello-world/runs/1001",
			StartedAt:   started,
			CompletedAt: completed,
		},
		{
			ID:          1002,
			PRID:        prID,
			Name:        "lint",
			Status:      "completed",
			Conclusion:  "failure",
			IsRequired:  false,
			DetailsURL:  "https://github.com/octocat/hello-world/runs/1002",
			StartedAt:   started,
			CompletedAt: completed,
		},
	}

	require.NoError(t, checkRepo.ReplaceCheckRunsForPR(ctx, prID, runs))

	got, err := checkRepo.GetCheckRunsByPR(ctx, prID)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Results are ordered by name, so "build" comes first.
	assert.Equal(t, int64(1001), got[0].ID)
	assert.Equal(t, "build", got[0].Name)
	assert.Equal(t, "completed", got[0].Status)
	assert.Equal(t, "success", got[0].Conclusion)
	assert.True(t, got[0].IsRequired)
	assert.Equal(t, "https://github.com/octocat/hello-world/runs/1001", got[0].DetailsURL)
	assert.Equal(t, started, got[0].StartedAt)
	assert.Equal(t, completed, got[0].CompletedAt)

	assert.Equal(t, int64(1002), got[1].ID)
	assert.Equal(t, "lint", got[1].Name)
	assert.Equal(t, "failure", got[1].Conclusion)
	assert.False(t, got[1].IsRequired)

	// Replace with a single different check run -- old ones should be deleted.
	replacement := []model.CheckRun{
		{
			ID:         2001,
			PRID:       prID,
			Name:       "test",
			Status:     "in_progress",
			Conclusion: "",
			IsRequired: true,
			DetailsURL: "https://github.com/octocat/hello-world/runs/2001",
			StartedAt:  started,
		},
	}

	require.NoError(t, checkRepo.ReplaceCheckRunsForPR(ctx, prID, replacement))

	got, err = checkRepo.GetCheckRunsByPR(ctx, prID)
	require.NoError(t, err)
	require.Len(t, got, 1)

	assert.Equal(t, int64(2001), got[0].ID)
	assert.Equal(t, "test", got[0].Name)
	assert.Equal(t, "in_progress", got[0].Status)
	assert.Equal(t, "", got[0].Conclusion)
	assert.True(t, got[0].IsRequired)
	assert.True(t, got[0].CompletedAt.IsZero(), "completed_at should be zero for in-progress run")
}

func TestCheckRepo_GetCheckRunsByPR_Empty(t *testing.T) {
	db := setupTestDB(t)
	prID := insertTestPR(t, db, "octocat/hello-world", 1)
	checkRepo := NewCheckRepo(db)
	ctx := context.Background()

	got, err := checkRepo.GetCheckRunsByPR(ctx, prID)
	require.NoError(t, err)
	assert.Nil(t, got, "no check runs should return nil slice")
}

func TestCheckRepo_ReplaceWithEmpty(t *testing.T) {
	db := setupTestDB(t)
	prID := insertTestPR(t, db, "octocat/hello-world", 1)
	checkRepo := NewCheckRepo(db)
	ctx := context.Background()

	started := time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC)

	// Insert some check runs first.
	runs := []model.CheckRun{
		{
			ID:        1001,
			PRID:      prID,
			Name:      "build",
			Status:    "completed",
			StartedAt: started,
		},
	}
	require.NoError(t, checkRepo.ReplaceCheckRunsForPR(ctx, prID, runs))

	// Replace with empty slice -- should delete all existing.
	require.NoError(t, checkRepo.ReplaceCheckRunsForPR(ctx, prID, []model.CheckRun{}))

	got, err := checkRepo.GetCheckRunsByPR(ctx, prID)
	require.NoError(t, err)
	assert.Nil(t, got, "replacing with empty slice should remove all check runs")
}
