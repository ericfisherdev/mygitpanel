package driven

import "context"

// DraftLineComment represents a single inline comment to be submitted as part
// of a pull request review.
type DraftLineComment struct {
	Path      string // File path relative to repository root.
	Line      int    // Source file line number (used with Side).
	Side      string // "RIGHT" for new content, "LEFT" for old content.
	StartLine int    // For multi-line comments: first line of the range.
	StartSide string // Side of StartLine for multi-line comments.
	Body      string // Comment body text.
}

// ReviewRequest is the input to GitHubWriter.SubmitReview.
type ReviewRequest struct {
	CommitID string             // HEAD SHA to attach the review to.
	Event    string             // "APPROVE", "REQUEST_CHANGES", or "COMMENT".
	Body     string             // Top-level review body.
	Comments []DraftLineComment // Optional inline comments.
}

// GitHubWriter defines the driven port for GitHub write operations.
// It is intentionally separate from GitHubClient (read operations) following
// the Interface Segregation Principle.
type GitHubWriter interface {
	// SubmitReview creates a pull request review with optional inline comments.
	SubmitReview(ctx context.Context, repoFullName string, prNumber int, req ReviewRequest) error

	// CreateReplyComment creates a reply to an existing review thread.
	CreateReplyComment(ctx context.Context, repoFullName string, prNumber int, inReplyTo int64, body, path, commitSHA string) error

	// CreateIssueComment creates a top-level (non-diff) comment on a pull request.
	CreateIssueComment(ctx context.Context, repoFullName string, prNumber int, body string) error

	// ConvertPullRequestToDraft converts a ready-for-review PR to draft status.
	ConvertPullRequestToDraft(ctx context.Context, repoFullName string, prNumber int) error

	// MarkPullRequestReadyForReview converts a draft PR to ready-for-review status.
	MarkPullRequestReadyForReview(ctx context.Context, repoFullName string, prNumber int) error

	// ValidateToken verifies that the given GitHub personal access token is valid
	// and returns the authenticated username on success.
	ValidateToken(ctx context.Context, token string) (username string, err error)
}
