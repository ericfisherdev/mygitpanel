package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// addTestPR inserts a PR for FK constraints in review/comment tests and returns
// the auto-generated database ID.
func addTestPR(t *testing.T, db *DB, repoFullName string, number int) int64 {
	t.Helper()
	addTestRepo(t, db, repoFullName)
	prRepo := NewPRRepo(db)
	pr := makePR(repoFullName, number, "Test PR", model.PRStatusOpen)
	require.NoError(t, prRepo.Upsert(context.Background(), pr))

	got, err := prRepo.GetByNumber(context.Background(), repoFullName, number)
	require.NoError(t, err)
	require.NotNil(t, got)
	return got.ID
}

func TestReviewRepo_UpsertAndGetReviews(t *testing.T) {
	db := setupTestDB(t)
	prID := addTestPR(t, db, "octocat/hello-world", 1)
	repo := NewReviewRepo(db)
	ctx := context.Background()

	earlier := time.Date(2026, 1, 20, 10, 0, 0, 0, time.UTC)
	later := time.Date(2026, 1, 20, 14, 0, 0, 0, time.UTC)

	review1 := model.Review{
		ID:            1001,
		PRID:          prID,
		ReviewerLogin: "alice",
		State:         model.ReviewStateApproved,
		Body:          "LGTM",
		CommitID:      "abc123",
		SubmittedAt:   later,
		IsBot:         false,
	}

	review2 := model.Review{
		ID:            1002,
		PRID:          prID,
		ReviewerLogin: "bob",
		State:         model.ReviewStateChangesRequested,
		Body:          "Please fix the tests",
		CommitID:      "def456",
		SubmittedAt:   earlier,
		IsBot:         false,
	}

	require.NoError(t, repo.UpsertReview(ctx, review1))
	require.NoError(t, repo.UpsertReview(ctx, review2))

	reviews, err := repo.GetReviewsByPR(ctx, prID)
	require.NoError(t, err)
	require.Len(t, reviews, 2)

	// Ordered by submitted_at: review2 (earlier) first
	assert.Equal(t, int64(1002), reviews[0].ID)
	assert.Equal(t, "bob", reviews[0].ReviewerLogin)
	assert.Equal(t, model.ReviewStateChangesRequested, reviews[0].State)
	assert.Equal(t, "def456", reviews[0].CommitID)

	assert.Equal(t, int64(1001), reviews[1].ID)
	assert.Equal(t, "alice", reviews[1].ReviewerLogin)
	assert.Equal(t, model.ReviewStateApproved, reviews[1].State)
	assert.Equal(t, "LGTM", reviews[1].Body)
	assert.Equal(t, "abc123", reviews[1].CommitID)
}

