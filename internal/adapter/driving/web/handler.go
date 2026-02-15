// Package web implements the HTML GUI driving adapter using templ components.
package web

import (
	"log/slog"
	"net/http"

	"github.com/ericfisherdev/mygitpanel/internal/adapter/driving/web/templates"
	"github.com/ericfisherdev/mygitpanel/internal/adapter/driving/web/templates/pages"
	"github.com/ericfisherdev/mygitpanel/internal/application"
	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
)

// Handler is the web GUI driving adapter that serves HTML via templ components.
type Handler struct {
	prStore   driven.PRStore
	repoStore driven.RepoStore
	reviewSvc *application.ReviewService
	healthSvc *application.HealthService
	pollSvc   *application.PollService
	username  string
	logger    *slog.Logger
}

// NewHandler creates a Handler with all required dependencies.
func NewHandler(
	prStore driven.PRStore,
	repoStore driven.RepoStore,
	reviewSvc *application.ReviewService,
	healthSvc *application.HealthService,
	pollSvc *application.PollService,
	username string,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		prStore:   prStore,
		repoStore: repoStore,
		reviewSvc: reviewSvc,
		healthSvc: healthSvc,
		pollSvc:   pollSvc,
		username:  username,
		logger:    logger,
	}
}

// Dashboard renders the main dashboard page with the full HTML layout.
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	component := pages.Dashboard()
	layout := templates.Layout("ReviewHub", component)

	if err := layout.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render dashboard", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
