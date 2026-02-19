package web

import (
	"io/fs"
	"log"
	"net/http"
)

// RegisterRoutes registers all web GUI routes on the provided mux.
// Web routes serve HTML at / and /app/* paths.
// Static assets are served from the embedded filesystem at /static/*.
func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	// Static assets (embedded via go:embed).
	staticFS, err := fs.Sub(StaticFS, "static")
	if err != nil {
		log.Fatalf("failed to create static sub-filesystem: %v", err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))

	// Page routes.
	mux.HandleFunc("GET /{$}", h.Dashboard)

	// HTMX partial routes.
	mux.HandleFunc("GET /app/prs/{owner}/{repo}/{number}", h.GetPRDetail)
	mux.HandleFunc("GET /app/prs/search", h.SearchPRs)

	// Repo management routes.
	mux.HandleFunc("POST /app/repos", h.AddRepo)
	mux.HandleFunc("DELETE /app/repos/{owner}/{repo}", h.RemoveRepo)

	// Settings / credential management routes.
	mux.HandleFunc("POST /app/settings/github", h.SaveGitHubCredentials)
	mux.HandleFunc("POST /app/settings/jira", h.SaveJiraCredentials)

	// Review write routes.
	mux.HandleFunc("POST /app/prs/{owner}/{repo}/{number}/comments/{rootID}/reply", h.CreateReplyComment)
	mux.HandleFunc("POST /app/prs/{owner}/{repo}/{number}/review", h.SubmitReview)
	mux.HandleFunc("POST /app/prs/{owner}/{repo}/{number}/issue-comments", h.CreateIssueComment)
	mux.HandleFunc("POST /app/prs/{owner}/{repo}/{number}/draft-toggle", h.ToggleDraftStatus)
}
