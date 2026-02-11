package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/efisher/reviewhub/internal/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// addTestRepo inserts a repository required for foreign key constraints in PR tests.
func addTestRepo(t *testing.T, db *DB, fullName string) {
	t.Helper()
	parts := splitFullName(fullName)
	repoRepo := NewRepoRepo(db)
	err := repoRepo.Add(context.Background(), model.Repository{
		FullName: fullName,
		Owner:    parts[0],
		Name:     parts[1],
		AddedAt:  time.Now().UTC(),
	})
	require.NoError(t, err)
}

// splitFullName splits "owner/name" into ["owner", "name"].
func splitFullName(fullName string) [2]string {
	for i, c := range fullName {
		if c == '/' {
			return [2]string{fullName[:i], fullName[i+1:]}
		}
	}
	return [2]string{fullName, ""}
}

func makePR(repoFullName string, number int, title string, status model.PRStatus) model.PullRequest {
	now := time.Date(2026, 1, 20, 12, 0, 0, 0, time.UTC)
	return model.PullRequest{
		Number:         number,
		RepoFullName:   repoFullName,
		Title:          title,
		Author:         "testuser",
		Status:         status,
		IsDraft:        false,
		URL:            "https://github.com/" + repoFullName + "/pull/" + string(rune('0'+number)),
		Branch:         "feature-branch",
		BaseBranch:     "main",
		Labels:         []string{},
		OpenedAt:       now,
		UpdatedAt:      now,
		LastActivityAt: now,
	}
}

func TestPRRepo_Upsert_Insert(t *testing.T) {
	db := setupTestDB(t)
	addTestRepo(t, db, "octocat/hello-world")
	prRepo := NewPRRepo(db)
	ctx := context.Background()

	pr := makePR("octocat/hello-world", 1, "Add README", model.PRStatusOpen)
	require.NoError(t, prRepo.Upsert(ctx, pr))

	got, err := prRepo.GetByNumber(ctx, "octocat/hello-world", 1)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, 1, got.Number)
	assert.Equal(t, "octocat/hello-world", got.RepoFullName)
	assert.Equal(t, "Add README", got.Title)
	assert.Equal(t, "testuser", got.Author)
	assert.Equal(t, model.PRStatusOpen, got.Status)
	assert.False(t, got.IsDraft)
}

func TestPRRepo_Upsert_Update(t *testing.T) {
	db := setupTestDB(t)
	addTestRepo(t, db, "octocat/hello-world")
	prRepo := NewPRRepo(db)
	ctx := context.Background()

	pr := makePR("octocat/hello-world", 1, "Add README", model.PRStatusOpen)
	require.NoError(t, prRepo.Upsert(ctx, pr))

	// Update the title and status
	pr.Title = "Add README and LICENSE"
	pr.Status = model.PRStatusMerged
	require.NoError(t, prRepo.Upsert(ctx, pr))

	got, err := prRepo.GetByNumber(ctx, "octocat/hello-world", 1)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, "Add README and LICENSE", got.Title)
	assert.Equal(t, model.PRStatusMerged, got.Status)
}

func TestPRRepo_GetByRepository(t *testing.T) {
	db := setupTestDB(t)
	addTestRepo(t, db, "octocat/hello-world")
	addTestRepo(t, db, "octocat/other-repo")
	prRepo := NewPRRepo(db)
	ctx := context.Background()

	require.NoError(t, prRepo.Upsert(ctx, makePR("octocat/hello-world", 1, "PR 1", model.PRStatusOpen)))
	require.NoError(t, prRepo.Upsert(ctx, makePR("octocat/hello-world", 2, "PR 2", model.PRStatusOpen)))
	require.NoError(t, prRepo.Upsert(ctx, makePR("octocat/other-repo", 1, "Other PR", model.PRStatusOpen)))

	prs, err := prRepo.GetByRepository(ctx, "octocat/hello-world")
	require.NoError(t, err)
	require.Len(t, prs, 2)

	assert.Equal(t, 1, prs[0].Number)
	assert.Equal(t, 2, prs[1].Number)
}

func TestPRRepo_GetByStatus(t *testing.T) {
	db := setupTestDB(t)
	addTestRepo(t, db, "octocat/hello-world")
	prRepo := NewPRRepo(db)
	ctx := context.Background()

	require.NoError(t, prRepo.Upsert(ctx, makePR("octocat/hello-world", 1, "Open PR", model.PRStatusOpen)))
	require.NoError(t, prRepo.Upsert(ctx, makePR("octocat/hello-world", 2, "Closed PR", model.PRStatusClosed)))
	require.NoError(t, prRepo.Upsert(ctx, makePR("octocat/hello-world", 3, "Another Open", model.PRStatusOpen)))

	openPRs, err := prRepo.GetByStatus(ctx, model.PRStatusOpen)
	require.NoError(t, err)
	require.Len(t, openPRs, 2)

	closedPRs, err := prRepo.GetByStatus(ctx, model.PRStatusClosed)
	require.NoError(t, err)
	require.Len(t, closedPRs, 1)
	assert.Equal(t, "Closed PR", closedPRs[0].Title)
}

func TestPRRepo_GetByNumber_NotFound(t *testing.T) {
	db := setupTestDB(t)
	prRepo := NewPRRepo(db)
	ctx := context.Background()

	got, err := prRepo.GetByNumber(ctx, "nonexistent/repo", 999)
	require.NoError(t, err)
	assert.Nil(t, got, "non-existent PR should return nil without error")
}

