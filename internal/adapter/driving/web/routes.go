package web

import (
	"io/fs"
	"net/http"
)

// RegisterRoutes registers all web GUI routes on the provided mux.
// Web routes serve HTML at / and /app/* paths.
// Static assets are served from the embedded filesystem at /static/*.
func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	// Static assets (embedded via go:embed).
	staticFS, _ := fs.Sub(StaticFS, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))

	// Page routes.
	mux.HandleFunc("GET /{$}", h.Dashboard)

	// HTMX partial routes.
	mux.HandleFunc("GET /app/prs/{owner}/{repo}/{number}", h.GetPRDetail)
	mux.HandleFunc("GET /app/prs/search", h.SearchPRs)

	// Repo management routes.
	mux.HandleFunc("POST /app/repos", h.AddRepo)
	mux.HandleFunc("DELETE /app/repos/{owner}/{repo}", h.RemoveRepo)
}
