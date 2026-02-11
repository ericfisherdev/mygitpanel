package httphandler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/efisher/reviewhub/internal/application"
	"github.com/efisher/reviewhub/internal/domain/model"
	"github.com/efisher/reviewhub/internal/domain/port/driven"
)

// Handler is the HTTP driving adapter that serves the REST API.
type Handler struct {
	prStore   driven.PRStore
	repoStore driven.RepoStore
	pollSvc   *application.PollService
	username  string
	logger    *slog.Logger
}

// NewHandler creates a Handler with all required dependencies.
func NewHandler(
	prStore driven.PRStore,
	repoStore driven.RepoStore,
	pollSvc *application.PollService,
	username string,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		prStore:   prStore,
		repoStore: repoStore,
		pollSvc:   pollSvc,
		username:  username,
		logger:    logger,
	}
}

// NewServeMux creates an http.Handler with all routes registered and wrapped
// with logging and recovery middleware.
func NewServeMux(h *Handler, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/prs", h.ListPRs)
	mux.HandleFunc("GET /api/v1/prs/attention", h.ListPRsNeedingAttention)
	mux.HandleFunc("GET /api/v1/repos/{owner}/{repo}/prs/{number}", h.GetPR)
	mux.HandleFunc("GET /api/v1/repos", h.ListRepos)
	mux.HandleFunc("POST /api/v1/repos", h.AddRepo)
	mux.HandleFunc("DELETE /api/v1/repos/{owner}/{repo}", h.RemoveRepo)
	mux.HandleFunc("GET /api/v1/health", h.Health)

	// Recovery innermost so panics are caught before logging.
	wrapped := recoveryMiddleware(logger, mux)
	wrapped = loggingMiddleware(logger, wrapped)

	return wrapped
}

// ListPRs returns all tracked pull requests.
func (h *Handler) ListPRs(w http.ResponseWriter, r *http.Request) {
	prs, err := h.prStore.ListAll(r.Context())
	if err != nil {
		h.logger.Error("failed to list PRs", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := make([]PRResponse, 0, len(prs))
	for _, pr := range prs {
		resp = append(resp, toPRResponse(pr))
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetPR returns a single pull request by repository and number.
func (h *Handler) GetPR(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	numberStr := r.PathValue("number")

	number, err := strconv.Atoi(numberStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid PR number")
		return
	}

	repoFullName := owner + "/" + repo

	pr, err := h.prStore.GetByNumber(r.Context(), repoFullName, number)
	if err != nil {
		h.logger.Error("failed to get PR", "repo", repoFullName, "number", number, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if pr == nil {
		writeError(w, http.StatusNotFound, "pull request not found")
		return
	}

	writeJSON(w, http.StatusOK, toPRResponse(*pr))
}

// ListPRsNeedingAttention returns only pull requests that need review.
func (h *Handler) ListPRsNeedingAttention(w http.ResponseWriter, r *http.Request) {
	prs, err := h.prStore.ListNeedingReview(r.Context())
	if err != nil {
		h.logger.Error("failed to list PRs needing review", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := make([]PRResponse, 0, len(prs))
	for _, pr := range prs {
		resp = append(resp, toPRResponse(pr))
	}

	writeJSON(w, http.StatusOK, resp)
}

// ListRepos returns all watched repositories.
func (h *Handler) ListRepos(w http.ResponseWriter, r *http.Request) {
	repos, err := h.repoStore.ListAll(r.Context())
	if err != nil {
		h.logger.Error("failed to list repos", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := make([]RepoResponse, 0, len(repos))
	for _, repo := range repos {
		resp = append(resp, toRepoResponse(repo))
	}

	writeJSON(w, http.StatusOK, resp)
}

// AddRepo adds a repository to the watch list and triggers an async refresh.
func (h *Handler) AddRepo(w http.ResponseWriter, r *http.Request) {
	var req AddRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !isValidRepoName(req.FullName) {
		writeError(w, http.StatusBadRequest, "invalid repository name: expected owner/repo format")
		return
	}

	parts := strings.SplitN(req.FullName, "/", 2)
	repo := model.Repository{
		FullName: req.FullName,
		Owner:    parts[0],
		Name:     parts[1],
		AddedAt:  time.Now().UTC(),
	}

	if err := h.repoStore.Add(r.Context(), repo); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			writeError(w, http.StatusConflict, "repository already exists")
			return
		}
		h.logger.Error("failed to add repo", "repo", req.FullName, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Fire-and-forget async refresh with background context since the HTTP
	// request context will be cancelled after the response is sent.
	if h.pollSvc != nil {
		go func() {
			if err := h.pollSvc.RefreshRepo(context.Background(), req.FullName); err != nil {
				h.logger.Error("async repo refresh failed", "repo", req.FullName, "error", err)
			}
		}()
	}

	writeJSON(w, http.StatusCreated, toRepoResponse(repo))
}

// RemoveRepo removes a repository from the watch list.
func (h *Handler) RemoveRepo(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	fullName := owner + "/" + repo

	if err := h.repoStore.Remove(r.Context(), fullName); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "repository not found")
			return
		}
		h.logger.Error("failed to remove repo", "repo", fullName, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Health returns a simple health check response.
func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{
		Status: "ok",
		Time:   time.Now().UTC().Format(time.RFC3339),
	})
}

// isValidRepoName validates that name is in owner/repo format where each part
// contains only alphanumeric characters, hyphens, dots, or underscores.
func isValidRepoName(name string) bool {
	parts := strings.SplitN(name, "/", 3)
	if len(parts) != 2 {
		return false
	}

	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, ch := range part {
			if !isValidRepoChar(ch) {
				return false
			}
		}
	}

	return true
}

// isValidRepoChar returns true if the rune is allowed in a repository owner or name.
func isValidRepoChar(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') ||
		ch == '-' || ch == '.' || ch == '_'
}
