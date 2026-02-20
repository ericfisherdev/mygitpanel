package model

// EffectiveThresholds is the merged result of global settings and per-repo overrides.
// Per-repo overrides take precedence over global defaults for non-nil fields.
type EffectiveThresholds struct {
	ReviewCountThreshold int
	AgeUrgencyDays       int
	StaleReviewEnabled   bool
	CIFailureEnabled     bool
}

// AttentionSignals is a transient model computed at query time from PR data and
// thresholds. It is never persisted to the database.
type AttentionSignals struct {
	NeedsMoreReviews bool // fewer than threshold approvals
	IsAgeUrgent      bool // open longer than threshold days
	HasStaleReview   bool // user's last review is on an outdated commit
	HasCIFailure     bool // own PR with failing CI
}

// HasAny returns true if any attention signal is active.
func (a AttentionSignals) HasAny() bool {
	return a.NeedsMoreReviews || a.IsAgeUrgent || a.HasStaleReview || a.HasCIFailure
}

// Severity returns the count of active signals (0–4), used to determine
// border color intensity in the UI.
func (a AttentionSignals) Severity() int {
	count := 0
	if a.NeedsMoreReviews {
		count++
	}
	if a.IsAgeUrgent {
		count++
	}
	if a.HasStaleReview {
		count++
	}
	if a.HasCIFailure {
		count++
	}
	return count
}
