package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/efisher/reviewhub/internal/application"
	"github.com/efisher/reviewhub/internal/domain/model"
)

// --- Mock implementations ---

type mockGitHubClient struct {
	fetchPRs             func(ctx context.Context, repoFullName string) ([]model.PullRequest, error)
	fetchReviews         func(ctx context.Context, repoFullName string, prNumber int) ([]model.Review, error)
	fetchReviewComments  func(ctx context.Context, repoFullName string, prNumber int) ([]model.ReviewComment, error)
	fetchIssueComments   func(ctx context.Context, repoFullName string, prNumber int) ([]model.IssueComment, error)
	fetchThreadResolution func(ctx context.Context, repoFullName string, prNumber int) (map[int64]bool, error)
}

func (m *mockGitHubClient) FetchPullRequests(ctx context.Context, repoFullName string) ([]model.PullRequest, error) {
	return m.fetchPRs(ctx, repoFullName)
}

func (m *mockGitHubClient) FetchReviews(ctx context.Context, repoFullName string, prNumber int) ([]model.Review, error) {
	if m.fetchReviews != nil {
		return m.fetchReviews(ctx, repoFullName, prNumber)
	}
	return nil, nil
}

func (m *mockGitHubClient) FetchReviewComments(ctx context.Context, repoFullName string, prNumber int) ([]model.ReviewComment, error) {
	if m.fetchReviewComments != nil {
		return m.fetchReviewComments(ctx, repoFullName, prNumber)
	}
	return nil, nil
}

func (m *mockGitHubClient) FetchIssueComments(ctx context.Context, repoFullName string, prNumber int) ([]model.IssueComment, error) {
	if m.fetchIssueComments != nil {
		return m.fetchIssueComments(ctx, repoFullName, prNumber)
	}
	return nil, nil
}

func (m *mockGitHubClient) FetchThreadResolution(ctx context.Context, repoFullName string, prNumber int) (map[int64]bool, error) {
	if m.fetchThreadResolution != nil {
		return m.fetchThreadResolution(ctx, repoFullName, prNumber)
	}
	return nil, nil
}

type upsertCall struct {
	PR model.PullRequest
}

type deleteCall struct {
	RepoFullName string
	Number       int
}

type mockPRStore struct {
	upserts []upsertCall
	deletes []deleteCall
	stored  []model.PullRequest
}

func (m *mockPRStore) Upsert(_ context.Context, pr model.PullRequest) error {
	m.upserts = append(m.upserts, upsertCall{PR: pr})
	return nil
}

func (m *mockPRStore) GetByRepository(_ context.Context, _ string) ([]model.PullRequest, error) {
	return m.stored, nil
}

func (m *mockPRStore) GetByStatus(_ context.Context, _ model.PRStatus) ([]model.PullRequest, error) {
	return nil, nil
}

func (m *mockPRStore) GetByNumber(_ context.Context, repoFullName string, number int) (*model.PullRequest, error) {
	for _, pr := range m.stored {
		if pr.RepoFullName == repoFullName && pr.Number == number {
			return &pr, nil
		}
	}
	// If not found in stored, return a PR with a default ID based on upserts.
	for _, u := range m.upserts {
		if u.PR.RepoFullName == repoFullName && u.PR.Number == number {
			pr := u.PR
			if pr.ID == 0 {
				pr.ID = int64(number) // Synthetic ID for testing.
			}
			return &pr, nil
		}
	}
	return nil, nil
}

func (m *mockPRStore) ListAll(_ context.Context) ([]model.PullRequest, error) {
	return nil, nil
}

func (m *mockPRStore) ListNeedingReview(_ context.Context) ([]model.PullRequest, error) {
	return nil, nil
}

func (m *mockPRStore) Delete(_ context.Context, repoFullName string, number int) error {
	m.deletes = append(m.deletes, deleteCall{RepoFullName: repoFullName, Number: number})
	return nil
}

type mockRepoStore struct {
	repos []model.Repository
}

func (m *mockRepoStore) Add(_ context.Context, _ model.Repository) error {
	return nil
}

