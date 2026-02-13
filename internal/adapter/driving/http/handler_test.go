package httphandler_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	httphandler "github.com/ericfisherdev/mygitpanel/internal/adapter/driving/http"
	"github.com/ericfisherdev/mygitpanel/internal/application"
	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock implementations ---

type mockPRStore struct {
	prs []model.PullRequest
	pr  *model.PullRequest
	err error
}

func (m *mockPRStore) Upsert(_ context.Context, _ model.PullRequest) error { return nil }
func (m *mockPRStore) GetByRepository(_ context.Context, _ string) ([]model.PullRequest, error) {
	return nil, nil
}
func (m *mockPRStore) GetByStatus(_ context.Context, _ model.PRStatus) ([]model.PullRequest, error) {
	return nil, nil
}
func (m *mockPRStore) GetByNumber(_ context.Context, _ string, _ int) (*model.PullRequest, error) {
	return m.pr, m.err
}
func (m *mockPRStore) ListAll(_ context.Context) ([]model.PullRequest, error) {
	return m.prs, m.err
}
func (m *mockPRStore) ListNeedingReview(_ context.Context) ([]model.PullRequest, error) {
	return m.prs, m.err
}
func (m *mockPRStore) Delete(_ context.Context, _ string, _ int) error { return nil }

type mockRepoStore struct {
	repos     []model.Repository
	err       error
	addErr    error
	removeErr error
	addedRepo model.Repository
}

func (m *mockRepoStore) Add(_ context.Context, repo model.Repository) error {
	m.addedRepo = repo
	return m.addErr
}
func (m *mockRepoStore) Remove(_ context.Context, _ string) error {
	return m.removeErr
}
func (m *mockRepoStore) GetByFullName(_ context.Context, _ string) (*model.Repository, error) {
	return nil, nil
}
func (m *mockRepoStore) ListAll(_ context.Context) ([]model.Repository, error) {
	return m.repos, m.err
}

type mockBotConfigStore struct {
	bots      []model.BotConfig
	usernames []string
	err       error
	addErr    error
	removeErr error
}

func (m *mockBotConfigStore) Add(_ context.Context, bot model.BotConfig) (model.BotConfig, error) {
	return bot, m.addErr
}
func (m *mockBotConfigStore) Remove(_ context.Context, _ string) error {
	return m.removeErr
}
func (m *mockBotConfigStore) ListAll(_ context.Context) ([]model.BotConfig, error) {
	return m.bots, m.err
}
func (m *mockBotConfigStore) GetUsernames(_ context.Context) ([]string, error) {
	return m.usernames, m.err
}

// mockCheckStore implements driven.CheckStore for handler tests.
type mockCheckStore struct {
	checkRuns []model.CheckRun
	err       error
}

func (m *mockCheckStore) ReplaceCheckRunsForPR(_ context.Context, _ int64, _ []model.CheckRun) error {
	return nil
}
func (m *mockCheckStore) GetCheckRunsByPR(_ context.Context, _ int64) ([]model.CheckRun, error) {
	return m.checkRuns, m.err
}

// mockReviewStore implements driven.ReviewStore for handler tests.
type mockReviewStore struct {
	reviews        []model.Review
	reviewComments []model.ReviewComment
	issueComments  []model.IssueComment
}

func (m *mockReviewStore) UpsertReview(_ context.Context, _ model.Review) error { return nil }
func (m *mockReviewStore) UpsertReviewComment(_ context.Context, _ model.ReviewComment) error {
	return nil
}
func (m *mockReviewStore) UpsertIssueComment(_ context.Context, _ model.IssueComment) error {
	return nil
}
func (m *mockReviewStore) GetReviewsByPR(_ context.Context, _ int64) ([]model.Review, error) {
	return m.reviews, nil
}
func (m *mockReviewStore) GetReviewCommentsByPR(_ context.Context, _ int64) ([]model.ReviewComment, error) {
	return m.reviewComments, nil
}
func (m *mockReviewStore) GetIssueCommentsByPR(_ context.Context, _ int64) ([]model.IssueComment, error) {
	return m.issueComments, nil
}
func (m *mockReviewStore) UpdateCommentResolution(_ context.Context, _ int64, _ bool) error {
	return nil
}
func (m *mockReviewStore) DeleteReviewsByPR(_ context.Context, _ int64) error { return nil }

