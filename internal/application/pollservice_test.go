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
	fetchPRs func(ctx context.Context, repoFullName string) ([]model.PullRequest, error)
}

func (m *mockGitHubClient) FetchPullRequests(ctx context.Context, repoFullName string) ([]model.PullRequest, error) {
	return m.fetchPRs(ctx, repoFullName)
}

func (m *mockGitHubClient) FetchReviews(_ context.Context, _ string, _ int) ([]model.Review, error) {
	return nil, nil
}

func (m *mockGitHubClient) FetchReviewComments(_ context.Context, _ string, _ int) ([]model.ReviewComment, error) {
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

func (m *mockPRStore) GetByNumber(_ context.Context, _ string, _ int) (*model.PullRequest, error) {
	return nil, nil
}

func (m *mockPRStore) ListAll(_ context.Context) ([]model.PullRequest, error) {
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

// --- Helper to create a PollService and trigger a single repo poll ---

// pollRepoVia creates a PollService, starts it, and triggers a RefreshRepo
// to invoke pollRepo for the given repo. It returns after the refresh completes.
func pollRepoVia(t *testing.T, ghClient *mockGitHubClient, prStore *mockPRStore, username string, teamSlugs []string, repoFullName string) {
	t.Helper()

	repoStore := &mockRepoStore{
		repos: []model.Repository{{FullName: repoFullName}},
	}

	svc := application.NewPollService(ghClient, prStore, repoStore, username, teamSlugs, 1*time.Hour)

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