func (m *mockRepoStore) Remove(_ context.Context, _ string) error {
	return nil
}

func (m *mockRepoStore) GetByFullName(_ context.Context, _ string) (*model.Repository, error) {
	return nil, nil
}

func (m *mockRepoStore) ListAll(_ context.Context) ([]model.Repository, error) {
	return m.repos, nil
}

// mockReviewStore records upsert/update calls for verification.
type mockReviewStore struct {
	upsertedReviews        []model.Review
	upsertedReviewComments []model.ReviewComment
	upsertedIssueComments  []model.IssueComment
	updatedResolutions     map[int64]bool
}

func newMockReviewStore() *mockReviewStore {
	return &mockReviewStore{
		updatedResolutions: make(map[int64]bool),
	}
}

func (m *mockReviewStore) UpsertReview(_ context.Context, review model.Review) error {
	m.upsertedReviews = append(m.upsertedReviews, review)
	return nil
}

func (m *mockReviewStore) UpsertReviewComment(_ context.Context, comment model.ReviewComment) error {
	m.upsertedReviewComments = append(m.upsertedReviewComments, comment)
	return nil
}

func (m *mockReviewStore) UpsertIssueComment(_ context.Context, comment model.IssueComment) error {
	m.upsertedIssueComments = append(m.upsertedIssueComments, comment)
	return nil
}

func (m *mockReviewStore) GetReviewsByPR(_ context.Context, _ int64) ([]model.Review, error) {
	return nil, nil
}

func (m *mockReviewStore) GetReviewCommentsByPR(_ context.Context, _ int64) ([]model.ReviewComment, error) {
	return nil, nil
}

func (m *mockReviewStore) GetIssueCommentsByPR(_ context.Context, _ int64) ([]model.IssueComment, error) {
	return nil, nil
}

func (m *mockReviewStore) UpdateCommentResolution(_ context.Context, commentID int64, isResolved bool) error {
	m.updatedResolutions[commentID] = isResolved
	return nil
}

func (m *mockReviewStore) DeleteReviewsByPR(_ context.Context, _ int64) error {
	return nil
}

// mockBotConfigStore returns empty config for poll service tests.
type mockBotConfigStore struct{}

func (m *mockBotConfigStore) Add(_ context.Context, _ model.BotConfig) error      { return nil }
func (m *mockBotConfigStore) Remove(_ context.Context, _ string) error             { return nil }
func (m *mockBotConfigStore) ListAll(_ context.Context) ([]model.BotConfig, error) { return nil, nil }
func (m *mockBotConfigStore) GetUsernames(_ context.Context) ([]string, error)     { return nil, nil }

// --- Helper to create a PollService and trigger a single repo poll ---

// pollRepoVia creates a PollService, starts it, and triggers a RefreshRepo
// to invoke pollRepo for the given repo. It returns after the refresh completes.
func pollRepoVia(t *testing.T, ghClient *mockGitHubClient, prStore *mockPRStore, username string, teamSlugs []string, repoFullName string) {
	t.Helper()
	pollRepoViaFull(t, ghClient, prStore, newMockReviewStore(), username, teamSlugs, repoFullName)
}

