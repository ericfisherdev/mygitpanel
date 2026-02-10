package model

// CheckStatus represents the result of a CI/CD check on a pull request.
type CheckStatus struct {
	Name       string
	Status     CIStatus
	Conclusion string
	IsRequired bool
	DetailsURL string
}
