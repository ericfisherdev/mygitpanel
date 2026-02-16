package application

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// --- Mock implementations for ReviewService tests ---

type mockReviewStoreForService struct {
	reviews        []model.Review
	reviewComments []model.ReviewComment
	issueComments  []model.IssueComment
}

func (m *mockReviewStoreForService) UpsertReview(_ context.Context, _ model.Review) error {
	return nil
}

func (m *mockReviewStoreForService) UpsertReviewComment(_ context.Context, _ model.ReviewComment) error {
	return nil
}

func (m *mockReviewStoreForService) UpsertIssueComment(_ context.Context, _ model.IssueComment) error {
	return nil
}

func (m *mockReviewStoreForService) GetReviewsByPR(_ context.Context, _ int64) ([]model.Review, error) {
	return m.reviews, nil
}

func (m *mockReviewStoreForService) GetReviewCommentsByPR(_ context.Context, _ int64) ([]model.ReviewComment, error) {
	return m.reviewComments, nil
}

func (m *mockReviewStoreForService) GetIssueCommentsByPR(_ context.Context, _ int64) ([]model.IssueComment, error) {
	return m.issueComments, nil
}

func (m *mockReviewStoreForService) UpdateCommentResolution(_ context.Context, _ int64, _ bool) error {
	return nil
}

func (m *mockReviewStoreForService) DeleteReviewsByPR(_ context.Context, _ int64) error {
	return nil
}

func (m *mockReviewStoreForService) CountApprovals(_ context.Context, _ int64) (int, error) {
	return 0, nil
}

type mockBotConfigStoreForService struct {
	usernames []string
}

func (m *mockBotConfigStoreForService) Add(_ context.Context, config model.BotConfig) (model.BotConfig, error) {
	return config, nil
}

func (m *mockBotConfigStoreForService) Remove(_ context.Context, _ string) error {
	return nil
}

func (m *mockBotConfigStoreForService) ListAll(_ context.Context) ([]model.BotConfig, error) {
	return nil, nil
}

func (m *mockBotConfigStoreForService) GetUsernames(_ context.Context) ([]string, error) {
	return m.usernames, nil
}

// --- Helper functions ---

func int64Ptr(v int64) *int64 {
	return &v
}

// --- Tests for groupIntoThreads ---

func TestGroupIntoThreads(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	comments := []model.ReviewComment{
		{ID: 100, InReplyToID: nil, CreatedAt: now, IsResolved: true},
		{ID: 101, InReplyToID: int64Ptr(100), CreatedAt: now.Add(2 * time.Minute)},
		{ID: 102, InReplyToID: int64Ptr(100), CreatedAt: now.Add(1 * time.Minute)},
	}

	threads := groupIntoThreads(comments)

	require.Len(t, threads, 1)
	assert.Equal(t, int64(100), threads[0].RootComment.ID)
	assert.True(t, threads[0].IsResolved)
	require.Len(t, threads[0].Replies, 2)
	// Replies sorted by CreatedAt: 102 (1min) before 101 (2min).
	assert.Equal(t, int64(102), threads[0].Replies[0].ID)
	assert.Equal(t, int64(101), threads[0].Replies[1].ID)
}

func TestGroupIntoThreads_MultipleThreads(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	comments := []model.ReviewComment{
		{ID: 200, InReplyToID: nil, CreatedAt: now},
		{ID: 300, InReplyToID: nil, CreatedAt: now.Add(5 * time.Minute)},
		{ID: 201, InReplyToID: int64Ptr(200), CreatedAt: now.Add(1 * time.Minute)},
		{ID: 301, InReplyToID: int64Ptr(300), CreatedAt: now.Add(6 * time.Minute)},
	}

	threads := groupIntoThreads(comments)

	require.Len(t, threads, 2)
	// Sorted by root CreatedAt: thread 200 first.
	assert.Equal(t, int64(200), threads[0].RootComment.ID)
	assert.Equal(t, int64(300), threads[1].RootComment.ID)
	require.Len(t, threads[0].Replies, 1)
	require.Len(t, threads[1].Replies, 1)
}

func TestGroupIntoThreads_OrphanReply(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	comments := []model.ReviewComment{
		{ID: 500, InReplyToID: int64Ptr(999), CreatedAt: now, IsResolved: false},
	}

	threads := groupIntoThreads(comments)

	require.Len(t, threads, 1)
	// Orphan becomes its own root.
	assert.Equal(t, int64(500), threads[0].RootComment.ID)
	assert.Empty(t, threads[0].Replies)
	assert.False(t, threads[0].IsResolved)
}

// --- Tests for extractSuggestions ---