// pollRepoViaFull is like pollRepoVia but accepts a custom review store for verification.
func pollRepoViaFull(t *testing.T, ghClient *mockGitHubClient, prStore *mockPRStore, reviewStore *mockReviewStore, username string, teamSlugs []string, repoFullName string) {
	t.Helper()

	repoStore := &mockRepoStore{
		repos: []model.Repository{{FullName: repoFullName}},
	}

	svc := application.NewPollService(ghClient, prStore, repoStore, reviewStore, &mockBotConfigStore{}, username, teamSlugs, 1*time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start poll service in background. The initial pollAll will run immediately.
	done := make(chan struct{})
	go func() {
		svc.Start(ctx)
		close(done)
	}()

	// Wait briefly for the initial poll to complete, then trigger a refresh
	// to ensure consistent test behavior.
	time.Sleep(50 * time.Millisecond)

	// Clear any upserts/deletes from the initial poll so we test fresh.
	prStore.upserts = nil
	prStore.deletes = nil
	reviewStore.upsertedReviews = nil
	reviewStore.upsertedReviewComments = nil
	reviewStore.upsertedIssueComments = nil
	reviewStore.updatedResolutions = make(map[int64]bool)

	err := svc.RefreshRepo(ctx, repoFullName)
	require.NoError(t, err)

	cancel()
	<-done
}

// --- Tests ---

func TestPollRepo_AuthoredPRs(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	ghClient := &mockGitHubClient{
		fetchPRs: func(_ context.Context, _ string) ([]model.PullRequest, error) {
			return []model.PullRequest{
				{Number: 1, Author: "testuser", RepoFullName: "org/repo", Status: model.PRStatusOpen, UpdatedAt: now},
				{Number: 2, Author: "otheruser", RepoFullName: "org/repo", Status: model.PRStatusOpen, UpdatedAt: now},
				{Number: 3, Author: "anotheruser", RepoFullName: "org/repo", Status: model.PRStatusOpen, UpdatedAt: now},
			}, nil
		},
	}

	prStore := &mockPRStore{}
	pollRepoVia(t, ghClient, prStore, "testuser", nil, "org/repo")

	assert.Len(t, prStore.upserts, 1)
	assert.Equal(t, 1, prStore.upserts[0].PR.Number)
	assert.False(t, prStore.upserts[0].PR.NeedsReview, "authored PR should not need review")
}

func TestPollRepo_ReviewRequestedPRs(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	ghClient := &mockGitHubClient{
		fetchPRs: func(_ context.Context, _ string) ([]model.PullRequest, error) {
			return []model.PullRequest{
				{
					Number:             10,
					Author:             "alice",
					RepoFullName:       "org/repo",
					Status:             model.PRStatusOpen,
					UpdatedAt:          now,
					RequestedReviewers: []string{"testuser"},
				},
				{
					Number:       11,
					Author:       "bob",
					RepoFullName: "org/repo",
					Status:       model.PRStatusOpen,
					UpdatedAt:    now,
				},
			}, nil
		},
	}

	prStore := &mockPRStore{}
	pollRepoVia(t, ghClient, prStore, "testuser", nil, "org/repo")

	assert.Len(t, prStore.upserts, 1)
	assert.Equal(t, 10, prStore.upserts[0].PR.Number)
	assert.True(t, prStore.upserts[0].PR.NeedsReview, "review-requested PR should need review")
}

func TestPollRepo_TeamReviewRequest(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	ghClient := &mockGitHubClient{
		fetchPRs: func(_ context.Context, _ string) ([]model.PullRequest, error) {
			return []model.PullRequest{
				{
					Number:             20,
					Author:             "alice",
					RepoFullName:       "org/repo",
					Status:             model.PRStatusOpen,
					UpdatedAt:          now,
					RequestedTeamSlugs: []string{"my-team"},
				},
			}, nil
		},
	}

	prStore := &mockPRStore{}
	pollRepoVia(t, ghClient, prStore, "testuser", []string{"my-team"}, "org/repo")

	assert.Len(t, prStore.upserts, 1)
	assert.Equal(t, 20, prStore.upserts[0].PR.Number)
}

func TestPollRepo_Deduplication(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	ghClient := &mockGitHubClient{
		fetchPRs: func(_ context.Context, _ string) ([]model.PullRequest, error) {
			return []model.PullRequest{
				{
					Number:             30,
					Author:             "testuser",
					RepoFullName:       "org/repo",
					Status:             model.PRStatusOpen,
					UpdatedAt:          now,
					RequestedReviewers: []string{"testuser"},
				},
			}, nil
		},
	}

	prStore := &mockPRStore{}
	pollRepoVia(t, ghClient, prStore, "testuser", nil, "org/repo")

	// PR is both authored and review-requested, but only one upsert.
	assert.Len(t, prStore.upserts, 1)
}

func TestPollRepo_SkipUnchanged(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	ghClient := &mockGitHubClient{
		fetchPRs: func(_ context.Context, _ string) ([]model.PullRequest, error) {
			return []model.PullRequest{
				{
					Number:       40,
					Author:       "testuser",
					RepoFullName: "org/repo",
					Status:       model.PRStatusOpen,
					UpdatedAt:    now,
				},
			}, nil
		},
	}

	prStore := &mockPRStore{
		stored: []model.PullRequest{
			{
				Number:       40,
				Author:       "testuser",
				RepoFullName: "org/repo",
				Status:       model.PRStatusOpen,
				UpdatedAt:    now,
			},
		},
	}

	pollRepoVia(t, ghClient, prStore, "testuser", nil, "org/repo")

	// UpdatedAt matches, so upsert should be skipped.
	assert.Empty(t, prStore.upserts)
}

func TestPollRepo_DraftFlagging(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	ghClient := &mockGitHubClient{
		fetchPRs: func(_ context.Context, _ string) ([]model.PullRequest, error) {
			return []model.PullRequest{
				{
					Number:       50,
					Author:       "testuser",
					RepoFullName: "org/repo",
					Status:       model.PRStatusOpen,
					IsDraft:      true,
					UpdatedAt:    now,
				},
			}, nil
		},
	}

	prStore := &mockPRStore{}
	pollRepoVia(t, ghClient, prStore, "testuser", nil, "org/repo")

	require.Len(t, prStore.upserts, 1)
	assert.True(t, prStore.upserts[0].PR.IsDraft)
}

func TestPollRepo_StaleCleanup(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	ghClient := &mockGitHubClient{
		fetchPRs: func(_ context.Context, _ string) ([]model.PullRequest, error) {
			return []model.PullRequest{
				{Number: 11, Author: "testuser", RepoFullName: "org/repo", Status: model.PRStatusOpen, UpdatedAt: now},
				{Number: 12, Author: "testuser", RepoFullName: "org/repo", Status: model.PRStatusOpen, UpdatedAt: now},
			}, nil
		},
	}

	prStore := &mockPRStore{
		stored: []model.PullRequest{
			{Number: 10, Author: "testuser", RepoFullName: "org/repo", Status: model.PRStatusOpen, UpdatedAt: now.Add(-1 * time.Hour)},
			{Number: 11, Author: "testuser", RepoFullName: "org/repo", Status: model.PRStatusOpen, UpdatedAt: now},
			{Number: 12, Author: "testuser", RepoFullName: "org/repo", Status: model.PRStatusOpen, UpdatedAt: now},
		},
	}

	pollRepoVia(t, ghClient, prStore, "testuser", nil, "org/repo")

	// PR #10 is stale (not in fetched results, status open) -- should be deleted.
	require.Len(t, prStore.deletes, 1)
	assert.Equal(t, 10, prStore.deletes[0].Number)
	assert.Equal(t, "org/repo", prStore.deletes[0].RepoFullName)
}

func TestIsReviewRequestedFrom(t *testing.T) {
	tests := []struct {
		name      string
		pr        model.PullRequest
		username  string
		teamSlugs []string
		want      bool
	}{
		{
			name:     "username match case insensitive",
			pr:       model.PullRequest{RequestedReviewers: []string{"TestUser"}},
			username: "testuser",
			want:     true,
		},
		{
			name:      "team slug match",
			pr:        model.PullRequest{RequestedTeamSlugs: []string{"backend-team"}},
			username:  "testuser",
			teamSlugs: []string{"backend-team"},
			want:      true,
		},
		{
			name:      "team slug match case insensitive",
			pr:        model.PullRequest{RequestedTeamSlugs: []string{"Backend-Team"}},
			username:  "testuser",
			teamSlugs: []string{"backend-team"},
			want:      true,
		},
		{
			name:      "no match",
			pr:        model.PullRequest{RequestedReviewers: []string{"alice"}, RequestedTeamSlugs: []string{"frontend"}},
			username:  "testuser",
			teamSlugs: []string{"backend-team"},
			want:      false,
		},
		{
			name:     "empty reviewers",
			pr:       model.PullRequest{},
			username: "testuser",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := application.IsReviewRequestedFrom(tt.pr, tt.username, tt.teamSlugs)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- New tests for review data fetching ---

func TestPollRepo_FetchesReviewData(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	var fetchReviewsCalled, fetchReviewCommentsCalled, fetchIssueCommentsCalled bool

	ghClient := &mockGitHubClient{
		fetchPRs: func(_ context.Context, _ string) ([]model.PullRequest, error) {
			return []model.PullRequest{
				{Number: 60, Author: "testuser", RepoFullName: "org/repo", Status: model.PRStatusOpen, UpdatedAt: now},
			}, nil
		},
		fetchReviews: func(_ context.Context, _ string, _ int) ([]model.Review, error) {
			fetchReviewsCalled = true
			return []model.Review{
				{ID: 1001, ReviewerLogin: "alice", State: model.ReviewStateApproved, SubmittedAt: now},
			}, nil
		},
		fetchReviewComments: func(_ context.Context, _ string, _ int) ([]model.ReviewComment, error) {
			fetchReviewCommentsCalled = true
			return []model.ReviewComment{
				{ID: 2001, Author: "alice", Body: "looks good", Path: "main.go", Line: 5, CreatedAt: now, UpdatedAt: now},
			}, nil
		},
		fetchIssueComments: func(_ context.Context, _ string, _ int) ([]model.IssueComment, error) {
			fetchIssueCommentsCalled = true
			return []model.IssueComment{
				{ID: 3001, Author: "bob", Body: "nice work", CreatedAt: now, UpdatedAt: now},
			}, nil
		},
	}

	prStore := &mockPRStore{}
	reviewStore := newMockReviewStore()
	pollRepoViaFull(t, ghClient, prStore, reviewStore, "testuser", nil, "org/repo")

	// Verify review fetch methods were called.
	assert.True(t, fetchReviewsCalled, "FetchReviews should be called for changed PR")
	assert.True(t, fetchReviewCommentsCalled, "FetchReviewComments should be called for changed PR")
	assert.True(t, fetchIssueCommentsCalled, "FetchIssueComments should be called for changed PR")

	// Verify review store received upserts with correct PRID.
	require.Len(t, reviewStore.upsertedReviews, 1)
	assert.Equal(t, int64(60), reviewStore.upsertedReviews[0].PRID, "review PRID should match stored PR ID")

	require.Len(t, reviewStore.upsertedReviewComments, 1)
	assert.Equal(t, int64(60), reviewStore.upsertedReviewComments[0].PRID, "review comment PRID should match stored PR ID")

	require.Len(t, reviewStore.upsertedIssueComments, 1)
	assert.Equal(t, int64(60), reviewStore.upsertedIssueComments[0].PRID, "issue comment PRID should match stored PR ID")
}

func TestPollRepo_SkipsReviewDataForUnchangedPRs(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	var fetchReviewsCalled bool

	ghClient := &mockGitHubClient{
		fetchPRs: func(_ context.Context, _ string) ([]model.PullRequest, error) {
			return []model.PullRequest{
				{Number: 70, Author: "testuser", RepoFullName: "org/repo", Status: model.PRStatusOpen, UpdatedAt: now},
			}, nil
		},
		fetchReviews: func(_ context.Context, _ string, _ int) ([]model.Review, error) {
			fetchReviewsCalled = true
			return nil, nil
		},
	}

	prStore := &mockPRStore{
		stored: []model.PullRequest{
			{Number: 70, Author: "testuser", RepoFullName: "org/repo", Status: model.PRStatusOpen, UpdatedAt: now},
		},
	}

	reviewStore := newMockReviewStore()
	pollRepoViaFull(t, ghClient, prStore, reviewStore, "testuser", nil, "org/repo")

	// PR is unchanged (same UpdatedAt), so review fetch should NOT be called.
	assert.False(t, fetchReviewsCalled, "FetchReviews should NOT be called for unchanged PR")
	assert.Empty(t, reviewStore.upsertedReviews, "no reviews should be upserted for unchanged PR")
}
