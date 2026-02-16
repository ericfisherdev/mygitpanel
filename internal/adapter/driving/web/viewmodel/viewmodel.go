// Package viewmodel defines presentation-ready structs for templ components.
// View models decouple template rendering from domain model types.
package viewmodel

// PRCardViewModel holds presentation-ready data for a PR card in the sidebar list.
type PRCardViewModel struct {
	Number                int
	Repository            string
	Title                 string
	Author                string
	Status                string
	IsDraft               bool
	NeedsReview           bool
	CIStatus              string
	MergeableStatus       string
	ReviewStatus          string
	DaysSinceOpened       int
	DaysSinceLastActivity int
	Labels                []string
	URL                   string
	DetailPath            string
}

// PRDetailViewModel holds presentation-ready data for the full PR detail panel.
type PRDetailViewModel struct {
	PRCardViewModel

	Branch       string
	BaseBranch   string
	HeadSHA      string
	NodeID       string
	Additions    int
	Deletions    int
	ChangedFiles int

	Reviews       []ReviewViewModel
	Threads       []ThreadViewModel
	IssueComments []IssueCommentViewModel
	CheckRuns     []CheckRunViewModel
	Suggestions   []SuggestionViewModel

	HasBotReview        bool
	HasCoderabbitReview bool
	AwaitingCoderabbit  bool
	ResolvedThreads     int
	UnresolvedThreads   int

	// Write capability flags.
	CanReview       bool   // true if PR author != current username and credentials configured
	CanToggleDraft  bool   // true if NodeID is non-empty and credentials configured
	IsCurrentUser   bool   // true if PR author matches current username
	RepoFullName    string // "owner/repo" for form action URLs
	PRNumber        int    // PR number for form action URLs
	ReviewActionURL string // POST target for review submission
	CommentURL      string // POST target for PR-level comments
	DraftToggleURL  string // POST target for draft toggle
}

// ReviewViewModel holds presentation-ready data for a single review.
type ReviewViewModel struct {
	ID          int64
	Reviewer    string
	State       string
	Body        string
	BodyHTML    string
	CommitID    string
	SubmittedAt string
	IsBot       bool
	IsOutdated  bool
	IsNitpick   bool
}

// ThreadViewModel holds presentation-ready data for a review comment thread.
type ThreadViewModel struct {
	RootComment  ReviewCommentViewModel
	Replies      []ReviewCommentViewModel
	IsResolved   bool
	CommentCount int
}

// ReviewCommentViewModel holds presentation-ready data for a single review comment.
type ReviewCommentViewModel struct {
	ID           int64
	Author       string
	Body         string
	BodyHTML     string
	FilePath     string
	Line         int
	StartLine    int
	DiffHunk     string
	DiffHunkHTML string
	CommitID     string
	IsOutdated   bool
	CreatedAt    string
}

// IssueCommentViewModel holds presentation-ready data for a PR-level general comment.
type IssueCommentViewModel struct {
	ID        int64
	Author    string
	Body      string
	BodyHTML  string
	IsBot     bool
	CreatedAt string
}

// CheckRunViewModel holds presentation-ready data for a single CI/CD check run.
type CheckRunViewModel struct {
	ID         int64
	Name       string
	Status     string
	Conclusion string
	IsRequired bool
	DetailsURL string
}

// SuggestionViewModel holds presentation-ready data for a proposed code change.
type SuggestionViewModel struct {
	CommentID    int64
	FilePath     string
	StartLine    int
	EndLine      int
	ProposedCode string
}

// RepoFilterViewModel holds presentation data for a repo in the filter dropdown.
type RepoFilterViewModel struct {
	FullName string
	Selected bool
}

// RepoViewModel holds presentation data for a watched repo in the repo manager.
type RepoViewModel struct {
	FullName   string
	Owner      string
	Name       string
	DeletePath string // computed: /app/repos/{owner}/{repo}
}

// CredentialStatusViewModel holds presentation data for the credential status indicator.
type CredentialStatusViewModel struct {
	Configured  bool
	Username    string
	TokenMasked string
}

// DashboardViewModel holds all data needed to render the dashboard page.
type DashboardViewModel struct {
	Cards            []PRCardViewModel
	Repos            []RepoViewModel
	RepoNames        []string // distinct repo names for search bar filter
	CredentialStatus CredentialStatusViewModel
}