// errReviewStore returns an error from GetReviewsByPR.
type errReviewStore struct{ mockReviewStore }

func (m *errReviewStore) GetReviewsByPR(_ context.Context, _ int64) ([]model.Review, error) {
	return nil, errors.New("review store error")
}

// --- Test helpers ---

var (
	testTime    = time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	testTimeStr = "2026-02-10T12:00:00Z"
)

// setupMux creates a mux with nil reviewSvc, nil healthSvc, and nil botConfigStore (backward compat).
func setupMux(prStore *mockPRStore, repoStore *mockRepoStore) http.Handler {
	h := httphandler.NewHandler(prStore, repoStore, nil, nil, nil, nil, "testuser", slog.Default())
	return httphandler.NewServeMux(h, slog.Default())
}

// setupMuxWithReview creates a mux with a real ReviewService backed by mock stores.
func setupMuxWithReview(
	prStore *mockPRStore,
	repoStore *mockRepoStore,
	botConfigStore driven.BotConfigStore,
	reviewStore driven.ReviewStore,
) http.Handler {
	reviewSvc := application.NewReviewService(reviewStore, botConfigStore)
	h := httphandler.NewHandler(prStore, repoStore, botConfigStore, reviewSvc, nil, nil, "testuser", slog.Default())
	return httphandler.NewServeMux(h, slog.Default())
}

// setupMuxWithBots creates a mux for bot config endpoint tests.
func setupMuxWithBots(botStore *mockBotConfigStore) http.Handler {
	h := httphandler.NewHandler(&mockPRStore{}, &mockRepoStore{}, botStore, nil, nil, nil, "testuser", slog.Default())
	return httphandler.NewServeMux(h, slog.Default())
}

// setupMuxWithHealth creates a mux with a real HealthService backed by a mock CheckStore.
func setupMuxWithHealth(
	prStore *mockPRStore,
	repoStore *mockRepoStore,
	checkStore driven.CheckStore,
) http.Handler {
	healthSvc := application.NewHealthService(checkStore, prStore)
	h := httphandler.NewHandler(prStore, repoStore, nil, nil, healthSvc, nil, "testuser", slog.Default())
	return httphandler.NewServeMux(h, slog.Default())
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	require.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	err := json.NewDecoder(rec.Body).Decode(v)
	require.NoError(t, err)
}

// --- Tests ---

func TestListPRs(t *testing.T) {
	tests := []struct {
		name       string
		prStore    *mockPRStore
		wantStatus int
		wantLen    int
		checkFirst func(t *testing.T, pr map[string]any)
	}{
		{
			name:       "empty list",
			prStore:    &mockPRStore{prs: nil},
			wantStatus: http.StatusOK,
			wantLen:    0,
		},
		{
			name: "two PRs",
			prStore: &mockPRStore{prs: []model.PullRequest{
				{
					Number:       42,
					RepoFullName: "owner/repo",
					Title:        "Fix bug",
					Author:       "alice",
					Status:       model.PRStatusOpen,
					IsDraft:      false,
					NeedsReview:  true,
					URL:          "https://github.com/owner/repo/pull/42",
					Branch:       "fix-bug",
					BaseBranch:   "main",
					Labels:       []string{"bug", "urgent"},
					OpenedAt:     testTime,
					UpdatedAt:    testTime,
				},
				{
					Number:       43,
					RepoFullName: "owner/repo",
					Title:        "Add feature",
					Author:       "bob",
					Status:       model.PRStatusMerged,
					IsDraft:      true,
					URL:          "https://github.com/owner/repo/pull/43",
					Branch:       "add-feature",
					BaseBranch:   "main",
					Labels:       nil,
					OpenedAt:     testTime,
					UpdatedAt:    testTime,
				},
			}},
			wantStatus: http.StatusOK,
			wantLen:    2,
			checkFirst: func(t *testing.T, pr map[string]any) {
				assert.Equal(t, float64(42), pr["number"])
				assert.Equal(t, "owner/repo", pr["repository"])
				assert.Equal(t, "Fix bug", pr["title"])
				assert.Equal(t, "alice", pr["author"])
				assert.Equal(t, "open", pr["status"])
				assert.Equal(t, false, pr["is_draft"])
				assert.Equal(t, true, pr["needs_review"])
				assert.Equal(t, "https://github.com/owner/repo/pull/42", pr["url"])
				assert.Equal(t, "fix-bug", pr["branch"])
				assert.Equal(t, "main", pr["base_branch"])
				assert.Equal(t, testTimeStr, pr["opened_at"])
				assert.Equal(t, testTimeStr, pr["updated_at"])
				// Labels is a non-nil array
				labels, ok := pr["labels"].([]any)
				require.True(t, ok)
				assert.Len(t, labels, 2)
				// Reviews and IssueComments are empty arrays, not null
				reviews, ok := pr["reviews"].([]any)
				require.True(t, ok)
				assert.Len(t, reviews, 0)
				issueComments, ok := pr["issue_comments"].([]any)
				require.True(t, ok)
				assert.Len(t, issueComments, 0)
			},
		},
		{
			name:       "store error",
			prStore:    &mockPRStore{err: errors.New("db fail")},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := setupMux(tt.prStore, &mockRepoStore{})
			req := httptest.NewRequest(http.MethodGet, "/api/v1/prs", nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)

			if tt.wantStatus == http.StatusOK {
				var resp []map[string]any
				decodeJSON(t, rec, &resp)
				assert.Len(t, resp, tt.wantLen)

				if tt.checkFirst != nil && len(resp) > 0 {
					tt.checkFirst(t, resp[0])
				}
			}
		})
	}
}

