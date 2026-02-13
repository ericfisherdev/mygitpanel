package model

// PRStatus represents the state of a pull request.
type PRStatus string

// PRStatus values.
const (
	PRStatusOpen   PRStatus = "open"
	PRStatusClosed PRStatus = "closed"
	PRStatusMerged PRStatus = "merged"
)

// ReviewState represents the state of a review.
type ReviewState string

// ReviewState values.
const (
	ReviewStateApproved         ReviewState = "approved"
	ReviewStateChangesRequested ReviewState = "changes_requested"
	ReviewStateCommented        ReviewState = "commented"
	ReviewStatePending          ReviewState = "pending"
	ReviewStateDismissed        ReviewState = "dismissed"
)

// CIStatus represents the state of a CI check.
type CIStatus string

// CIStatus values.
const (
	CIStatusPassing CIStatus = "passing"
	CIStatusFailing CIStatus = "failing"
	CIStatusPending CIStatus = "pending"
	CIStatusUnknown CIStatus = "unknown"
)

// CommentType distinguishes between different origins of PR comments.
type CommentType string

// CommentType values.
const (
	CommentTypeInline  CommentType = "inline"  // Review comment on a code line.
	CommentTypeGeneral CommentType = "general" // Issue comment / PR-level discussion.
	CommentTypeFile    CommentType = "file"    // Review comment on a file, not a specific line.
)
