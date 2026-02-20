// Package github implements the GitHubClient and GitHubWriter ports using the go-github library.
package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	gh "github.com/google/go-github/v82/github"

	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
)

// Compile-time interface satisfaction check.
var _ driven.GitHubWriter = (*Client)(nil)

// ValidateToken verifies that the given GitHub personal access token is valid
// and returns the authenticated username on success. It creates a one-shot
// client with the provided token to avoid mutating the receiver's state.
func (c *Client) ValidateToken(ctx context.Context, token string) (string, error) {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	tempClient := gh.NewClient(httpClient).WithAuthToken(token)
	user, _, err := tempClient.Users.Get(ctx, "")
	if err != nil {
		return "", fmt.Errorf("token validation failed: %w", err)
	}
	return user.GetLogin(), nil
}

// SubmitReview creates a pull request review with optional inline comments.
// If the CommitID in req is empty, the current PR head SHA is fetched first
// to avoid submitting against a stale commit.
func (c *Client) SubmitReview(ctx context.Context, repoFullName string, prNumber int, req driven.ReviewRequest) error {
	owner, repo, err := splitRepo(repoFullName)
	if err != nil {
		return err
	}

	// Re-fetch the head SHA if not provided to avoid 422 "commit not found" errors.
	commitID := req.CommitID
	if commitID == "" {
		pr, _, err := c.gh.PullRequests.Get(ctx, owner, repo, prNumber)
		if err != nil {
			return fmt.Errorf("fetching PR head SHA before review submit: %w", err)
		}
		commitID = pr.GetHead().GetSHA()
	}

	// Map DraftLineComments to GitHub API types.
	var draftComments []*gh.DraftReviewComment
	for _, dlc := range req.Comments {
		dc := &gh.DraftReviewComment{
			Path: gh.Ptr(dlc.Path),
			Body: gh.Ptr(dlc.Body),
			Line: gh.Ptr(dlc.Line),
			Side: gh.Ptr(dlc.Side),
		}
		if dlc.StartLine > 0 {
			dc.StartLine = gh.Ptr(dlc.StartLine)
			dc.StartSide = gh.Ptr(dlc.StartSide)
		}
		draftComments = append(draftComments, dc)
	}

	reviewReq := &gh.PullRequestReviewRequest{
		CommitID: gh.Ptr(commitID),
		Event:    gh.Ptr(req.Event),
		Comments: draftComments,
	}

	// Only set Body if non-empty or event requires it (not APPROVE with empty body).
	if req.Body != "" || req.Event != "APPROVE" {
		reviewReq.Body = gh.Ptr(req.Body)
	}

	_, _, err = c.gh.PullRequests.CreateReview(ctx, owner, repo, prNumber, reviewReq)
	if err != nil {
		var ghErr *gh.ErrorResponse
		if errors.As(err, &ghErr) && ghErr.Response != nil && ghErr.Response.StatusCode == 422 {
			return fmt.Errorf("PR was updated since you started reviewing; refresh and try again: %w", err)
		}
		return fmt.Errorf("submitting review for %s#%d: %w", repoFullName, prNumber, err)
	}

	return nil
}

// CreateReplyComment creates a reply to an existing review thread.
func (c *Client) CreateReplyComment(ctx context.Context, repoFullName string, prNumber int, inReplyTo int64, body, path, commitSHA string) error {
	owner, repo, err := splitRepo(repoFullName)
	if err != nil {
		return err
	}

	// When in_reply_to is set, GitHub ignores all fields except body.
	_, _, err = c.gh.PullRequests.CreateComment(ctx, owner, repo, prNumber, &gh.PullRequestComment{
		Body:      gh.Ptr(body),
		InReplyTo: gh.Ptr(inReplyTo),
	})
	if err != nil {
		return fmt.Errorf("creating reply comment on %s#%d: %w", repoFullName, prNumber, err)
	}

	return nil
}

// CreateIssueComment creates a top-level (non-diff) comment on a pull request.
func (c *Client) CreateIssueComment(ctx context.Context, repoFullName string, prNumber int, body string) error {
	owner, repo, err := splitRepo(repoFullName)
	if err != nil {
		return err
	}

	_, _, err = c.gh.Issues.CreateComment(ctx, owner, repo, prNumber, &gh.IssueComment{
		Body: gh.Ptr(body),
	})
	if err != nil {
		return fmt.Errorf("creating issue comment on %s#%d: %w", repoFullName, prNumber, err)
	}

	return nil
}

// ConvertPullRequestToDraft converts a ready-for-review PR to draft status using
// the GitHub GraphQL API. The PR node ID is fetched on-demand via REST.
func (c *Client) ConvertPullRequestToDraft(ctx context.Context, repoFullName string, prNumber int) error {
	owner, repo, err := splitRepo(repoFullName)
	if err != nil {
		return err
	}
	pr, _, err := c.gh.PullRequests.Get(ctx, owner, repo, prNumber)
	if err != nil {
		return fmt.Errorf("fetching PR node ID: %w", err)
	}
	nodeID := pr.GetNodeID()
	if nodeID == "" {
		return fmt.Errorf("PR node ID is empty — cannot execute GraphQL mutation")
	}
	return c.executeDraftMutation(ctx, convertToDraftMutation, nodeID)
}

// MarkPullRequestReadyForReview converts a draft PR to ready-for-review status using
// the GitHub GraphQL API. The PR node ID is fetched on-demand via REST.
func (c *Client) MarkPullRequestReadyForReview(ctx context.Context, repoFullName string, prNumber int) error {
	owner, repo, err := splitRepo(repoFullName)
	if err != nil {
		return err
	}
	pr, _, err := c.gh.PullRequests.Get(ctx, owner, repo, prNumber)
	if err != nil {
		return fmt.Errorf("fetching PR node ID: %w", err)
	}
	nodeID := pr.GetNodeID()
	if nodeID == "" {
		return fmt.Errorf("PR node ID is empty — cannot execute GraphQL mutation")
	}
	return c.executeDraftMutation(ctx, markReadyMutation, nodeID)
}
