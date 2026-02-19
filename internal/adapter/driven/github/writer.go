// Package github implements the GitHubClient and GitHubWriter ports using the go-github library.
package github

import (
	"context"
	"fmt"
	"net/http"

	gh "github.com/google/go-github/v82/github"

	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
)

// Compile-time interface satisfaction check.
var _ driven.GitHubWriter = (*Client)(nil)

// ValidateToken verifies that the given GitHub personal access token is valid
// and returns the authenticated username on success. It creates a one-shot
// client with the provided token to avoid mutating the receiver's state.
func (c *Client) ValidateToken(ctx context.Context, token string) (string, error) {
	tempClient := gh.NewClient(http.DefaultClient).WithAuthToken(token)
	user, _, err := tempClient.Users.Get(ctx, "")
	if err != nil {
		return "", fmt.Errorf("token validation failed: %w", err)
	}
	return user.GetLogin(), nil
}

// SubmitReview creates a pull request review with optional inline comments.
// Full implementation in Plan 03.
func (c *Client) SubmitReview(_ context.Context, _ string, _ int, _ driven.ReviewRequest) error {
	return fmt.Errorf("not yet implemented")
}

// CreateReplyComment creates a reply to an existing review thread.
// Full implementation in Plan 03.
func (c *Client) CreateReplyComment(_ context.Context, _ string, _ int, _ int64, _, _, _ string) error {
	return fmt.Errorf("not yet implemented")
}

// CreateIssueComment creates a top-level (non-diff) comment on a pull request.
// Full implementation in Plan 03.
func (c *Client) CreateIssueComment(_ context.Context, _ string, _ int, _ string) error {
	return fmt.Errorf("not yet implemented")
}

// ConvertPullRequestToDraft converts a ready-for-review PR to draft status.
// Full implementation in Plan 04.
func (c *Client) ConvertPullRequestToDraft(_ context.Context, _ string, _ int) error {
	return fmt.Errorf("not yet implemented")
}

// MarkPullRequestReadyForReview converts a draft PR to ready-for-review status.
// Full implementation in Plan 04.
func (c *Client) MarkPullRequestReadyForReview(_ context.Context, _ string, _ int) error {
	return fmt.Errorf("not yet implemented")
}
