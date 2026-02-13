package github_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	ghAdapter "github.com/ericfisherdev/mygitpanel/internal/adapter/driven/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchThreadResolution_Success(t *testing.T) {
	gqlResponse := map[string]any{
		"data": map[string]any{
			"repository": map[string]any{
				"pullRequest": map[string]any{
					"reviewThreads": map[string]any{
						"pageInfo": map[string]any{
							"hasNextPage": false,
						},
						"nodes": []any{
							map[string]any{
								"isResolved": true,
								"comments": map[string]any{
									"nodes": []any{
										map[string]any{"databaseId": 2001},
									},
								},
							},
							map[string]any{
								"isResolved": false,
								"comments": map[string]any{
									"nodes": []any{
										map[string]any{"databaseId": 2002},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/graphql" {
			assert.Equal(t, "bearer test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(gqlResponse)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, err := ghAdapter.NewClientWithHTTPClient(
		server.Client(),
		server.URL+"/",
		"testuser",
		"test-token",
	)
	require.NoError(t, err)

	result, err := client.FetchThreadResolution(context.Background(), "owner/repo", 42)
	require.NoError(t, err)

	require.Len(t, result, 2)
	assert.True(t, result[2001], "comment 2001 should be resolved")
	assert.False(t, result[2002], "comment 2002 should be unresolved")
}

func TestFetchThreadResolution_GraphQLErrors(t *testing.T) {
	gqlResponse := map[string]any{
		"data":   nil,
		"errors": []any{map[string]any{"message": "Something went wrong"}},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(gqlResponse)
	}))
	defer server.Close()

	client, err := ghAdapter.NewClientWithHTTPClient(
		server.Client(),
		server.URL+"/",
		"testuser",
		"test-token",
	)
	require.NoError(t, err)

	result, err := client.FetchThreadResolution(context.Background(), "owner/repo", 42)
	require.NoError(t, err)
	assert.Empty(t, result, "GraphQL errors should return empty map")
}

func TestFetchThreadResolution_NoToken(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, err := ghAdapter.NewClientWithHTTPClient(
		server.Client(),
		server.URL+"/",
		"testuser",
		"", // empty token
	)
	require.NoError(t, err)

	result, err := client.FetchThreadResolution(context.Background(), "owner/repo", 42)
	require.NoError(t, err)
	assert.Empty(t, result, "no-token should return empty map immediately")
	assert.False(t, called, "no HTTP call should be made when token is empty")
}

func TestFetchThreadResolution_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := ghAdapter.NewClientWithHTTPClient(
		server.Client(),
		server.URL+"/",
		"testuser",
		"test-token",
	)
	require.NoError(t, err)

	result, err := client.FetchThreadResolution(context.Background(), "owner/repo", 42)
	require.NoError(t, err)
	assert.Empty(t, result, "HTTP 500 should return empty map")
}
