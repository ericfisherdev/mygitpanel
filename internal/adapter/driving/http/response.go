package httphandler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/ericfisherdev/mygitpanel/internal/application"
	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// writeJSON marshals v to JSON and writes it to the response with the given
// status code. If marshaling fails, a 500 error is written instead.
func writeJSON(w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

// writeError writes a JSON error response with the given status code and message.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

// errorResponse is the standard error response body.
type errorResponse struct {
	Error string `json:"error"`
}

// PRResponse is the JSON representation of a pull request.
type PRResponse struct {
	Number      int      `json:"number"`
	Repository  string   `json:"repository"`
	Title       string   `json:"title"`
	Author      string   `json:"author"`
	Status      string   `json:"status"`
	IsDraft     bool     `json:"is_draft"`
	NeedsReview bool     `json:"needs_review"`
	URL         string   `json:"url"`
	Branch      string   `json:"branch"`
	BaseBranch  string   `json:"base_branch"`
	Labels      []string `json:"labels"`
	OpenedAt    string   `json:"opened_at"`
	UpdatedAt   string   `json:"updated_at"`

	// Enriched review data -- populated only on single PR detail endpoint.
	HeadSHA             string                 `json:"head_sha"`
	Reviews             []ReviewResponse       `json:"reviews"`
	Threads             []ReviewThreadResponse `json:"threads"`
	IssueComments       []IssueCommentResponse `json:"issue_comments"`
	Suggestions         []SuggestionResponse   `json:"suggestions"`
	ReviewStatus        string                 `json:"review_status"`
	HasBotReview        bool                   `json:"has_bot_review"`
	HasCoderabbitReview bool                   `json:"has_coderabbit_review"`
	AwaitingCoderabbit  bool                   `json:"awaiting_coderabbit"`
	ResolvedThreads     int                    `json:"resolved_threads"`
	UnresolvedThreads   int                    `json:"unresolved_threads"`

	// Health signal fields -- populated from PR model on all endpoints.
	DaysSinceOpened       int                `json:"days_since_opened"`
	DaysSinceLastActivity int                `json:"days_since_last_activity"`
	Additions             int                `json:"additions"`
	Deletions             int                `json:"deletions"`
	ChangedFiles          int                `json:"changed_files"`
	MergeableStatus       string             `json:"mergeable_status"`
	CIStatus              string             `json:"ci_status"`
	CheckRuns             []CheckRunResponse `json:"check_runs"`
}

// ReviewResponse is the JSON representation of a single review.
type ReviewResponse struct {
	ID            int64  `json:"id"`
	ReviewerLogin string `json:"reviewer"`
	State         string `json:"state"`
	Body          string `json:"body"`
	CommitID      string `json:"commit_id"`
	SubmittedAt   string `json:"submitted_at"`
	IsBot         bool   `json:"is_bot"`
	IsOutdated    bool   `json:"is_outdated"`
	IsNitpick     bool   `json:"is_nitpick"`
}

// ReviewCommentResponse is the JSON representation of a single review comment.
type ReviewCommentResponse struct {
	ID          int64  `json:"id"`
	Author      string `json:"author"`
	Body        string `json:"body"`
	FilePath    string `json:"file_path"`
	Line        int    `json:"line"`
	StartLine   int    `json:"start_line,omitempty"`
	Side        string `json:"side"`
	SubjectType string `json:"subject_type"`
	DiffHunk    string `json:"diff_hunk"`
	CommitID    string `json:"commit_id"`
	IsOutdated  bool   `json:"is_outdated"`
	CreatedAt   string `json:"created_at"`
}

// ReviewThreadResponse is a grouped conversation thread with root comment and replies.
type ReviewThreadResponse struct {
	RootComment  ReviewCommentResponse   `json:"root_comment"`
	Replies      []ReviewCommentResponse `json:"replies"`
	IsResolved   bool                    `json:"is_resolved"`
	CommentCount int                     `json:"comment_count"`
}

// SuggestionResponse is a structured proposed code change extracted from a comment.
type SuggestionResponse struct {
	CommentID    int64  `json:"comment_id"`
	FilePath     string `json:"file_path"`
	StartLine    int    `json:"start_line"`
	EndLine      int    `json:"end_line"`
	ProposedCode string `json:"proposed_code"`
}

// IssueCommentResponse is a general PR-level comment.
type IssueCommentResponse struct {
	ID        int64  `json:"id"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	IsBot     bool   `json:"is_bot"`
	CreatedAt string `json:"created_at"`
}

// CheckRunResponse is the JSON representation of an individual CI/CD check run.
type CheckRunResponse struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	IsRequired bool   `json:"is_required"`
	DetailsURL string `json:"details_url"`
}

// BotConfigResponse is the JSON representation of a bot configuration entry.
type BotConfigResponse struct {
	Username string `json:"username"`
	AddedAt  string `json:"added_at"`
}

// AddBotRequest is the JSON body for the add bot endpoint.
type AddBotRequest struct {
	Username string `json:"username"`
}

// RepoResponse is the JSON representation of a watched repository.
type RepoResponse struct {
	FullName string `json:"full_name"`
	Owner    string `json:"owner"`
	Name     string `json:"name"`
	AddedAt  string `json:"added_at"`
}

// HealthResponse is the JSON representation of the health check endpoint.
type HealthResponse struct {
	Status string `json:"status"`
	Time   string `json:"time"`
}

// AddRepoRequest is the JSON body for the add repository endpoint.
type AddRepoRequest struct {
	FullName string `json:"full_name"`
}

// toPRResponse converts a domain PullRequest to its JSON response representation.
// All enriched fields are initialized with empty defaults (empty slices, zero values).
// The GetPR handler populates enriched data from ReviewService after this call.
func toPRResponse(pr model.PullRequest) PRResponse {
	labels := pr.Labels
	if labels == nil {
		labels = []string{}
	}

	return PRResponse{
		Number:        pr.Number,
		Repository:    pr.RepoFullName,
		Title:         pr.Title,
		Author:        pr.Author,
		Status:        string(pr.Status),
		IsDraft:       pr.IsDraft,
		NeedsReview:   pr.NeedsReview,
		URL:           pr.URL,
		Branch:        pr.Branch,
		BaseBranch:    pr.BaseBranch,
		Labels:        labels,
		OpenedAt:      pr.OpenedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     pr.UpdatedAt.UTC().Format(time.RFC3339),
		HeadSHA:       pr.HeadSHA,
		Reviews:       []ReviewResponse{},
		Threads:       []ReviewThreadResponse{},
		IssueComments: []IssueCommentResponse{},
		Suggestions:   []SuggestionResponse{},

		// Health signals from PR model -- available on all endpoints.
		DaysSinceOpened:       pr.DaysSinceOpened(),
		DaysSinceLastActivity: pr.DaysSinceLastActivity(),
		Additions:             pr.Additions,
		Deletions:             pr.Deletions,
		ChangedFiles:          pr.ChangedFiles,
		MergeableStatus:       string(pr.MergeableStatus),
		CIStatus:              string(pr.CIStatus),
		CheckRuns:             []CheckRunResponse{},
	}
}

// toReviewResponse converts a domain Review to its JSON response representation.
func toReviewResponse(r model.Review, headSHA string, botUsernames []string) ReviewResponse {
	isOutdated := r.CommitID != "" && r.CommitID != headSHA
	isNitpick := application.IsNitpickComment(r.ReviewerLogin, r.Body, botUsernames)

	return ReviewResponse{
		ID:            r.ID,
		ReviewerLogin: r.ReviewerLogin,
		State:         string(r.State),
		Body:          r.Body,
		CommitID:      r.CommitID,
		SubmittedAt:   r.SubmittedAt.UTC().Format(time.RFC3339),
		IsBot:         r.IsBot,
		IsOutdated:    isOutdated,
		IsNitpick:     isNitpick,
	}
}

// toReviewCommentResponse converts a domain ReviewComment to its JSON representation.
func toReviewCommentResponse(c model.ReviewComment) ReviewCommentResponse {
	return ReviewCommentResponse{
		ID:          c.ID,
		Author:      c.Author,
		Body:        c.Body,
		FilePath:    c.Path,
		Line:        c.Line,
		StartLine:   c.StartLine,
		Side:        c.Side,
		SubjectType: c.SubjectType,
		DiffHunk:    c.DiffHunk,
		CommitID:    c.CommitID,
		IsOutdated:  c.IsOutdated,
		CreatedAt:   c.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// toReviewThreadResponse converts an application CommentThread to its JSON representation.
func toReviewThreadResponse(t application.CommentThread) ReviewThreadResponse {
	replies := make([]ReviewCommentResponse, 0, len(t.Replies))
	for _, r := range t.Replies {
		replies = append(replies, toReviewCommentResponse(r))
	}

	return ReviewThreadResponse{
		RootComment:  toReviewCommentResponse(t.RootComment),
		Replies:      replies,
		IsResolved:   t.IsResolved,
		CommentCount: 1 + len(t.Replies),
	}
}

// toSuggestionResponse converts an application Suggestion to its JSON representation.
func toSuggestionResponse(s application.Suggestion) SuggestionResponse {
	return SuggestionResponse{
		CommentID:    s.CommentID,
		FilePath:     s.FilePath,
		StartLine:    s.StartLine,
		EndLine:      s.EndLine,
		ProposedCode: s.ProposedCode,
	}
}

// toIssueCommentResponse converts a domain IssueComment to its JSON representation.
func toIssueCommentResponse(c model.IssueComment) IssueCommentResponse {
	return IssueCommentResponse{
		ID:        c.ID,
		Author:    c.Author,
		Body:      c.Body,
		IsBot:     c.IsBot,
		CreatedAt: c.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// toCheckRunResponse converts a domain CheckRun to its JSON representation.
func toCheckRunResponse(cr model.CheckRun) CheckRunResponse {
	return CheckRunResponse{
		ID:         cr.ID,
		Name:       cr.Name,
		Status:     cr.Status,
		Conclusion: cr.Conclusion,
		IsRequired: cr.IsRequired,
		DetailsURL: cr.DetailsURL,
	}
}

// toBotConfigResponse converts a domain BotConfig to its JSON representation.
func toBotConfigResponse(bot model.BotConfig) BotConfigResponse {
	return BotConfigResponse{
		Username: bot.Username,
		AddedAt:  bot.AddedAt.UTC().Format(time.RFC3339),
	}
}

// toRepoResponse converts a domain Repository to its JSON response representation.
func toRepoResponse(repo model.Repository) RepoResponse {
	return RepoResponse{
		FullName: repo.FullName,
		Owner:    repo.Owner,
		Name:     repo.Name,
		AddedAt:  repo.AddedAt.UTC().Format(time.RFC3339),
	}
}
