package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/efisher/reviewhub/internal/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBotConfigRepo_SeededDefaults(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBotConfigRepo(db)
	ctx := context.Background()

	configs, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, configs, 3)

	// Ordered alphabetically by username
	assert.Equal(t, "coderabbitai", configs[0].Username)
	assert.Equal(t, "copilot[bot]", configs[1].Username)
	assert.Equal(t, "github-actions[bot]", configs[2].Username)
}

func TestBotConfigRepo_AddAndListAll(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBotConfigRepo(db)
	ctx := context.Background()

	err := repo.Add(ctx, model.BotConfig{
		Username: "dependabot[bot]",
		AddedAt:  time.Now().UTC(),
	})
	require.NoError(t, err)

	configs, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, configs, 4)

	// dependabot[bot] should be alphabetically between copilot[bot] and github-actions[bot]
	assert.Equal(t, "coderabbitai", configs[0].Username)
	assert.Equal(t, "copilot[bot]", configs[1].Username)
	assert.Equal(t, "dependabot[bot]", configs[2].Username)
	assert.Equal(t, "github-actions[bot]", configs[3].Username)
}

func TestBotConfigRepo_AddDuplicate(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBotConfigRepo(db)
	ctx := context.Background()

	err := repo.Add(ctx, model.BotConfig{
		Username: "coderabbitai",
		AddedAt:  time.Now().UTC(),
	})
	assert.Error(t, err, "adding duplicate username should fail")
}

func TestBotConfigRepo_Remove(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBotConfigRepo(db)
	ctx := context.Background()

	err := repo.Remove(ctx, "copilot[bot]")
	require.NoError(t, err)

	configs, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, configs, 2)

	assert.Equal(t, "coderabbitai", configs[0].Username)
	assert.Equal(t, "github-actions[bot]", configs[1].Username)
}

func TestBotConfigRepo_RemoveNonexistent(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBotConfigRepo(db)
	ctx := context.Background()

	err := repo.Remove(ctx, "nonexistent-bot")
	assert.Error(t, err, "removing nonexistent username should fail")
}

func TestBotConfigRepo_GetUsernames(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBotConfigRepo(db)
	ctx := context.Background()

	usernames, err := repo.GetUsernames(ctx)
	require.NoError(t, err)
	require.Len(t, usernames, 3)

	assert.Equal(t, []string{"coderabbitai", "copilot[bot]", "github-actions[bot]"}, usernames)
}
