package github_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	ghAdapter "github.com/ericfisherdev/mygitpanel/internal/adapter/driven/github"
	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
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
		"test-token",
	)
	require.NoError(t, err)

	return client, server
}

// prJSON is a helper struct for building GitHub API pull request responses.
type prJSON struct {
	Number   int       `json:"number"`
	Title    string    `json:"title"`
	State    string    `json:"state"`
	Draft    bool      `json:"draft"`
	HTMLURL  string    `json:"html_url"`
	User     userJSON  `json:"user"`
	Head     refJSON   `json:"head"`
	Base     refJSON   `json:"base"`
	Labels   []lblJSON `json:"labels"`
	Created  string    `json:"created_at"`
	Updated  string    `json:"updated_at"`
	MergedAt *string   `json:"merged_at,omitempty"`
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

// --- FetchCheckRuns tests ---

func TestFetchCheckRuns(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"total_count": 2,
			"check_runs": []map[string]any{
				{
					"id":           int64(5001),
					"name":         "build",
					"status":       "completed",
					"conclusion":   "success",
					"details_url":  "https://github.com/owner/repo/actions/runs/123",
					"started_at":   "2026-01-10T10:00:00Z",
					"completed_at": "2026-01-10T10:05:00Z",
				},
				{
					"id":          int64(5002),
					"name":        "lint",
					"status":      "in_progress",
					"conclusion":  nil,
					"details_url": "https://github.com/owner/repo/actions/runs/124",
					"started_at":  "2026-01-10T10:01:00Z",
				},
			},
		})
	})

	client, _ := newTestClient(t, handler)
	result, err := client.FetchCheckRuns(context.Background(), "owner/repo", "abc123")

	require.NoError(t, err)
	require.Len(t, result, 2)

	// First check run: completed success
	assert.Equal(t, int64(5001), result[0].ID)
	assert.Equal(t, int64(0), result[0].PRID, "PRID should be 0 (caller assigns)")
	assert.Equal(t, "build", result[0].Name)
	assert.Equal(t, "completed", result[0].Status)
	assert.Equal(t, "success", result[0].Conclusion)
	assert.False(t, result[0].IsRequired, "IsRequired defaults to false")
	assert.Equal(t, "https://github.com/owner/repo/actions/runs/123", result[0].DetailsURL)
	assert.False(t, result[0].StartedAt.IsZero())
	assert.False(t, result[0].CompletedAt.IsZero())

	// Second check run: in progress
	assert.Equal(t, int64(5002), result[1].ID)
	assert.Equal(t, "lint", result[1].Name)
	assert.Equal(t, "in_progress", result[1].Status)
	assert.Equal(t, "", result[1].Conclusion)
	assert.False(t, result[1].StartedAt.IsZero())
	assert.True(t, result[1].CompletedAt.IsZero(), "in-progress check should have zero CompletedAt")
}

// --- FetchCombinedStatus tests ---

func TestFetchCombinedStatus(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"state":       "success",
			"total_count": 1,
			"statuses": []map[string]any{
				{
					"context":     "ci/circleci",
					"state":       "success",
					"description": "Build passed",
					"target_url":  "https://circleci.com/build/123",
				},
			},
		})
	})

	client, _ := newTestClient(t, handler)
	result, err := client.FetchCombinedStatus(context.Background(), "owner/repo", "abc123")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "success", result.State)
	require.Len(t, result.Statuses, 1)
	assert.Equal(t, "ci/circleci", result.Statuses[0].Context)
	assert.Equal(t, "success", result.Statuses[0].State)
	assert.Equal(t, "Build passed", result.Statuses[0].Description)
	assert.Equal(t, "https://circleci.com/build/123", result.Statuses[0].TargetURL)
}

// --- FetchPRDetail tests ---

func TestFetchPRDetail(t *testing.T) {
	mergeable := true
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"number":        42,
			"additions":     150,
			"deletions":     30,
			"changed_files": 8,
			"mergeable":     mergeable,
			"state":         "open",
			"user":          map[string]any{"login": "alice"},
			"head":          map[string]any{"ref": "feature", "sha": "abc123"},
			"base":          map[string]any{"ref": "main"},
			"created_at":    "2026-01-01T00:00:00Z",
			"updated_at":    "2026-01-02T00:00:00Z",
		})
	})

	client, _ := newTestClient(t, handler)
	result, err := client.FetchPRDetail(context.Background(), "owner/repo", 42)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 150, result.Additions)
	assert.Equal(t, 30, result.Deletions)
	assert.Equal(t, 8, result.ChangedFiles)
	assert.Equal(t, model.MergeableMergeable, result.Mergeable)
}

func TestFetchPRDetail_MergeableNull(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Omit "mergeable" field entirely to simulate null.
		json.NewEncoder(w).Encode(map[string]any{
			"number":        42,
			"additions":     10,
			"deletions":     5,
			"changed_files": 2,
			"state":         "open",
			"user":          map[string]any{"login": "alice"},
			"head":          map[string]any{"ref": "feature", "sha": "abc123"},
			"base":          map[string]any{"ref": "main"},
			"created_at":    "2026-01-01T00:00:00Z",
			"updated_at":    "2026-01-02T00:00:00Z",
		})
	})

	client, _ := newTestClient(t, handler)
	result, err := client.FetchPRDetail(context.Background(), "owner/repo", 42)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, model.MergeableUnknown, result.Mergeable, "null mergeable should map to MergeableUnknown")
}

// --- FetchRequiredStatusChecks tests ---

func TestFetchRequiredStatusChecks_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"strict": true,
			"checks": []map[string]any{
				{"context": "build"},
				{"context": "lint"},
			},
		})
	})

	client, _ := newTestClient(t, handler)
	result, err := client.FetchRequiredStatusChecks(context.Background(), "owner/repo", "main")

	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "build", result[0])
	assert.Equal(t, "lint", result[1])
}

func TestFetchRequiredStatusChecks_404(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"message": "Branch not protected",
		})
	})

	client, _ := newTestClient(t, handler)
	result, err := client.FetchRequiredStatusChecks(context.Background(), "owner/repo", "main")

	require.NoError(t, err, "404 should not return an error")
	assert.Nil(t, result, "404 should return nil slice")
}

func TestFetchRequiredStatusChecks_403(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]any{
			"message": "Insufficient permissions",
		})
	})

	client, _ := newTestClient(t, handler)
	result, err := client.FetchRequiredStatusChecks(context.Background(), "owner/repo", "main")

	require.NoError(t, err, "403 should not return an error")
	assert.Nil(t, result, "403 should return nil slice")
}
