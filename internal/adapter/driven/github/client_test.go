package github_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	ghAdapter "github.com/efisher/reviewhub/internal/adapter/driven/github"
	"github.com/efisher/reviewhub/internal/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient creates a Client backed by the given httptest handler.
func newTestClient(t *testing.T, handler http.Handler) (*ghAdapter.Client, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := ghAdapter.NewClientWithHTTPClient(
		server.Client(),
		server.URL+"/",
		"testuser",
	)
	require.NoError(t, err)

	return client, server
}

// prJSON is a helper struct for building GitHub API pull request responses.
type prJSON struct {
	Number   int        `json:"number"`
	Title    string     `json:"title"`
	State    string     `json:"state"`
	Draft    bool       `json:"draft"`
	HTMLURL  string     `json:"html_url"`
	User     userJSON   `json:"user"`
	Head     refJSON    `json:"head"`
	Base     refJSON    `json:"base"`
	Labels   []lblJSON  `json:"labels"`
	Created  string     `json:"created_at"`
	Updated  string     `json:"updated_at"`
	MergedAt *string    `json:"merged_at,omitempty"`
}

type userJSON struct {
	Login string `json:"login"`
}

type refJSON struct {
	Ref string `json:"ref"`
	SHA string `json:"sha,omitempty"`
}

type lblJSON struct {
	Name string `json:"name"`
}

func TestFetchPullRequests_SinglePage(t *testing.T) {
	prs := []prJSON{
		{
			Number:  42,
			Title:   "Add feature X",
			State:   "open",
			Draft:   false,
			HTMLURL: "https://github.com/owner/repo/pull/42",
			User:    userJSON{Login: "alice"},
			Head:    refJSON{Ref: "feature-x"},
			Base:    refJSON{Ref: "main"},
			Labels:  []lblJSON{{Name: "enhancement"}, {Name: "priority:high"}},
			Created: "2026-01-01T00:00:00Z",
			Updated: "2026-01-02T12:00:00Z",
		},
		{
			Number:  43,
			Title:   "Fix bug Y",
			State:   "open",
			Draft:   false,
			HTMLURL: "https://github.com/owner/repo/pull/43",
			User:    userJSON{Login: "bob"},
			Head:    refJSON{Ref: "fix-bug-y"},
			Base:    refJSON{Ref: "develop"},
			Labels:  []lblJSON{},
			Created: "2026-01-03T00:00:00Z",
			Updated: "2026-01-04T00:00:00Z",
		},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(prs)
	})

	client, _ := newTestClient(t, handler)
	result, err := client.FetchPullRequests(context.Background(), "owner/repo")

	require.NoError(t, err)
	require.Len(t, result, 2)

	// Verify first PR mapping
	assert.Equal(t, 42, result[0].Number)
	assert.Equal(t, "owner/repo", result[0].RepoFullName)
	assert.Equal(t, "Add feature X", result[0].Title)
	assert.Equal(t, "alice", result[0].Author)
	assert.Equal(t, model.PRStatusOpen, result[0].Status)
	assert.False(t, result[0].IsDraft)
	assert.Equal(t, "https://github.com/owner/repo/pull/42", result[0].URL)
	assert.Equal(t, "feature-x", result[0].Branch)
	assert.Equal(t, "main", result[0].BaseBranch)
	assert.Equal(t, []string{"enhancement", "priority:high"}, result[0].Labels)

	// Verify second PR mapping
	assert.Equal(t, 43, result[1].Number)
	assert.Equal(t, "bob", result[1].Author)
	assert.Equal(t, "fix-bug-y", result[1].Branch)
	assert.Equal(t, "develop", result[1].BaseBranch)
	assert.Equal(t, []string{}, result[1].Labels)
}

func TestFetchPullRequests_Pagination(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")

		w.Header().Set("Content-Type", "application/json")

		if page == "" || page == "1" {
			// Page 1: include Link header pointing to page 2
			w.Header().Set("Link", fmt.Sprintf(`<%s?page=2>; rel="next"`, "http://"+r.Host+r.URL.Path))
			json.NewEncoder(w).Encode([]prJSON{
				{
					Number:  1,
					Title:   "PR One",
					State:   "open",
					User:    userJSON{Login: "dev1"},
					Head:    refJSON{Ref: "branch-1"},
					Base:    refJSON{Ref: "main"},
					Labels:  []lblJSON{},
					Created: "2026-01-01T00:00:00Z",
					Updated: "2026-01-01T00:00:00Z",
				},
			})
		} else {
			// Page 2: no Link header (last page)
			json.NewEncoder(w).Encode([]prJSON{
				{
					Number:  2,
					Title:   "PR Two",
					State:   "open",
					User:    userJSON{Login: "dev2"},
					Head:    refJSON{Ref: "branch-2"},
					Base:    refJSON{Ref: "main"},
					Labels:  []lblJSON{},
					Created: "2026-01-02T00:00:00Z",
					Updated: "2026-01-02T00:00:00Z",
				},
			})
		}
	})

	client, _ := newTestClient(t, handler)
	result, err := client.FetchPullRequests(context.Background(), "owner/repo")

	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, 1, result[0].Number)
	assert.Equal(t, "PR One", result[0].Title)
	assert.Equal(t, 2, result[1].Number)
	assert.Equal(t, "PR Two", result[1].Title)
}

