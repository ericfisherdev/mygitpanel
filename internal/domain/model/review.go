package model

import "time"

// Review represents a review submitted on a pull request.
type Review struct {
	ID            int64
	PRID          int64
	ReviewerLogin string
	State         ReviewState
	Body          string
	CommitID      string // SHA of the commit this review targets; used for outdated detection.
	SubmittedAt   time.Time
	IsBot         bool
}