func TestExtractSuggestions_HasSuggestion(t *testing.T) {
	body := "Please change this:\n```suggestion\nfmt.Println(\"fixed\")\n```\nThanks!"

	comments := []model.ReviewComment{
		{
			ID:        10,
			Path:      "main.go",
			Line:      42,
			StartLine: 40,
			Body:      body,
		},
	}

	suggestions := extractSuggestions(comments)

	require.Len(t, suggestions, 1)
	assert.Equal(t, int64(10), suggestions[0].CommentID)
	assert.Equal(t, "main.go", suggestions[0].FilePath)
	assert.Equal(t, 40, suggestions[0].StartLine)
	assert.Equal(t, 42, suggestions[0].EndLine)
	assert.Equal(t, "fmt.Println(\"fixed\")", suggestions[0].ProposedCode)
	assert.Equal(t, body, suggestions[0].OriginalBody)
}

func TestExtractSuggestions_NoSuggestion(t *testing.T) {
	comments := []model.ReviewComment{
		{ID: 11, Body: "This looks good, no suggestion here."},
	}

	suggestions := extractSuggestions(comments)
	assert.Empty(t, suggestions)
}

func TestExtractSuggestions_MultipleSuggestions(t *testing.T) {
	comments := []model.ReviewComment{
		{ID: 20, Path: "a.go", Line: 10, Body: "```suggestion\ncode1\n```"},
		{ID: 21, Path: "b.go", Line: 20, Body: "```suggestion\ncode2\n```"},
	}

	suggestions := extractSuggestions(comments)

	require.Len(t, suggestions, 2)
	assert.Equal(t, "code1", suggestions[0].ProposedCode)
	assert.Equal(t, "code2", suggestions[1].ProposedCode)
}

func TestExtractSuggestions_StartLineZeroFallsBackToLine(t *testing.T) {
	comments := []model.ReviewComment{
		{ID: 30, Path: "c.go", Line: 15, StartLine: 0, Body: "```suggestion\nfixed\n```"},
	}

	suggestions := extractSuggestions(comments)

	require.Len(t, suggestions, 1)
	assert.Equal(t, 15, suggestions[0].StartLine)
	assert.Equal(t, 15, suggestions[0].EndLine)
}

// --- Tests for isBotUser ---

func TestIsBotUser(t *testing.T) {
	bots := []string{"coderabbitai", "github-actions[bot]", "copilot[bot]"}

	assert.True(t, isBotUser("coderabbitai", bots))
	assert.True(t, isBotUser("CodeRabbitAI", bots), "case-insensitive match")
	assert.True(t, isBotUser("CODERABBITAI", bots), "uppercase match")
	assert.False(t, isBotUser("humanuser", bots))
	assert.False(t, isBotUser("", bots))
}

// --- Tests for isNitpickComment ---