func TestGetPR(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		prStore    *mockPRStore
		wantStatus int
		wantError  string
	}{
		{
			name: "found",
			path: "/api/v1/repos/owner/repo/prs/42",
			prStore: &mockPRStore{pr: &model.PullRequest{
				Number:       42,
				RepoFullName: "owner/repo",
				Title:        "Fix bug",
				Author:       "alice",
				Status:       model.PRStatusOpen,
				URL:          "https://github.com/owner/repo/pull/42",
				Branch:       "fix-bug",
				BaseBranch:   "main",
				OpenedAt:     testTime,
				UpdatedAt:    testTime,
			}},
			wantStatus: http.StatusOK,
		},
		{
			name:       "not found",
			path:       "/api/v1/repos/owner/repo/prs/99",
			prStore:    &mockPRStore{pr: nil},
			wantStatus: http.StatusNotFound,
			wantError:  "pull request not found",
		},
		{
			name:       "invalid number",
			path:       "/api/v1/repos/owner/repo/prs/abc",
			prStore:    &mockPRStore{},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid PR number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := setupMux(tt.prStore, &mockRepoStore{})
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)

			if tt.wantStatus == http.StatusOK {
				var resp map[string]any
				decodeJSON(t, rec, &resp)
				assert.Equal(t, float64(42), resp["number"])
				assert.Equal(t, "owner/repo", resp["repository"])
			}

			if tt.wantError != "" {
				var resp map[string]any
				decodeJSON(t, rec, &resp)
				assert.Equal(t, tt.wantError, resp["error"])
			}
		})
	}
}

func TestListPRsNeedingAttention(t *testing.T) {
	tests := []struct {
		name       string
		prStore    *mockPRStore
		wantStatus int
		wantLen    int
	}{
		{
			name: "returns only needing review",
			prStore: &mockPRStore{prs: []model.PullRequest{
				{
					Number:       42,
					RepoFullName: "owner/repo",
					Title:        "Needs review",
					Author:       "alice",
					Status:       model.PRStatusOpen,
					NeedsReview:  true,
					OpenedAt:     testTime,
					UpdatedAt:    testTime,
				},
			}},
			wantStatus: http.StatusOK,
			wantLen:    1,
		},
		{
			name:       "empty",
			prStore:    &mockPRStore{prs: nil},
			wantStatus: http.StatusOK,
			wantLen:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := setupMux(tt.prStore, &mockRepoStore{})
			req := httptest.NewRequest(http.MethodGet, "/api/v1/prs/attention", nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)

			var resp []map[string]any
			decodeJSON(t, rec, &resp)
			assert.Len(t, resp, tt.wantLen)
		})
	}
}

