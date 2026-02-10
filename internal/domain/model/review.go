package model

import "time"

// Review represents a review submitted on a pull request.
type Review struct {
	ID            int64
	PRID          int64
	ReviewerLogin string
	State         ReviewState
	Body          string
	SubmittedAt   time.Time
	IsBot         bool
}