func TestPRRepo_ListAll(t *testing.T) {
	db := setupTestDB(t)
	addTestRepo(t, db, "octocat/hello-world")
	addTestRepo(t, db, "octocat/other-repo")
	prRepo := NewPRRepo(db)
	ctx := context.Background()

	require.NoError(t, prRepo.Upsert(ctx, makePR("octocat/hello-world", 1, "PR 1", model.PRStatusOpen)))
	require.NoError(t, prRepo.Upsert(ctx, makePR("octocat/hello-world", 2, "PR 2", model.PRStatusClosed)))
	require.NoError(t, prRepo.Upsert(ctx, makePR("octocat/other-repo", 1, "PR 3", model.PRStatusMerged)))

	all, err := prRepo.ListAll(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 3)
}

func TestPRRepo_Delete(t *testing.T) {
	db := setupTestDB(t)
	addTestRepo(t, db, "octocat/hello-world")
	prRepo := NewPRRepo(db)
	ctx := context.Background()

	require.NoError(t, prRepo.Upsert(ctx, makePR("octocat/hello-world", 1, "To Delete", model.PRStatusOpen)))

	err := prRepo.Delete(ctx, "octocat/hello-world", 1)
	require.NoError(t, err)

	got, err := prRepo.GetByNumber(ctx, "octocat/hello-world", 1)
	require.NoError(t, err)
	assert.Nil(t, got, "deleted PR should not be found")
}

func TestPRRepo_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	prRepo := NewPRRepo(db)
	ctx := context.Background()

	err := prRepo.Delete(ctx, "nonexistent/repo", 999)
	assert.Error(t, err, "deleting non-existent PR should fail")
}

func TestPRRepo_Labels(t *testing.T) {
	db := setupTestDB(t)
	addTestRepo(t, db, "octocat/hello-world")
	prRepo := NewPRRepo(db)
	ctx := context.Background()

	pr := makePR("octocat/hello-world", 1, "Labeled PR", model.PRStatusOpen)
	pr.Labels = []string{"bug", "urgent", "help wanted"}
	require.NoError(t, prRepo.Upsert(ctx, pr))

	got, err := prRepo.GetByNumber(ctx, "octocat/hello-world", 1)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, []string{"bug", "urgent", "help wanted"}, got.Labels)
}

func TestPRRepo_Labels_Empty(t *testing.T) {
	db := setupTestDB(t)
	addTestRepo(t, db, "octocat/hello-world")
	prRepo := NewPRRepo(db)
	ctx := context.Background()

	pr := makePR("octocat/hello-world", 1, "No Labels", model.PRStatusOpen)
	pr.Labels = []string{}
	require.NoError(t, prRepo.Upsert(ctx, pr))

	got, err := prRepo.GetByNumber(ctx, "octocat/hello-world", 1)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Empty(t, got.Labels)
}

func TestPRRepo_Labels_Nil(t *testing.T) {
	db := setupTestDB(t)
	addTestRepo(t, db, "octocat/hello-world")
	prRepo := NewPRRepo(db)
	ctx := context.Background()

	pr := makePR("octocat/hello-world", 1, "Nil Labels", model.PRStatusOpen)
	pr.Labels = nil
	require.NoError(t, prRepo.Upsert(ctx, pr))

	got, err := prRepo.GetByNumber(ctx, "octocat/hello-world", 1)
	require.NoError(t, err)
	require.NotNil(t, got)

	// nil labels are stored as "[]" and read back as empty slice
	assert.Empty(t, got.Labels)
}

func TestPRRepo_CascadeDelete(t *testing.T) {
	db := setupTestDB(t)
	addTestRepo(t, db, "octocat/hello-world")
	prRepo := NewPRRepo(db)
	repoRepo := NewRepoRepo(db)
	ctx := context.Background()

	require.NoError(t, prRepo.Upsert(ctx, makePR("octocat/hello-world", 1, "PR 1", model.PRStatusOpen)))
	require.NoError(t, prRepo.Upsert(ctx, makePR("octocat/hello-world", 2, "PR 2", model.PRStatusOpen)))

	// Remove the repository -- PRs should cascade delete
	require.NoError(t, repoRepo.Remove(ctx, "octocat/hello-world"))

	prs, err := prRepo.GetByRepository(ctx, "octocat/hello-world")
	require.NoError(t, err)
	assert.Empty(t, prs, "PRs should be cascade-deleted with repository")
}

func TestPRRepo_ListNeedingReview(t *testing.T) {
	db := setupTestDB(t)
	addTestRepo(t, db, "octocat/hello-world")
	prRepo := NewPRRepo(db)
	ctx := context.Background()

	// PR that needs review
	prNeedsReview := makePR("octocat/hello-world", 1, "Review Me", model.PRStatusOpen)
	prNeedsReview.NeedsReview = true
	require.NoError(t, prRepo.Upsert(ctx, prNeedsReview))

	// PR that does not need review
	prNoReview := makePR("octocat/hello-world", 2, "My Own PR", model.PRStatusOpen)
	prNoReview.NeedsReview = false
	require.NoError(t, prRepo.Upsert(ctx, prNoReview))

	prs, err := prRepo.ListNeedingReview(ctx)
	require.NoError(t, err)
	require.Len(t, prs, 1)

	assert.Equal(t, 1, prs[0].Number)
	assert.Equal(t, "Review Me", prs[0].Title)
	assert.True(t, prs[0].NeedsReview)
}

func TestPRRepo_IsDraft(t *testing.T) {
	db := setupTestDB(t)
	addTestRepo(t, db, "octocat/hello-world")
	prRepo := NewPRRepo(db)
	ctx := context.Background()

	pr := makePR("octocat/hello-world", 1, "Draft PR", model.PRStatusOpen)
	pr.IsDraft = true
	require.NoError(t, prRepo.Upsert(ctx, pr))

	got, err := prRepo.GetByNumber(ctx, "octocat/hello-world", 1)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.True(t, got.IsDraft)
}
