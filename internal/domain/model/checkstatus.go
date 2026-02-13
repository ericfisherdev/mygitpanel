package model

import "time"

// CheckRun represents an individual CI/CD check run from the GitHub Checks API.
type CheckRun struct {
	ID          int64     // GitHub check run ID, used as primary key.
	PRID        int64     // Foreign key to pull_requests.
	Name        string    // Check run name (e.g., "build", "lint").
	Status      string    // queued, in_progress, completed, waiting, requested, pending.
	Conclusion  string    // success, failure, neutral, canceled, skipped, timed_out, action_required.
	IsRequired  bool      // From branch protection cross-reference.
	DetailsURL  string    // URL to the check run details page.
	StartedAt   time.Time // When the check run started.
	CompletedAt time.Time // When the check run completed (zero if not yet completed).
}

// CombinedStatus represents the aggregated commit status from the GitHub Status API.
type CombinedStatus struct {
	State    string         // Overall state: success, failure, pending.
	Statuses []CommitStatus // Individual status entries.
}

// CommitStatus represents an individual status entry from the GitHub Status API.
type CommitStatus struct {
	Context     string // CI service identifier (e.g., "ci/circleci").
	State       string // success, failure, pending, error.
	Description string // Human-readable description of the status.
	TargetURL   string // URL for more details on the status.
}

// PRDetail carries per-PR detail data returned by single-PR GET endpoints.
// Used as a data transfer struct, not persisted separately.
type PRDetail struct {
	Additions    int
	Deletions    int
	ChangedFiles int
	Mergeable    MergeableStatus
}
