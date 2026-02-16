package github

import (
	"context"
	"fmt"

	gh "github.com/google/go-github/v82/github"
)

// CreateReview submits a review on a pull request.
// event must be one of "APPROVE", "REQUEST_CHANGES", or "COMMENT".
func (c *Client) CreateReview(ctx context.Context, repoFullName string, prNumber int, event string, body string) error {
	owner, repo, err := splitRepo(repoFullName)
	if err != nil {
		return err
	}

	review := &gh.PullRequestReviewRequest{
		Event: gh.Ptr(event),
		Body:  gh.Ptr(body),
	}

	_, resp, err := c.gh.PullRequests.CreateReview(ctx, owner, repo, prNumber, review)
	if err != nil {
		return fmt.Errorf("creating review for %s#%d: %w", repoFullName, prNumber, err)
	}

	logRateLimit(resp, repoFullName+"/create-review", 0, 1)
	return nil
}

// CreateIssueComment adds a PR-level comment via the Issues API.
func (c *Client) CreateIssueComment(ctx context.Context, repoFullName string, prNumber int, body string) error {
	owner, repo, err := splitRepo(repoFullName)
	if err != nil {
		return err
	}

	comment := &gh.IssueComment{Body: gh.Ptr(body)}
	_, resp, err := c.gh.Issues.CreateComment(ctx, owner, repo, prNumber, comment)
	if err != nil {
		return fmt.Errorf("creating comment on %s#%d: %w", repoFullName, prNumber, err)
	}

	logRateLimit(resp, repoFullName+"/create-comment", 0, 1)
	return nil
}

// ReplyToReviewComment replies to an existing review comment thread.
// commentID must be the root comment ID of the thread.
func (c *Client) ReplyToReviewComment(ctx context.Context, repoFullName string, prNumber int, commentID int64, body string) error {
	owner, repo, err := splitRepo(repoFullName)
	if err != nil {
		return err
	}

	_, resp, err := c.gh.PullRequests.CreateCommentInReplyTo(ctx, owner, repo, prNumber, body, commentID)
	if err != nil {
		return fmt.Errorf("replying to comment %d on %s#%d: %w", commentID, repoFullName, prNumber, err)
	}

	logRateLimit(resp, repoFullName+"/reply-comment", 0, 1)
	return nil
}
