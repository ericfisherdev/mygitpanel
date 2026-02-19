package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// graphqlHTTPClient is the HTTP client used for GraphQL requests.
// It enforces a 30-second timeout as a safety net alongside context cancellation.
var graphqlHTTPClient = &http.Client{Timeout: 30 * time.Second}

const markReadyMutation = `mutation($id: ID!) {
	markPullRequestReadyForReview(input: {pullRequestId: $id}) {
		pullRequest { isDraft }
	}
}`

const convertToDraftMutation = `mutation($id: ID!) {
	convertPullRequestToDraft(input: {pullRequestId: $id}) {
		pullRequest { isDraft }
	}
}`

const threadResolutionQuery = `query($owner: String!, $repo: String!, $pr: Int!) {
	repository(owner: $owner, name: $repo) {
		pullRequest(number: $pr) {
			reviewThreads(first: 100) {
				pageInfo {
					hasNextPage
				}
				nodes {
					isResolved
					comments(first: 1) {
						nodes {
							databaseId
						}
					}
				}
			}
		}
	}
}`

// graphqlRequest is the JSON body sent to the GitHub GraphQL API.
type graphqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

// graphqlResponse represents the expected shape of a GitHub GraphQL response
// for thread resolution status.
type graphqlResponse struct {
	Data struct {
		Repository struct {
			PullRequest struct {
				ReviewThreads struct {
					PageInfo struct {
						HasNextPage bool `json:"hasNextPage"`
					} `json:"pageInfo"`
					Nodes []struct {
						IsResolved bool `json:"isResolved"`
						Comments   struct {
							Nodes []struct {
								DatabaseID int64 `json:"databaseId"`
							} `json:"nodes"`
						} `json:"comments"`
					} `json:"nodes"`
				} `json:"reviewThreads"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// FetchThreadResolution queries the GitHub GraphQL API for review thread resolution status.
// It returns a map of review comment database ID to its resolved status (true = resolved).
//
// This is a supplementary data source. All error paths return an empty map and log a warning;
// failures never propagate to callers.
func (c *Client) FetchThreadResolution(ctx context.Context, repoFullName string, prNumber int) (map[int64]bool, error) {
	if c.token == "" {
		return map[int64]bool{}, nil
	}

	owner, repo, err := splitRepo(repoFullName)
	if err != nil {
		return map[int64]bool{}, nil
	}

	reqBody := graphqlRequest{
		Query: threadResolutionQuery,
		Variables: map[string]any{
			"owner": owner,
			"repo":  repo,
			"pr":    prNumber,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		slog.Warn("graphql: failed to marshal request", "error", err)
		return map[int64]bool{}, nil
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.graphqlURL, bytes.NewReader(bodyBytes))
	if err != nil {
		slog.Warn("graphql: failed to create request", "error", err)
		return map[int64]bool{}, nil
	}
	httpReq.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.token))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := graphqlHTTPClient.Do(httpReq)
	if err != nil {
		slog.Warn("graphql: request failed", "error", err, "repo", repoFullName, "pr", prNumber)
		return map[int64]bool{}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("graphql: non-200 response", "status", resp.StatusCode, "repo", repoFullName, "pr", prNumber)
		return map[int64]bool{}, nil
	}

	var gqlResp graphqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		slog.Warn("graphql: failed to decode response", "error", err, "repo", repoFullName, "pr", prNumber)
		return map[int64]bool{}, nil
	}

	if len(gqlResp.Errors) > 0 {
		slog.Warn("graphql: response contains errors",
			"errors", gqlResp.Errors[0].Message,
			"repo", repoFullName,
			"pr", prNumber,
		)
		return map[int64]bool{}, nil
	}

	threads := gqlResp.Data.Repository.PullRequest.ReviewThreads

	if threads.PageInfo.HasNextPage {
		slog.Warn("graphql: review threads exceed 100, pagination needed",
			"repo", repoFullName,
			"pr", prNumber,
		)
	}

	result := make(map[int64]bool, len(threads.Nodes))
	for _, thread := range threads.Nodes {
		if len(thread.Comments.Nodes) > 0 && thread.Comments.Nodes[0].DatabaseID != 0 {
			result[thread.Comments.Nodes[0].DatabaseID] = thread.IsResolved
		}
	}

	return result, nil
}

// graphqlMutationResponse represents the minimal response shape for GraphQL mutations.
// We only check for errors; the actual mutation payload is not inspected.
type graphqlMutationResponse struct {
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// SetDraftStatus toggles a PR's draft state via GitHub GraphQL mutations.
// nodeID is the PR's GraphQL node ID (not the numeric PR number).
// draft=true converts to draft; draft=false marks ready for review.
func (c *Client) SetDraftStatus(ctx context.Context, repoFullName string, _ int, nodeID string, draft bool) error {
	if c.token == "" {
		return fmt.Errorf("SetDraftStatus requires a GitHub token")
	}

	mutation := markReadyMutation
	if draft {
		mutation = convertToDraftMutation
	}

	reqBody := graphqlRequest{
		Query: mutation,
		Variables: map[string]any{
			"id": nodeID,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling draft status mutation: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.graphqlURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("creating draft status request: %w", err)
	}
	httpReq.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.token))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := graphqlHTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("draft status mutation for %s: %w", repoFullName, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("draft status mutation for %s: HTTP %d", repoFullName, resp.StatusCode)
	}

	var gqlResp graphqlMutationResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return fmt.Errorf("decoding draft status response for %s: %w", repoFullName, err)
	}

	if len(gqlResp.Errors) > 0 {
		return fmt.Errorf("draft status mutation for %s: %s", repoFullName, gqlResp.Errors[0].Message)
	}

	return nil
}