func TestFetchPullRequests_DraftDetection(t *testing.T) {
	prs := []prJSON{
		{
			Number:  10,
			Title:   "Draft PR",
			State:   "open",
			Draft:   true,
			User:    userJSON{Login: "dev"},
			Head:    refJSON{Ref: "wip"},
			Base:    refJSON{Ref: "main"},
			Labels:  []lblJSON{},
			Created: "2026-01-01T00:00:00Z",
			Updated: "2026-01-01T00:00:00Z",
		},
		{
			Number:  11,
			Title:   "Ready PR",
			State:   "open",
			Draft:   false,
			User:    userJSON{Login: "dev"},
			Head:    refJSON{Ref: "ready"},
			Base:    refJSON{Ref: "main"},
			Labels:  []lblJSON{},
			Created: "2026-01-01T00:00:00Z",
			Updated: "2026-01-01T00:00:00Z",
		},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(prs)
	})

	client, _ := newTestClient(t, handler)
	result, err := client.FetchPullRequests(context.Background(), "owner/repo")

	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.True(t, result[0].IsDraft, "first PR should be draft")
	assert.False(t, result[1].IsDraft, "second PR should not be draft")
}

func TestFetchPullRequests_EmptyRepo(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]prJSON{})
	})

	client, _ := newTestClient(t, handler)
	result, err := client.FetchPullRequests(context.Background(), "owner/repo")

	require.NoError(t, err)
	assert.NotNil(t, result, "should return empty slice, not nil")
	assert.Empty(t, result)
}

func TestFetchPullRequests_InvalidRepoName(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called for invalid repo name")
	})

	client, _ := newTestClient(t, handler)

	tests := []struct {
		name string
		repo string
	}{
		{name: "no slash", repo: "invalid"},
		{name: "empty owner", repo: "/repo"},
		{name: "empty repo", repo: "owner/"},
		{name: "empty string", repo: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.FetchPullRequests(context.Background(), tc.repo)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid repo name")
		})
	}
}

func TestFetchPullRequests_HeadSHA(t *testing.T) {
	prs := []prJSON{
		{
			Number:  42,
			Title:   "Feature PR",
			State:   "open",
			User:    userJSON{Login: "alice"},
			Head:    refJSON{Ref: "feature-x", SHA: "abc123def456"},
			Base:    refJSON{Ref: "main"},
			Labels:  []lblJSON{},
			Created: "2026-01-01T00:00:00Z",
			Updated: "2026-01-02T12:00:00Z",
		},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(prs)
	})

	client, _ := newTestClient(t, handler)
	result, err := client.FetchPullRequests(context.Background(), "owner/repo")

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "abc123def456", result[0].HeadSHA, "HeadSHA should be populated from head.sha")
}

func TestFetchReviews(t *testing.T) {
	reviews := []map[string]any{
		{
			"id":           int64(1001),
			"state":        "APPROVED",
			"body":         "LGTM!",
			"commit_id":    "abc123",
			"submitted_at": "2026-01-10T10:00:00Z",
			"user":         map[string]any{"login": "alice"},
		},
		{
			"id":           int64(1002),
			"state":        "CHANGES_REQUESTED",
			"body":         "Please fix the error handling.",
			"commit_id":    "def456",
			"submitted_at": "2026-01-11T11:00:00Z",
			"user":         map[string]any{"login": "bob"},
		},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(reviews)
	})

	client, _ := newTestClient(t, handler)
	result, err := client.FetchReviews(context.Background(), "owner/repo", 42)

	require.NoError(t, err)
	require.Len(t, result, 2)

	// First review: approved
	assert.Equal(t, int64(1001), result[0].ID)
	assert.Equal(t, int64(0), result[0].PRID, "PRID should be 0 (caller assigns)")
	assert.Equal(t, "alice", result[0].ReviewerLogin)
	assert.Equal(t, model.ReviewState("approved"), result[0].State)
	assert.Equal(t, "LGTM!", result[0].Body)
	assert.Equal(t, "abc123", result[0].CommitID)
	assert.False(t, result[0].IsBot)

	// Second review: changes_requested
	assert.Equal(t, int64(1002), result[1].ID)
	assert.Equal(t, "bob", result[1].ReviewerLogin)
	assert.Equal(t, model.ReviewState("changes_requested"), result[1].State)
	assert.Equal(t, "def456", result[1].CommitID)
}