func TestListRepos(t *testing.T) {
	tests := []struct {
		name       string
		repoStore  *mockRepoStore
		wantStatus int
		wantLen    int
	}{
		{
			name:       "empty",
			repoStore:  &mockRepoStore{repos: nil},
			wantStatus: http.StatusOK,
			wantLen:    0,
		},
		{
			name: "two repos",
			repoStore: &mockRepoStore{repos: []model.Repository{
				{FullName: "owner/repo1", Owner: "owner", Name: "repo1", AddedAt: testTime},
				{FullName: "owner/repo2", Owner: "owner", Name: "repo2", AddedAt: testTime},
			}},
			wantStatus: http.StatusOK,
			wantLen:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := setupMux(&mockPRStore{}, tt.repoStore)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/repos", nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)

			var resp []map[string]any
			decodeJSON(t, rec, &resp)
			assert.Len(t, resp, tt.wantLen)

			if tt.wantLen == 2 {
				assert.Equal(t, "owner/repo1", resp[0]["full_name"])
				assert.Equal(t, "owner", resp[0]["owner"])
				assert.Equal(t, "repo1", resp[0]["name"])
				assert.Equal(t, testTimeStr, resp[0]["added_at"])
			}
		})
	}
}

func TestAddRepo(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		repoStore  *mockRepoStore
		wantStatus int
		wantError  string
	}{
		{
			name:       "valid",
			body:       `{"full_name": "owner/repo"}`,
			repoStore:  &mockRepoStore{},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "invalid format - no slash",
			body:       `{"full_name": "invalid"}`,
			repoStore:  &mockRepoStore{},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid repository name: expected owner/repo format",
		},
		{
			name:       "invalid format - empty owner",
			body:       `{"full_name": "/repo"}`,
			repoStore:  &mockRepoStore{},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid repository name: expected owner/repo format",
		},
		{
			name:       "invalid format - extra slashes",
			body:       `{"full_name": "a/b/c"}`,
			repoStore:  &mockRepoStore{},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid repository name: expected owner/repo format",
		},
		{
			name:       "duplicate",
			body:       `{"full_name": "owner/repo"}`,
			repoStore:  &mockRepoStore{addErr: errors.New("UNIQUE constraint failed: repositories.full_name")},
			wantStatus: http.StatusConflict,
			wantError:  "repository already exists",
		},
		{
			name:       "invalid JSON",
			body:       `not json`,
			repoStore:  &mockRepoStore{},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid request body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := setupMux(&mockPRStore{}, tt.repoStore)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/repos", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)

			if tt.wantStatus == http.StatusCreated {
				var resp map[string]any
				decodeJSON(t, rec, &resp)
				assert.Equal(t, "owner/repo", resp["full_name"])
				assert.Equal(t, "owner", resp["owner"])
				assert.Equal(t, "repo", resp["name"])
				assert.NotEmpty(t, resp["added_at"])
			}

			if tt.wantError != "" {
				var resp map[string]any
				decodeJSON(t, rec, &resp)
				assert.Equal(t, tt.wantError, resp["error"])
			}
		})
	}
}

func TestRemoveRepo(t *testing.T) {
	tests := []struct {
		name       string
		repoStore  *mockRepoStore
		wantStatus int
		wantError  string
	}{
		{
			name:       "success",
			repoStore:  &mockRepoStore{},
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "not found",
			repoStore:  &mockRepoStore{removeErr: errors.New("not found")},
			wantStatus: http.StatusNotFound,
			wantError:  "repository not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := setupMux(&mockPRStore{}, tt.repoStore)
			req := httptest.NewRequest(http.MethodDelete, "/api/v1/repos/owner/repo", nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)

			if tt.wantStatus == http.StatusNoContent {
				assert.Empty(t, rec.Body.String())
			}

			if tt.wantError != "" {
				var resp map[string]any
				decodeJSON(t, rec, &resp)
				assert.Equal(t, tt.wantError, resp["error"])
			}
		})
	}
}

