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
	Side        string
	DiffHunk    string
	IsResolved  bool
	IsOutdated  bool
	InReplyToID *int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
