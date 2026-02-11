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

	httphandler "github.com/efisher/reviewhub/internal/adapter/driving/http"
	"github.com/efisher/reviewhub/internal/domain/model"
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

// --- Test helpers ---

var (
	testTime    = time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	testTimeStr = "2026-02-10T12:00:00Z"
)

func setupMux(prStore *mockPRStore, repoStore *mockRepoStore) http.Handler {
	h := httphandler.NewHandler(prStore, repoStore, nil, "testuser", slog.Default())
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
				// Reviews and Comments are empty arrays, not null
				reviews, ok := pr["reviews"].([]any)
				require.True(t, ok)
				assert.Len(t, reviews, 0)
				comments, ok := pr["comments"].([]any)
				require.True(t, ok)
				assert.Len(t, comments, 0)
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
	assert.Contains(t, body, `"comments":[]`)
	assert.NotContains(t, body, `"labels":null`)
	assert.NotContains(t, body, `"reviews":null`)
	assert.NotContains(t, body, `"comments":null`)
}