func TestHealth(t *testing.T) {
	mux := setupMux(&mockPRStore{}, &mockRepoStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	decodeJSON(t, rec, &resp)
	assert.Equal(t, "ok", resp["status"])
	assert.NotEmpty(t, resp["time"])
}

func TestNilLabelsBecomesEmptyArray(t *testing.T) {
	prStore := &mockPRStore{prs: []model.PullRequest{
		{
			Number:       1,
			RepoFullName: "owner/repo",
			Title:        "Test",
			Author:       "alice",
			Status:       model.PRStatusOpen,
			Labels:       nil,
			OpenedAt:     testTime,
			UpdatedAt:    testTime,
		},
	}}
	mux := setupMux(prStore, &mockRepoStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/prs", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Check raw JSON for [] not null
	body := rec.Body.String()
	assert.Contains(t, body, `"labels":[]`)
	assert.Contains(t, body, `"reviews":[]`)
	assert.Contains(t, body, `"issue_comments":[]`)
	assert.NotContains(t, body, `"labels":null`)
	assert.NotContains(t, body, `"reviews":null`)
	assert.NotContains(t, body, `"issue_comments":null`)
}

// --- New Phase 4 Tests ---

func TestGetPR_WithEnrichedReviews(t *testing.T) {
	now := testTime

	prStore := &mockPRStore{pr: &model.PullRequest{
		ID:           1,
		Number:       42,
		RepoFullName: "owner/repo",
		Title:        "Fix bug",
		Author:       "alice",
		Status:       model.PRStatusOpen,
		HeadSHA:      "current-sha",
		URL:          "https://github.com/owner/repo/pull/42",
		Branch:       "fix-bug",
		BaseBranch:   "main",
		OpenedAt:     now,
		UpdatedAt:    now,
	}}

	reviewStore := &mockReviewStore{
		reviews: []model.Review{
			{
				ID:            1001,
				PRID:          1,
				ReviewerLogin: "bob",
				State:         model.ReviewStateApproved,
				Body:          "LGTM",
				CommitID:      "current-sha",
				SubmittedAt:   now,
			},
			{
				ID:            1002,
				PRID:          1,
				ReviewerLogin: "coderabbitai",
				State:         model.ReviewStateCommented,
				Body:          "**Nitpick** minor style issue",
				CommitID:      "old-sha",
				SubmittedAt:   now.Add(-1 * time.Hour),
				IsBot:         true,
			},
		},
		reviewComments: []model.ReviewComment{
			{
				ID:          2001,
				PRID:        1,
				Author:      "bob",
				Body:        "Please fix:\n```suggestion\nreturn nil\n```",
				Path:        "main.go",
				Line:        10,
				StartLine:   8,
				Side:        "RIGHT",
				SubjectType: "line",
				DiffHunk:    "@@ -7,5 +7,5 @@\n old line",
				CommitID:    "current-sha",
				InReplyToID: nil,
				IsResolved:  false,
				CreatedAt:   now,
			},
		},
		issueComments: []model.IssueComment{
			{
				ID:        3001,
				PRID:      1,
				Author:    "charlie",
				Body:      "Great work overall!",
				IsBot:     false,
				CreatedAt: now,
			},
		},
	}

	botConfigStore := &mockBotConfigStore{
		usernames: []string{"coderabbitai", "github-actions[bot]"},
	}

	mux := setupMuxWithReview(prStore, &mockRepoStore{}, botConfigStore, reviewStore)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/repos/owner/repo/prs/42", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	decodeJSON(t, rec, &resp)

	// Basic fields
	assert.Equal(t, float64(42), resp["number"])
	assert.Equal(t, "current-sha", resp["head_sha"])

	// Reviews
	reviews, ok := resp["reviews"].([]any)
	require.True(t, ok)
	require.Len(t, reviews, 2)

	firstReview := reviews[0].(map[string]any)
	assert.Equal(t, "bob", firstReview["reviewer"])
	assert.Equal(t, "approved", firstReview["state"])
	assert.Equal(t, false, firstReview["is_outdated"])
	assert.Equal(t, false, firstReview["is_bot"])

	secondReview := reviews[1].(map[string]any)
	assert.Equal(t, "coderabbitai", secondReview["reviewer"])
	assert.Equal(t, true, secondReview["is_outdated"])
	assert.Equal(t, true, secondReview["is_bot"])
	assert.Equal(t, true, secondReview["is_nitpick"])

	// Threads
	threads, ok := resp["threads"].([]any)
	require.True(t, ok)
	require.Len(t, threads, 1)
	thread := threads[0].(map[string]any)
	rootComment := thread["root_comment"].(map[string]any)
	assert.Equal(t, "main.go", rootComment["file_path"])
	assert.Equal(t, float64(10), rootComment["line"])
	assert.Equal(t, "bob", rootComment["author"])
	assert.Equal(t, float64(1), thread["comment_count"])

	// Suggestions
	suggestions, ok := resp["suggestions"].([]any)
	require.True(t, ok)
	require.Len(t, suggestions, 1)
	sug := suggestions[0].(map[string]any)
	assert.Equal(t, "return nil", sug["proposed_code"])
	assert.Equal(t, "main.go", sug["file_path"])

	// Issue comments
	issueComments, ok := resp["issue_comments"].([]any)
	require.True(t, ok)
	require.Len(t, issueComments, 1)
	ic := issueComments[0].(map[string]any)
	assert.Equal(t, "charlie", ic["author"])
	assert.Equal(t, false, ic["is_bot"])

	// Bot flags
	assert.Equal(t, true, resp["has_bot_review"])
	assert.Equal(t, true, resp["has_coderabbit_review"])
	assert.Equal(t, true, resp["awaiting_coderabbit"])

	// Review status (only bob is human, bob approved)
	assert.Equal(t, "approved", resp["review_status"])
}

func TestGetPR_ReviewServiceError(t *testing.T) {
	prStore := &mockPRStore{pr: &model.PullRequest{
		ID:           1,
		Number:       42,
		RepoFullName: "owner/repo",
		Title:        "Fix bug",
		Author:       "alice",
		Status:       model.PRStatusOpen,
		HeadSHA:      "abc123",
		URL:          "https://github.com/owner/repo/pull/42",
		Branch:       "fix-bug",
		BaseBranch:   "main",
		OpenedAt:     testTime,
		UpdatedAt:    testTime,
	}}

	// errReviewStore causes GetReviewsByPR to error, which means
	// GetPRReviewSummary will return an error.
	errStore := &errReviewStore{}
	botConfigStore := &mockBotConfigStore{usernames: []string{"bot"}}

	mux := setupMuxWithReview(prStore, &mockRepoStore{}, botConfigStore, errStore)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/repos/owner/repo/prs/42", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	// Should still return 200 with basic data -- enrichment failure is non-fatal.
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	decodeJSON(t, rec, &resp)
	assert.Equal(t, float64(42), resp["number"])
	assert.Equal(t, "owner/repo", resp["repository"])

	// Enriched fields should be empty defaults.
	reviews, ok := resp["reviews"].([]any)
	require.True(t, ok)
	assert.Len(t, reviews, 0)
}

func TestGetPR_CFMT05_InlineVsGeneralSeparation(t *testing.T) {
	now := testTime

	prStore := &mockPRStore{pr: &model.PullRequest{
		ID:           1,
		Number:       42,
		RepoFullName: "owner/repo",
		Title:        "Feature PR",
		Author:       "alice",
		Status:       model.PRStatusOpen,
		HeadSHA:      "sha-123",
		OpenedAt:     now,
		UpdatedAt:    now,
	}}

	// 2 inline threads + 1 general issue comment.
	reviewStore := &mockReviewStore{
		reviewComments: []model.ReviewComment{
			{
				ID:          100,
				PRID:        1,
				Author:      "reviewer1",
				Body:        "Inline comment on code",
				Path:        "service.go",
				Line:        15,
				Side:        "RIGHT",
				SubjectType: "line",
				DiffHunk:    "@@ -10,5 +10,5 @@",
				InReplyToID: nil,
				CreatedAt:   now,
			},
			{
				ID:          200,
				PRID:        1,
				Author:      "reviewer2",
				Body:        "Another inline comment",
				Path:        "handler.go",
				Line:        30,
				Side:        "RIGHT",
				SubjectType: "line",
				DiffHunk:    "@@ -25,5 +25,5 @@",
				InReplyToID: nil,
				CreatedAt:   now.Add(1 * time.Minute),
			},
		},
		issueComments: []model.IssueComment{
			{
				ID:        300,
				PRID:      1,
				Author:    "reviewer3",
				Body:      "General discussion comment",
				CreatedAt: now.Add(2 * time.Minute),
			},
		},
	}

	botConfigStore := &mockBotConfigStore{usernames: []string{}}

	mux := setupMuxWithReview(prStore, &mockRepoStore{}, botConfigStore, reviewStore)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/repos/owner/repo/prs/42", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Capture raw body before decoding (decoding drains the buffer).
	body := rec.Body.String()

	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &resp))

	// Threads: 2 inline code comment threads.
	threads, ok := resp["threads"].([]any)
	require.True(t, ok)
	require.Len(t, threads, 2, "should have 2 inline thread entries")

	// Each thread has root_comment with file_path proving they are inline/code comments.
	for i, threadAny := range threads {
		thread := threadAny.(map[string]any)
		root := thread["root_comment"].(map[string]any)
		assert.NotEmpty(t, root["file_path"], "thread %d root_comment should have file_path", i)
	}

	// Issue comments: 1 general PR-level comment.
	issueComments, ok := resp["issue_comments"].([]any)
	require.True(t, ok)
	require.Len(t, issueComments, 1, "should have 1 general issue comment")

	// Issue comments have NO file_path field (it is a separate struct with no file_path).
	ic := issueComments[0].(map[string]any)
	_, hasFilePath := ic["file_path"]
	assert.False(t, hasFilePath, "issue_comments should not have file_path field")

	// Verify the two JSON keys are distinct.
	assert.Contains(t, body, `"threads"`)
	assert.Contains(t, body, `"issue_comments"`)
}

