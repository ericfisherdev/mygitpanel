package httphandler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/efisher/reviewhub/internal/domain/model"
)

// writeJSON marshals v to JSON and writes it to the response with the given
// status code. If marshalling fails, a 500 error is written instead.
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
	Number     int      `json:"number"`
	Repository string   `json:"repository"`
	Title      string   `json:"title"`
	Author     string   `json:"author"`
	Status     string   `json:"status"`
	IsDraft    bool     `json:"is_draft"`
	NeedsReview bool   `json:"needs_review"`
	URL        string   `json:"url"`
	Branch     string   `json:"branch"`
	BaseBranch string   `json:"base_branch"`
	Labels     []string `json:"labels"`
	Reviews    []any    `json:"reviews"`
	Comments   []any    `json:"comments"`
	OpenedAt   string   `json:"opened_at"`
	UpdatedAt  string   `json:"updated_at"`
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
func toPRResponse(pr model.PullRequest) PRResponse {
	labels := pr.Labels
	if labels == nil {
		labels = []string{}
	}

	return PRResponse{
		Number:      pr.Number,
		Repository:  pr.RepoFullName,
		Title:       pr.Title,
		Author:      pr.Author,
		Status:      string(pr.Status),
		IsDraft:     pr.IsDraft,
		NeedsReview: pr.NeedsReview,
		URL:         pr.URL,
		Branch:      pr.Branch,
		BaseBranch:  pr.BaseBranch,
		Labels:      labels,
		Reviews:     []any{},
		Comments:    []any{},
		OpenedAt:    pr.OpenedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   pr.UpdatedAt.UTC().Format(time.RFC3339),
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
