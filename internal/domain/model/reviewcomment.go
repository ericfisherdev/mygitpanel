package model

import "time"

// ReviewComment represents a comment on a specific line within a pull request review.
type ReviewComment struct {
	ID          int64
	ReviewID    int64
	PRID        int64
	Author      string
	Body        string
	Path        string
	Line        int
	StartLine   int // Multi-line comment range start; 0 if single-line.
	Side        string
	SubjectType string // From GitHub: "line" or "file".
	DiffHunk    string
	CommitID    string // SHA of the commit this comment targets.
	IsResolved  bool
	IsOutdated  bool
	InReplyToID *int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