func TestIsNitpickComment(t *testing.T) {
	bots := []string{"coderabbitai"}

	tests := []struct {
		name   string
		author string
		body   string
		want   bool
	}{
		{"bold nitpick", "coderabbitai", "**Nitpick** blah", true},
		{"bracket nitpick", "coderabbitai", "Some [nitpick] here", true},
		{"paren nitpick", "coderabbitai", "This is (nitpick) stuff", true},
		{"colon nitpick", "coderabbitai", "nitpick: minor thing", true},
		{"non-blocking nitpick", "coderabbitai", "nitpick (non-blocking)", true},
		{"non-bot author", "humanuser", "**Nitpick** blah", false},
		{"bot no nitpick", "coderabbitai", "This is a regular comment", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNitpickComment(tt.author, tt.body, bots)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- Tests for isReviewOutdated ---

func TestIsReviewOutdated(t *testing.T) {
	tests := []struct {
		name     string
		commitID string
		headSHA  string
		want     bool
	}{
		{"same commit", "abc123", "abc123", false},
		{"different commit", "abc123", "def456", true},
		{"empty commit", "", "abc123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			review := model.Review{CommitID: tt.commitID}
			got := isReviewOutdated(review, tt.headSHA)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- Tests for aggregateReviewStatus ---

func TestAggregateReviewStatus_ChangesRequested(t *testing.T) {
	bots := []string{"bot"}
	now := time.Now()

	reviews := []model.Review{
		{ReviewerLogin: "alice", State: model.ReviewStateApproved, SubmittedAt: now},
		{ReviewerLogin: "bob", State: model.ReviewStateChangesRequested, SubmittedAt: now},
	}

	status := aggregateReviewStatus(reviews, bots)
	assert.Equal(t, model.ReviewStateChangesRequested, status)
}

func TestAggregateReviewStatus_Approved(t *testing.T) {
	bots := []string{"bot"}
	now := time.Now()

	reviews := []model.Review{
		{ReviewerLogin: "alice", State: model.ReviewStateApproved, SubmittedAt: now},
		{ReviewerLogin: "bob", State: model.ReviewStateApproved, SubmittedAt: now},
	}

	status := aggregateReviewStatus(reviews, bots)
	assert.Equal(t, model.ReviewStateApproved, status)
}

func TestAggregateReviewStatus_LatestWins(t *testing.T) {
	bots := []string{"bot"}
	now := time.Now()

	reviews := []model.Review{
		{ReviewerLogin: "alice", State: model.ReviewStateChangesRequested, SubmittedAt: now},
		{ReviewerLogin: "alice", State: model.ReviewStateApproved, SubmittedAt: now.Add(1 * time.Hour)},
	}

	status := aggregateReviewStatus(reviews, bots)
	assert.Equal(t, model.ReviewStateApproved, status)
}

func TestAggregateReviewStatus_IgnoreBots(t *testing.T) {
	bots := []string{"coderabbitai"}
	now := time.Now()

	reviews := []model.Review{
		{ReviewerLogin: "coderabbitai", State: model.ReviewStateApproved, SubmittedAt: now},
	}

	status := aggregateReviewStatus(reviews, bots)
	assert.Equal(t, model.ReviewStatePending, status, "bot reviews should not count toward status")
}

func TestAggregateReviewStatus_NoReviews(t *testing.T) {
	status := aggregateReviewStatus(nil, nil)
	assert.Equal(t, model.ReviewStatePending, status)
}

// --- Tests for GetPRReviewSummary ---

func TestGetPRReviewSummary(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	reviewStore := &mockReviewStoreForService{
		reviews: []model.Review{
			{
				ID:            1,
				PRID:          42,
				ReviewerLogin: "alice",
				State:         model.ReviewStateApproved,
				CommitID:      "current-sha",
				SubmittedAt:   now,
			},
			{
				ID:            2,
				PRID:          42,
				ReviewerLogin: "coderabbitai",
				State:         model.ReviewStateCommented,
				CommitID:      "old-sha",
				SubmittedAt:   now.Add(-1 * time.Hour),
				IsBot:         false, // Will be set by enrichment.
			},
		},
		reviewComments: []model.ReviewComment{
			{
				ID:          100,
				PRID:        42,
				Author:      "alice",
				Body:        "Please fix this:\n```suggestion\nreturn nil\n```",
				Path:        "main.go",
				Line:        10,
				StartLine:   8,
				InReplyToID: nil,
				CreatedAt:   now,
				IsResolved:  true,
			},
			{
				ID:          101,
				PRID:        42,
				Author:      "bob",
				Body:        "I agree with Alice",
				InReplyToID: int64Ptr(100),
				CreatedAt:   now.Add(1 * time.Minute),
			},
			{
				ID:          102,
				PRID:        42,
				Author:      "alice",
				Body:        "Another comment on a different line",
				Path:        "util.go",
				Line:        20,
				InReplyToID: nil,
				CreatedAt:   now.Add(2 * time.Minute),
				IsResolved:  false,
			},
		},
		issueComments: []model.IssueComment{
			{
				ID:     200,
				PRID:   42,
				Author: "coderabbitai",
				Body:   "Automated review summary",
			},
			{
				ID:     201,
				PRID:   42,
				Author: "humandev",
				Body:   "Looks good overall",
			},
		},
	}

	botConfigStore := &mockBotConfigStoreForService{
		usernames: []string{"coderabbitai", "github-actions[bot]"},
	}

	svc := NewReviewService(reviewStore, botConfigStore)
	summary, err := svc.GetPRReviewSummary(context.Background(), 42, "current-sha")

	require.NoError(t, err)
	require.NotNil(t, summary)

	// Reviews enriched with bot flag.
	assert.False(t, summary.Reviews[0].IsBot, "alice is not a bot")
	assert.True(t, summary.Reviews[1].IsBot, "coderabbitai is a bot")

	// Threads: 2 threads (comment 100 with reply 101, comment 102 alone).
	require.Len(t, summary.Threads, 2)
	assert.Equal(t, int64(100), summary.Threads[0].RootComment.ID)
	assert.True(t, summary.Threads[0].IsResolved)
	require.Len(t, summary.Threads[0].Replies, 1)
	assert.Equal(t, int64(102), summary.Threads[1].RootComment.ID)
	assert.False(t, summary.Threads[1].IsResolved)

	// Thread counts.
	assert.Equal(t, 1, summary.ResolvedThreadCount)
	assert.Equal(t, 1, summary.UnresolvedThreadCount)

	// Issue comments enriched with bot flag.
	assert.True(t, summary.IssueComments[0].IsBot, "coderabbitai is a bot")
	assert.False(t, summary.IssueComments[1].IsBot, "humandev is not a bot")

	// Suggestions: 1 from comment 100.
	require.Len(t, summary.Suggestions, 1)
	assert.Equal(t, int64(100), summary.Suggestions[0].CommentID)
	assert.Equal(t, "return nil", summary.Suggestions[0].ProposedCode)

	// Review status: alice approved (non-bot), coderabbitai ignored.
	assert.Equal(t, model.ReviewStateApproved, summary.ReviewStatus)

	// Bot flags.
	assert.True(t, summary.HasBotReview)
	assert.True(t, summary.HasCoderabbitReview)
	assert.True(t, summary.AwaitingCoderabbit, "coderabbit reviewed old-sha, not current-sha")
}