func TestListBots(t *testing.T) {
	botStore := &mockBotConfigStore{
		bots: []model.BotConfig{
			{ID: 1, Username: "coderabbitai", AddedAt: testTime},
			{ID: 2, Username: "github-actions[bot]", AddedAt: testTime},
			{ID: 3, Username: "copilot[bot]", AddedAt: testTime},
		},
	}

	mux := setupMuxWithBots(botStore)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bots", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp []map[string]any
	decodeJSON(t, rec, &resp)
	require.Len(t, resp, 3)
	assert.Equal(t, "coderabbitai", resp[0]["username"])
	assert.Equal(t, testTimeStr, resp[0]["added_at"])
	assert.Equal(t, "github-actions[bot]", resp[1]["username"])
	assert.Equal(t, "copilot[bot]", resp[2]["username"])
}

func TestAddBot(t *testing.T) {
	botStore := &mockBotConfigStore{}

	mux := setupMuxWithBots(botStore)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bots", strings.NewReader(`{"username":"newbot"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]any
	decodeJSON(t, rec, &resp)
	assert.Equal(t, "newbot", resp["username"])
	assert.NotEmpty(t, resp["added_at"])
}

func TestAddBot_Duplicate(t *testing.T) {
	botStore := &mockBotConfigStore{
		addErr: errors.New("UNIQUE constraint failed: bot_config.username"),
	}

	mux := setupMuxWithBots(botStore)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bots", strings.NewReader(`{"username":"existing"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)

	var resp map[string]any
	decodeJSON(t, rec, &resp)
	assert.Equal(t, "bot username already exists", resp["error"])
}

