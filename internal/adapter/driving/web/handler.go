// Package web implements the HTML GUI driving adapter using templ components.
package web

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ericfisherdev/mygitpanel/internal/adapter/driving/web/templates"
	"github.com/ericfisherdev/mygitpanel/internal/adapter/driving/web/templates/pages"
	"github.com/ericfisherdev/mygitpanel/internal/adapter/driving/web/templates/partials"
	"github.com/ericfisherdev/mygitpanel/internal/application"
	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
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

// Dashboard renders the main dashboard page with PR list in the sidebar.
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	prs, err := h.prStore.ListAll(r.Context())
	if err != nil {
		h.logger.Error("failed to list PRs", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	cards := toPRCardViewModels(prs)
	component := pages.Dashboard(cards)
	layout := templates.Layout("ReviewHub", component)

	if err := layout.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render dashboard", "error", err)
	}
}

// GetPRDetail renders the PR detail partial for HTMX swap into the main panel.
// Enrichment failures (review, health) are non-fatal: basic PR data is always shown.
func (h *Handler) GetPRDetail(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	numberStr := r.PathValue("number")

	number, err := strconv.Atoi(numberStr)
	if err != nil {
		http.Error(w, "invalid PR number", http.StatusBadRequest)
		return
	}

	repoFullName := owner + "/" + repo

	pr, err := h.prStore.GetByNumber(r.Context(), repoFullName, number)
	if err != nil {
		h.logger.Error("failed to get PR", "repo", repoFullName, "number", number, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if pr == nil {
		http.Error(w, "pull request not found", http.StatusNotFound)
		return
	}

	// Enrich with review data (non-fatal).
	var summary *application.PRReviewSummary
	var botUsernames []string

	if h.reviewSvc != nil {
		summary, err = h.reviewSvc.GetPRReviewSummary(r.Context(), pr.ID, pr.HeadSHA)
		if err != nil {
			h.logger.Error("failed to get review summary", "error", err)
		}

		if summary != nil {
			botUsernames = summary.BotUsernames
		}
	}

	// Enrich with health/CI data (non-fatal).
	var checkRuns []model.CheckRun

	if h.healthSvc != nil {
		healthSummary, healthErr := h.healthSvc.GetPRHealthSummary(r.Context(), pr.ID, pr.RepoFullName, pr.Number)
		if healthErr != nil {
			h.logger.Error("failed to get health summary", "error", healthErr)
		}

		if healthSummary != nil {
			checkRuns = healthSummary.CheckRuns
		}
	}

	detail := toPRDetailViewModel(*pr, summary, checkRuns, botUsernames)
	component := partials.PRDetailContent(detail)

	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render PR detail", "error", err)
	}
}
