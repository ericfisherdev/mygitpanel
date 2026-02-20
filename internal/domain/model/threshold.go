package model

// GlobalSettings holds the global default thresholds for attention signal computation.
type GlobalSettings struct {
	ReviewCountThreshold int
	AgeUrgencyDays       int
	StaleReviewEnabled   bool
	CIFailureEnabled     bool
}

// defaultAgeUrgencyDays is the default number of days before a PR is considered age-urgent.
const defaultAgeUrgencyDays = 7

// DefaultGlobalSettings returns the hard-coded defaults used when no global
// settings row exists in the database.
func DefaultGlobalSettings() GlobalSettings {
	return GlobalSettings{
		ReviewCountThreshold: 1,
		AgeUrgencyDays:       defaultAgeUrgencyDays,
		StaleReviewEnabled:   true,
		CIFailureEnabled:     true,
	}
}

// RepoThreshold holds per-repository threshold overrides. Nil pointer fields
// mean "use the global default" for that setting.
type RepoThreshold struct {
	RepoFullName       string
	ReviewCount        *int
	AgeUrgencyDays     *int
	StaleReviewEnabled *bool
	CIFailureEnabled   *bool
}