func TestFetchReviewComments(t *testing.T) {
	comments := []map[string]any{
		{
			"id":                     int64(2001),
			"pull_request_review_id": int64(1001),
			"body":                   "This looks wrong.",
			"path":                   "main.go",
			"line":                   42,
			"start_line":             40,
			"side":                   "RIGHT",
			"subject_type":           "line",
			"diff_hunk":              "@@ -38,7 +38,7 @@\n context line\n-old line\n+new line",
			"commit_id":              "abc123",
			"created_at":             "2026-01-10T10:00:00Z",
			"updated_at":             "2026-01-10T10:00:00Z",
			"user":                   map[string]any{"login": "alice"},
		},
		{
			"id":                     int64(2002),
			"pull_request_review_id": int64(1001),
			"body":                   "Good point, I agree.",
			"path":                   "main.go",
			"line":                   42,
			"side":                   "RIGHT",
			"subject_type":           "line",
			"diff_hunk":              "@@ -38,7 +38,7 @@\n context line",
			"commit_id":              "abc123",
			"in_reply_to_id":         int64(2001),
			"created_at":             "2026-01-10T11:00:00Z",
			"updated_at":             "2026-01-10T11:00:00Z",
			"user":                   map[string]any{"login": "bob"},
		},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(comments)
	})

	client, _ := newTestClient(t, handler)
	result, err := client.FetchReviewComments(context.Background(), "owner/repo", 42)

	require.NoError(t, err)
	require.Len(t, result, 2)

	// Root comment
	assert.Equal(t, int64(2001), result[0].ID)
	assert.Equal(t, int64(1001), result[0].ReviewID)
	assert.Equal(t, int64(0), result[0].PRID, "PRID should be 0 (caller assigns)")
	assert.Equal(t, "alice", result[0].Author)
	assert.Equal(t, "This looks wrong.", result[0].Body)
	assert.Equal(t, "main.go", result[0].Path)
	assert.Equal(t, 42, result[0].Line)
	assert.Equal(t, 40, result[0].StartLine)
	assert.Equal(t, "line", result[0].SubjectType)
	assert.Contains(t, result[0].DiffHunk, "@@ -38,7 +38,7 @@")
	assert.Equal(t, "abc123", result[0].CommitID)
	assert.Nil(t, result[0].InReplyToID, "root comment should have nil InReplyToID")
	assert.False(t, result[0].IsResolved)
	assert.False(t, result[0].IsOutdated)

	// Reply comment
	assert.Equal(t, int64(2002), result[1].ID)
	assert.Equal(t, "bob", result[1].Author)
	require.NotNil(t, result[1].InReplyToID, "reply should have non-nil InReplyToID")
	assert.Equal(t, int64(2001), *result[1].InReplyToID)
}

func TestFetchIssueComments(t *testing.T) {
	comments := []map[string]any{
		{
			"id":         int64(3001),
			"body":       "Great work on this PR!",
			"created_at": "2026-01-10T10:00:00Z",
			"updated_at": "2026-01-10T10:00:00Z",
			"user":       map[string]any{"login": "charlie"},
		},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(comments)
	})

	client, _ := newTestClient(t, handler)
	result, err := client.FetchIssueComments(context.Background(), "owner/repo", 42)

	require.NoError(t, err)
	require.Len(t, result, 1)

	assert.Equal(t, int64(3001), result[0].ID)
	assert.Equal(t, int64(0), result[0].PRID, "PRID should be 0 (caller assigns)")
	assert.Equal(t, "charlie", result[0].Author)
	assert.Equal(t, "Great work on this PR!", result[0].Body)
	assert.False(t, result[0].IsBot)
	assert.False(t, result[0].CreatedAt.IsZero())
	assert.False(t, result[0].UpdatedAt.IsZero())
}

func TestFetchPullRequests_StatusMapping(t *testing.T) {
	mergedAt := "2026-01-05T00:00:00Z"

	prs := []prJSON{
		{
			Number:  1,
			Title:   "Open PR",
			State:   "open",
			User:    userJSON{Login: "dev"},
			Head:    refJSON{Ref: "open-branch"},
			Base:    refJSON{Ref: "main"},
			Labels:  []lblJSON{},
			Created: "2026-01-01T00:00:00Z",
			Updated: "2026-01-01T00:00:00Z",
		},
		{
			Number:  2,
			Title:   "Closed PR",
			State:   "closed",
			User:    userJSON{Login: "dev"},
			Head:    refJSON{Ref: "closed-branch"},
			Base:    refJSON{Ref: "main"},
			Labels:  []lblJSON{},
			Created: "2026-01-02T00:00:00Z",
			Updated: "2026-01-02T00:00:00Z",
		},
		{
			Number:   3,
			Title:    "Merged PR",
			State:    "closed",
			User:     userJSON{Login: "dev"},
			Head:     refJSON{Ref: "merged-branch"},
			Base:     refJSON{Ref: "main"},
			Labels:   []lblJSON{},
			Created:  "2026-01-03T00:00:00Z",
			Updated:  "2026-01-03T00:00:00Z",
			MergedAt: &mergedAt,
		},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(prs)
	})

	client, _ := newTestClient(t, handler)
	result, err := client.FetchPullRequests(context.Background(), "owner/repo")

	require.NoError(t, err)
	require.Len(t, result, 3)

	assert.Equal(t, model.PRStatusOpen, result[0].Status, "open PR should have Open status")
	assert.Equal(t, model.PRStatusClosed, result[1].Status, "closed PR should have Closed status")
	assert.Equal(t, model.PRStatusMerged, result[2].Status, "merged PR should have Merged status")
}