func TestReviewRepo_UpsertReviewComment_WithReply(t *testing.T) {
	db := setupTestDB(t)
	prID := addTestPR(t, db, "octocat/hello-world", 1)
	repo := NewReviewRepo(db)
	ctx := context.Background()

	now := time.Date(2026, 1, 20, 12, 0, 0, 0, time.UTC)

	rootComment := model.ReviewComment{
		ID:          2001,
		ReviewID:    1001,
		PRID:        prID,
		Author:      "alice",
		Body:        "This needs a nil check",
		Path:        "main.go",
		Line:        42,
		StartLine:   40,
		Side:        "RIGHT",
		SubjectType: "line",
		DiffHunk:    "@@ -38,6 +38,10 @@",
		CommitID:    "abc123",
		IsResolved:  false,
		IsOutdated:  false,
		InReplyToID: nil,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	replyToID := int64(2001)
	replyComment := model.ReviewComment{
		ID:          2002,
		ReviewID:    1001,
		PRID:        prID,
		Author:      "bob",
		Body:        "Good catch, will fix",
		Path:        "main.go",
		Line:        42,
		StartLine:   0,
		Side:        "RIGHT",
		SubjectType: "line",
		DiffHunk:    "@@ -38,6 +38,10 @@",
		CommitID:    "abc123",
		IsResolved:  false,
		IsOutdated:  false,
		InReplyToID: &replyToID,
		CreatedAt:   now.Add(time.Hour),
		UpdatedAt:   now.Add(time.Hour),
	}

	require.NoError(t, repo.UpsertReviewComment(ctx, rootComment))
	require.NoError(t, repo.UpsertReviewComment(ctx, replyComment))

	comments, err := repo.GetReviewCommentsByPR(ctx, prID)
	require.NoError(t, err)
	require.Len(t, comments, 2)

	// Root comment first (ordered by created_at)
	assert.Equal(t, int64(2001), comments[0].ID)
	assert.Equal(t, "This needs a nil check", comments[0].Body)
	assert.Equal(t, 40, comments[0].StartLine)
	assert.Equal(t, "line", comments[0].SubjectType)
	assert.Equal(t, "abc123", comments[0].CommitID)
	assert.Nil(t, comments[0].InReplyToID)

	// Reply comment second
	assert.Equal(t, int64(2002), comments[1].ID)
	assert.Equal(t, "Good catch, will fix", comments[1].Body)
	require.NotNil(t, comments[1].InReplyToID)
	assert.Equal(t, int64(2001), *comments[1].InReplyToID)
}

func TestReviewRepo_UpsertAndGetIssueComments(t *testing.T) {
	db := setupTestDB(t)
	prID := addTestPR(t, db, "octocat/hello-world", 1)
	repo := NewReviewRepo(db)
	ctx := context.Background()

	now := time.Date(2026, 1, 20, 12, 0, 0, 0, time.UTC)

	comment := model.IssueComment{
		ID:        3001,
		PRID:      prID,
		Author:    "coderabbitai",
		Body:      "## Summary\nThis PR adds feature X",
		IsBot:     true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	require.NoError(t, repo.UpsertIssueComment(ctx, comment))

	comments, err := repo.GetIssueCommentsByPR(ctx, prID)
	require.NoError(t, err)
	require.Len(t, comments, 1)

	assert.Equal(t, int64(3001), comments[0].ID)
	assert.Equal(t, "coderabbitai", comments[0].Author)
	assert.Equal(t, "## Summary\nThis PR adds feature X", comments[0].Body)
	assert.True(t, comments[0].IsBot)
}

func TestReviewRepo_UpdateCommentResolution(t *testing.T) {
	db := setupTestDB(t)
	prID := addTestPR(t, db, "octocat/hello-world", 1)
	repo := NewReviewRepo(db)
	ctx := context.Background()

	now := time.Date(2026, 1, 20, 12, 0, 0, 0, time.UTC)

	comment := model.ReviewComment{
		ID:          2001,
		ReviewID:    1001,
		PRID:        prID,
		Author:      "alice",
		Body:        "Fix this",
		Path:        "main.go",
		Line:        10,
		SubjectType: "line",
		IsResolved:  false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	require.NoError(t, repo.UpsertReviewComment(ctx, comment))

	// Resolve the comment
	require.NoError(t, repo.UpdateCommentResolution(ctx, 2001, true))

	comments, err := repo.GetReviewCommentsByPR(ctx, prID)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assert.True(t, comments[0].IsResolved)
}

func TestReviewRepo_DeleteReviewsByPR(t *testing.T) {
	db := setupTestDB(t)
	prID := addTestPR(t, db, "octocat/hello-world", 1)
	repo := NewReviewRepo(db)
	ctx := context.Background()

	now := time.Date(2026, 1, 20, 12, 0, 0, 0, time.UTC)

	// Insert a review, a review comment, and an issue comment
	require.NoError(t, repo.UpsertReview(ctx, model.Review{
		ID:            1001,
		PRID:          prID,
		ReviewerLogin: "alice",
		State:         model.ReviewStateApproved,
		CommitID:      "abc",
		SubmittedAt:   now,
	}))

	require.NoError(t, repo.UpsertReviewComment(ctx, model.ReviewComment{
		ID:          2001,
		PRID:        prID,
		Author:      "alice",
		Body:        "Note",
		SubjectType: "line",
		CreatedAt:   now,
		UpdatedAt:   now,
	}))

	require.NoError(t, repo.UpsertIssueComment(ctx, model.IssueComment{
		ID:        3001,
		PRID:      prID,
		Author:    "bot",
		Body:      "Bot note",
		CreatedAt: now,
		UpdatedAt: now,
	}))

	// Delete everything for this PR
	require.NoError(t, repo.DeleteReviewsByPR(ctx, prID))

	reviews, err := repo.GetReviewsByPR(ctx, prID)
	require.NoError(t, err)
	assert.Empty(t, reviews)

	reviewComments, err := repo.GetReviewCommentsByPR(ctx, prID)
	require.NoError(t, err)
	assert.Empty(t, reviewComments)

	issueComments, err := repo.GetIssueCommentsByPR(ctx, prID)
	require.NoError(t, err)
	assert.Empty(t, issueComments)
}

func TestReviewRepo_UpsertIdempotency(t *testing.T) {
	db := setupTestDB(t)
	prID := addTestPR(t, db, "octocat/hello-world", 1)
	repo := NewReviewRepo(db)
	ctx := context.Background()

	now := time.Date(2026, 1, 20, 12, 0, 0, 0, time.UTC)

	review := model.Review{
		ID:            1001,
		PRID:          prID,
		ReviewerLogin: "alice",
		State:         model.ReviewStateCommented,
		Body:          "First pass",
		CommitID:      "abc123",
		SubmittedAt:   now,
	}

	require.NoError(t, repo.UpsertReview(ctx, review))

	// Upsert again with updated body
	review.Body = "Updated review"
	review.State = model.ReviewStateApproved
	require.NoError(t, repo.UpsertReview(ctx, review))

	reviews, err := repo.GetReviewsByPR(ctx, prID)
	require.NoError(t, err)
	require.Len(t, reviews, 1, "should have exactly one row after double upsert")

	assert.Equal(t, "Updated review", reviews[0].Body)
	assert.Equal(t, model.ReviewStateApproved, reviews[0].State)
}
