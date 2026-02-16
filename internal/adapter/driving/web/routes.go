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

	// Credential management routes.
	mux.HandleFunc("GET /app/credentials", h.GetCredentialForm)
	mux.HandleFunc("POST /app/credentials", h.SaveCredentials)

	// PR write operation routes.
	mux.HandleFunc("POST /app/prs/{owner}/{repo}/{number}/review", h.SubmitReview)
	mux.HandleFunc("POST /app/prs/{owner}/{repo}/{number}/comment", h.AddComment)
	mux.HandleFunc("POST /app/prs/{owner}/{repo}/{number}/comments/{id}/reply", h.ReplyToComment)
	mux.HandleFunc("POST /app/prs/{owner}/{repo}/{number}/draft", h.ToggleDraft)

	// Repo settings routes.
	mux.HandleFunc("GET /app/repos/{owner}/{repo}/settings", h.GetRepoSettings)
	mux.HandleFunc("POST /app/repos/{owner}/{repo}/settings", h.SaveRepoSettings)
}