func TestRemoveBot(t *testing.T) {
	botStore := &mockBotConfigStore{}

	mux := setupMuxWithBots(botStore)
	// URL-encoded brackets: copilot%5Bbot%5D -> copilot[bot]
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/bots/copilot%5Bbot%5D", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}

// --- Phase 5 Health Signal Tests ---

func TestGetPR_WithHealthSignals(t *testing.T) {
	now := testTime

	prStore := &mockPRStore{pr: &model.PullRequest{
		ID:              1,
		Number:          42,
		RepoFullName:    "owner/repo",
		Title:           "Fix bug",
		Author:          "alice",
		Status:          model.PRStatusOpen,
		HeadSHA:         "abc123",
		URL:             "https://github.com/owner/repo/pull/42",
		Branch:          "fix-bug",
		BaseBranch:      "main",
		Additions:       100,
		Deletions:       50,
		ChangedFiles:    8,
		MergeableStatus: model.MergeableMergeable,
		CIStatus:        model.CIStatusPassing,
		OpenedAt:        now,
		UpdatedAt:       now,
		LastActivityAt:  now,
	}}

	checkStore := &mockCheckStore{
		checkRuns: []model.CheckRun{
			{
				ID:         5001,
				PRID:       1,
				Name:       "build",
				Status:     "completed",
				Conclusion: "success",
				IsRequired: true,
				DetailsURL: "https://github.com/owner/repo/actions/runs/5001",
			},
			{
				ID:         5002,
				PRID:       1,
				Name:       "lint",
				Status:     "completed",
				Conclusion: "success",
				IsRequired: false,
				DetailsURL: "https://github.com/owner/repo/actions/runs/5002",
			},
		},
	}

	mux := setupMuxWithHealth(prStore, &mockRepoStore{}, checkStore)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/repos/owner/repo/prs/42", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	decodeJSON(t, rec, &resp)

	// Basic fields
	assert.Equal(t, float64(42), resp["number"])

	// Health signal fields from PR model
	assert.Equal(t, float64(100), resp["additions"])
	assert.Equal(t, float64(50), resp["deletions"])
	assert.Equal(t, float64(8), resp["changed_files"])
	assert.Equal(t, "mergeable", resp["mergeable_status"])

	// CI status should be overwritten by HealthService computation
	assert.Equal(t, "passing", resp["ci_status"])

	// Check runs enriched from HealthService
	checkRuns, ok := resp["check_runs"].([]any)
	require.True(t, ok)
	require.Len(t, checkRuns, 2)

	firstCheck := checkRuns[0].(map[string]any)
	assert.Equal(t, float64(5001), firstCheck["id"])
	assert.Equal(t, "build", firstCheck["name"])
	assert.Equal(t, "completed", firstCheck["status"])
	assert.Equal(t, "success", firstCheck["conclusion"])
	assert.Equal(t, true, firstCheck["is_required"])
	assert.Equal(t, "https://github.com/owner/repo/actions/runs/5001", firstCheck["details_url"])

	secondCheck := checkRuns[1].(map[string]any)
	assert.Equal(t, "lint", secondCheck["name"])
	assert.Equal(t, false, secondCheck["is_required"])
}

func TestListPRs_IncludesHealthFields(t *testing.T) {
	now := testTime

	prStore := &mockPRStore{prs: []model.PullRequest{
		{
			Number:          42,
			RepoFullName:    "owner/repo",
			Title:           "Fix bug",
			Author:          "alice",
			Status:          model.PRStatusOpen,
			Additions:       25,
			Deletions:       10,
			ChangedFiles:    3,
			MergeableStatus: model.MergeableConflicted,
			CIStatus:        model.CIStatusFailing,
			OpenedAt:        now,
			UpdatedAt:       now,
			LastActivityAt:  now,
		},
	}}

	mux := setupMux(prStore, &mockRepoStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/prs", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp []map[string]any
	decodeJSON(t, rec, &resp)
	require.Len(t, resp, 1)

	pr := resp[0]
	assert.Equal(t, float64(25), pr["additions"])
	assert.Equal(t, float64(10), pr["deletions"])
	assert.Equal(t, float64(3), pr["changed_files"])
	assert.Equal(t, "conflicted", pr["mergeable_status"])
	assert.Equal(t, "failing", pr["ci_status"])

	// days_since_opened and days_since_last_activity should be present (numeric)
	_, hasDaysOpened := pr["days_since_opened"]
	assert.True(t, hasDaysOpened, "should have days_since_opened field")
	_, hasDaysActivity := pr["days_since_last_activity"]
	assert.True(t, hasDaysActivity, "should have days_since_last_activity field")

	// check_runs should be an empty array on list endpoints
	checkRuns, ok := pr["check_runs"].([]any)
	require.True(t, ok, "check_runs should be an array")
	assert.Len(t, checkRuns, 0, "check_runs should be empty on list endpoint")
}
