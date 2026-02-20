// Package web implements the HTML GUI driving adapter using templ components.
package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	sqliteadapter "github.com/ericfisherdev/mygitpanel/internal/adapter/driven/sqlite"
	"github.com/ericfisherdev/mygitpanel/internal/adapter/driving/web/templates"
	"github.com/ericfisherdev/mygitpanel/internal/adapter/driving/web/templates/components"
	"github.com/ericfisherdev/mygitpanel/internal/adapter/driving/web/templates/pages"
	"github.com/ericfisherdev/mygitpanel/internal/adapter/driving/web/templates/partials"
	vm "github.com/ericfisherdev/mygitpanel/internal/adapter/driving/web/viewmodel"
	"github.com/ericfisherdev/mygitpanel/internal/application"
	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
)

// Handler is the web GUI driving adapter that serves HTML via templ components.
type Handler struct {
	prStore        driven.PRStore
	repoStore      driven.RepoStore
	reviewSvc      *application.ReviewService
	healthSvc      *application.HealthService
	pollSvc        *application.PollService
	attentionSvc   *application.AttentionService
	username       string
	logger         *slog.Logger
	credStore      driven.CredentialStore
	thresholdStore driven.ThresholdStore
	ignoreStore    driven.IgnoreStore
	// writerFactory creates a fresh GitHubWriter per request using the current token,
	// allowing credentials updated via the GUI to take effect without restarting.
	writerFactory func(token string) driven.GitHubWriter
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
	credStore driven.CredentialStore,
	thresholdStore driven.ThresholdStore,
	ignoreStore driven.IgnoreStore,
	writerFactory func(token string) driven.GitHubWriter,
) *Handler {
	return &Handler{
		prStore:        prStore,
		repoStore:      repoStore,
		reviewSvc:      reviewSvc,
		healthSvc:      healthSvc,
		pollSvc:        pollSvc,
		username:       username,
		logger:         logger,
		credStore:      credStore,
		thresholdStore: thresholdStore,
		ignoreStore:    ignoreStore,
		writerFactory:  writerFactory,
	}
}

// WithAttentionService sets the attention service on the handler after construction.
// This avoids a circular dependency between Handler and AttentionService.
func (h *Handler) WithAttentionService(svc *application.AttentionService) *Handler {
	h.attentionSvc = svc
	return h
}

// Dashboard renders the main dashboard page with PR list in the sidebar.
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	prs, err := h.prStore.ListAll(r.Context())
	if err != nil {
		h.logger.Error("failed to list PRs", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	repos, err := h.repoStore.ListAll(r.Context())
	if err != nil {
		h.logger.Error("failed to list repos", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	ignoredPRs, err := h.prStore.ListIgnoredWithPRData(r.Context())
	if err != nil {
		h.logger.Warn("failed to list ignored PRs", "error", err)
		ignoredPRs = nil
	}

	globalSettings := h.getGlobalSettings(r.Context())

	// Ensure CSRF cookie is set for mutating requests.
	csrfToken(w, r)

	cards := h.toPRCardViewModelsWithSignals(r.Context(), prs)
	data := buildDashboardViewModel(cards, repos, ignoredPRs, globalSettings)
	component := pages.Dashboard(data)
	layout := templates.Layout("ReviewHub", component, globalSettings)

	if err := layout.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render dashboard", "error", err)
	}
}

// SearchPRs handles HTMX search/filter requests and returns an updated PR list partial.
func (h *Handler) SearchPRs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	status := r.URL.Query().Get("status")
	repo := r.URL.Query().Get("repo")

	prs, err := h.prStore.ListAll(r.Context())
	if err != nil {
		h.logger.Error("failed to list PRs for search", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	filtered := filterPRs(prs, query, status, repo)
	cards := h.toPRCardViewModelsWithSignals(r.Context(), filtered)
	component := partials.PRList(cards, nil)

	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render search results", "error", err)
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

	detail := toPRDetailViewModel(*pr, summary, checkRuns, botUsernames, h.authenticatedUsername(r.Context()))
	component := partials.PRDetailContent(detail)

	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render PR detail", "error", err)
	}
}

// AddRepo adds a repo to the watch list via the GUI form and returns updated partials.
func (h *Handler) AddRepo(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	fullName := strings.TrimSpace(r.FormValue("full_name"))

	if !isValidRepoName(fullName) {
		http.Error(w, "invalid repository name: expected owner/repo format", http.StatusBadRequest)
		return
	}

	parts := strings.SplitN(fullName, "/", 2)
	repo := model.Repository{
		FullName: fullName,
		Owner:    parts[0],
		Name:     parts[1],
		AddedAt:  time.Now().UTC(),
	}

	if err := h.repoStore.Add(r.Context(), repo); err != nil {
		if errors.Is(err, driven.ErrRepoAlreadyExists) {
			http.Error(w, "repository already exists", http.StatusConflict)
			return
		}
		h.logger.Error("failed to add repo", "repo", fullName, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Fire-and-forget async refresh.
	if h.pollSvc != nil {
		go func() { //nolint:contextcheck // intentional background context for fire-and-forget
			if err := h.pollSvc.RefreshRepo(context.Background(), fullName); err != nil {
				h.logger.Error("async repo refresh failed", "repo", fullName, "error", err)
			}
		}()
	}

	h.renderRepoMutationResponse(w, r)
}

// RemoveRepo removes a repo from the watch list via the GUI and returns updated partials.
func (h *Handler) RemoveRepo(w http.ResponseWriter, r *http.Request) {
	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	fullName := owner + "/" + repo

	if err := h.repoStore.Remove(r.Context(), fullName); err != nil {
		if errors.Is(err, driven.ErrRepoNotFound) {
			http.Error(w, "repository not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to remove repo", "repo", fullName, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.renderRepoMutationResponse(w, r)
}

// renderRepoMutationResponse renders the updated repo list with OOB swaps for PR list
// and repo filter dropdown after an add or remove operation.
func (h *Handler) renderRepoMutationResponse(w http.ResponseWriter, r *http.Request) {
	repos, err := h.repoStore.ListAll(r.Context())
	if err != nil {
		h.logger.Error("failed to list repos after mutation", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	prs, err := h.prStore.ListAll(r.Context())
	if err != nil {
		h.logger.Error("failed to list PRs after repo mutation", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	repoVMs := toRepoViewModels(repos)
	cards := h.toPRCardViewModelsWithSignals(r.Context(), prs)
	repoNames := extractRepoNames(repos)

	ignoredPRs, ignoredErr := h.prStore.ListIgnoredWithPRData(r.Context())
	if ignoredErr != nil {
		h.logger.Warn("failed to list ignored PRs for OOB swap", "error", ignoredErr)
		ignoredPRs = nil
	}

	// Primary target: repo list.
	repoListComp := partials.RepoList(repoVMs)
	if err := repoListComp.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render repo list", "error", err)
		return
	}

	// OOB swap: PR list.
	prListComp := partials.PRListOOB(cards, ignoredPRs)
	if err := prListComp.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render OOB PR list", "error", err)
		return
	}

	// OOB swap: repo filter dropdown.
	filterComp := components.RepoFilterOptions(repoNames)
	if err := filterComp.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render OOB repo filter", "error", err)
	}
}

// IgnorePR handles POST /app/prs/{id}/ignore.
// It marks a PR as ignored and returns an OOB swap to refresh the PR list.
func (h *Handler) IgnorePR(w http.ResponseWriter, r *http.Request) {
	var action func(context.Context, int64) error
	if h.ignoreStore != nil {
		action = h.ignoreStore.Ignore
	}
	h.handleIgnoreToggle(w, r, action, "failed to ignore PR")
}

// UnignorePR handles POST /app/prs/{id}/unignore.
// It removes a PR from the ignore list and returns an OOB swap to refresh the PR list.
func (h *Handler) UnignorePR(w http.ResponseWriter, r *http.Request) {
	var action func(context.Context, int64) error
	if h.ignoreStore != nil {
		action = h.ignoreStore.Unignore
	}
	h.handleIgnoreToggle(w, r, action, "failed to unignore PR")
}

// handleIgnoreToggle is the shared implementation for IgnorePR and UnignorePR.
// action is called with the parsed PR ID if non-nil; pass nil to skip the store call.
func (h *Handler) handleIgnoreToggle(w http.ResponseWriter, r *http.Request, action func(context.Context, int64) error, logMsg string) {
	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid PR ID", http.StatusBadRequest)
		return
	}

	if action != nil {
		if err := action(r.Context(), id); err != nil {
			h.logger.Error(logMsg, "pr_id", id, "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}

	h.renderPRListOOBResponse(w, r)
}

// SaveGlobalThresholds handles POST /app/settings/thresholds/global.
// It parses and persists global threshold settings, then returns an OOB PR list refresh.
func (h *Handler) SaveGlobalThresholds(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: invalid form data</span>`)
		return
	}

	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	reviewCount, _ := strconv.Atoi(r.FormValue("review_count_threshold"))
	if reviewCount < 0 {
		reviewCount = 0
	}
	ageDays, _ := strconv.Atoi(r.FormValue("age_urgency_days"))
	if ageDays < 0 {
		ageDays = 0
	}
	staleEnabled := r.FormValue("stale_review_enabled") == "on" || r.FormValue("stale_review_enabled") == "1"
	ciEnabled := r.FormValue("ci_failure_enabled") == "on" || r.FormValue("ci_failure_enabled") == "1"

	settings := model.GlobalSettings{
		ReviewCountThreshold: reviewCount,
		AgeUrgencyDays:       ageDays,
		StaleReviewEnabled:   staleEnabled,
		CIFailureEnabled:     ciEnabled,
	}

	if h.thresholdStore != nil {
		if err := h.thresholdStore.SetGlobalSettings(r.Context(), settings); err != nil {
			h.logger.Error("failed to save global thresholds", "error", err)
			fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: failed to save settings</span>`)
			return
		}
	}

	// Status fragment for hx-target="#threshold-status".
	fmt.Fprintf(w, `<span class="text-green-600 text-sm">Thresholds saved</span>`)

	// OOB swap: refresh PR list with updated signals.
	h.renderPRListOOB(w, r)
}

// SaveRepoThreshold handles POST /app/settings/thresholds/repo.
// It parses and persists per-repo threshold overrides.
func (h *Handler) SaveRepoThreshold(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: invalid form data</span>`)
		return
	}

	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	repoFullName := strings.TrimSpace(r.FormValue("repo_full_name"))
	if repoFullName == "" {
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: repo name required</span>`)
		return
	}

	threshold := model.RepoThreshold{RepoFullName: repoFullName}

	if v := r.FormValue("review_count"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n >= 0 {
			threshold.ReviewCount = &n
		}
	}
	if v := r.FormValue("age_urgency_days"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n >= 0 {
			threshold.AgeUrgencyDays = &n
		}
	}

	if h.thresholdStore != nil {
		if err := h.thresholdStore.SetRepoThreshold(r.Context(), threshold); err != nil {
			h.logger.Error("failed to save repo threshold", "repo", repoFullName, "error", err)
			fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: failed to save settings</span>`)
			return
		}
	}

	fmt.Fprintf(w, `<span class="text-green-600 text-sm">Saved</span>`)

	// OOB swap: refresh PR list with updated signals.
	h.renderPRListOOB(w, r)
}

// DeleteRepoThreshold handles DELETE /app/settings/thresholds/repo/{owner}/{repo}.
// It removes the per-repo override and returns a success fragment + OOB PR list swap.
func (h *Handler) DeleteRepoThreshold(w http.ResponseWriter, r *http.Request) {
	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	repoFullName := owner + "/" + repo

	if h.thresholdStore != nil {
		if err := h.thresholdStore.DeleteRepoThreshold(r.Context(), repoFullName); err != nil {
			h.logger.Error("failed to delete repo threshold", "repo", repoFullName, "error", err)
			fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: failed to reset</span>`)
			return
		}
	}

	fmt.Fprintf(w, `<span class="text-green-600 text-sm">Reset to global defaults</span>`)

	// OOB swap: refresh PR list with updated signals.
	h.renderPRListOOB(w, r)
}

// renderPRListOOBResponse renders just the PR list OOB swap after an ignore/unignore.
// The primary response is the empty OOB swap itself (no separate primary target).
func (h *Handler) renderPRListOOBResponse(w http.ResponseWriter, r *http.Request) {
	h.renderPRListOOB(w, r)
}

// renderPRListOOB fetches the current PR list and ignored PRs and writes an OOB swap.
func (h *Handler) renderPRListOOB(w http.ResponseWriter, r *http.Request) {
	prs, err := h.prStore.ListAll(r.Context())
	if err != nil {
		h.logger.Error("failed to list PRs for OOB swap", "error", err)
		return
	}

	ignoredPRs, err := h.prStore.ListIgnoredWithPRData(r.Context())
	if err != nil {
		h.logger.Warn("failed to list ignored PRs for OOB swap", "error", err)
		ignoredPRs = nil
	}

	cards := h.toPRCardViewModelsWithSignals(r.Context(), prs)
	prListComp := partials.PRListOOB(cards, ignoredPRs)
	if err := prListComp.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render OOB PR list", "error", err)
	}
}

// filterPRs applies text search, status, and repo filters to a slice of PRs.
func filterPRs(prs []model.PullRequest, query, status, repo string) []model.PullRequest {
	filtered := make([]model.PullRequest, 0, len(prs))
	queryLower := strings.ToLower(strings.TrimSpace(query))

	for _, pr := range prs {
		if status != "" && status != "all" && string(pr.Status) != status {
			continue
		}
		if repo != "" && repo != "all" && pr.RepoFullName != repo {
			continue
		}
		if queryLower != "" && !matchesPRQuery(pr, queryLower) {
			continue
		}
		filtered = append(filtered, pr)
	}

	return filtered
}

// matchesPRQuery checks if any searchable PR field contains the query substring.
func matchesPRQuery(pr model.PullRequest, queryLower string) bool {
	return strings.Contains(strings.ToLower(pr.Title), queryLower) ||
		strings.Contains(strings.ToLower(pr.Author), queryLower) ||
		strings.Contains(strings.ToLower(pr.RepoFullName), queryLower) ||
		strings.Contains(strings.ToLower(pr.Branch), queryLower)
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

// buildDashboardViewModel constructs the full view model for the dashboard page.
func buildDashboardViewModel(cards []vm.PRCardViewModel, repos []model.Repository, ignoredPRs []model.PullRequest, globalSettings model.GlobalSettings) vm.DashboardViewModel {
	ignoredCards := make([]vm.PRCardViewModel, 0, len(ignoredPRs))
	for _, pr := range ignoredPRs {
		// Ignored PRs show with zero-value attention signals in the ignore list.
		ignoredCards = append(ignoredCards, toPRCardViewModel(pr, model.AttentionSignals{}))
	}

	return vm.DashboardViewModel{
		Cards:          cards,
		Repos:          toRepoViewModels(repos),
		RepoNames:      extractRepoNames(repos),
		IgnoredPRs:     ignoredCards,
		GlobalSettings: globalSettings,
	}
}

// toPRCardViewModelsWithSignals converts PRs to card view models, computing attention signals for each.
// On signal computation failure, falls back to zero-value signals (non-fatal).
func (h *Handler) toPRCardViewModelsWithSignals(ctx context.Context, prs []model.PullRequest) []vm.PRCardViewModel {
	cards := make([]vm.PRCardViewModel, 0, len(prs))
	for _, pr := range prs {
		var signals model.AttentionSignals
		if h.attentionSvc != nil {
			var err error
			signals, err = h.attentionSvc.SignalsForPR(ctx, pr)
			if err != nil {
				h.logger.Warn("failed to compute attention signals, using zero-value", "pr_id", pr.ID, "error", err)
			}
		}
		cards = append(cards, toPRCardViewModel(pr, signals))
	}
	return cards
}

// getGlobalSettings fetches global settings from the threshold store, returning defaults on error.
func (h *Handler) getGlobalSettings(ctx context.Context) model.GlobalSettings {
	if h.thresholdStore == nil {
		return model.DefaultGlobalSettings()
	}
	settings, err := h.thresholdStore.GetGlobalSettings(ctx)
	if err != nil {
		h.logger.Warn("failed to get global settings, using defaults", "error", err)
		return model.DefaultGlobalSettings()
	}
	return settings
}

// toRepoViewModels converts domain repos to presentation view models.
func toRepoViewModels(repos []model.Repository) []vm.RepoViewModel {
	vms := make([]vm.RepoViewModel, 0, len(repos))
	for _, r := range repos {
		vms = append(vms, vm.RepoViewModel{
			FullName:   r.FullName,
			Owner:      r.Owner,
			Name:       r.Name,
			DeletePath: fmt.Sprintf("/app/repos/%s/%s", r.Owner, r.Name),
		})
	}
	return vms
}

// extractRepoNames returns the distinct full names from a slice of repositories.
func extractRepoNames(repos []model.Repository) []string {
	names := make([]string, 0, len(repos))
	for _, r := range repos {
		names = append(names, r.FullName)
	}
	return names
}

// SaveGitHubCredentials handles POST /app/settings/github.
// It validates the provided token against the GitHub API before storing it.
// The response is an HTML fragment injected into #cred-github-status by HTMX.
func (h *Handler) SaveGitHubCredentials(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: invalid form data</span>`)
		return
	}

	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	token := strings.TrimSpace(r.FormValue("github_token"))

	if token == "" {
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: GitHub token is required</span>`)
		return
	}

	if h.writerFactory == nil || h.credStore == nil {
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: credential storage is not configured</span>`)
		return
	}

	// Validate the token against the GitHub API.
	// A token-less writer is sufficient for validation since we pass the token explicitly.
	validatedUsername, err := h.writerFactory("").ValidateToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, sqliteadapter.ErrEncryptionKeyNotSet) {
			fmt.Fprintf(w, `<span class="text-red-600 text-sm">Credential storage requires MYGITPANEL_SECRET_KEY to be set</span>`)
			return
		}
		h.logger.Error("github token validation failed", "error", err)
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: %s</span>`, err.Error())
		return
	}

	// Store the validated token.
	if err := h.credStore.Set(r.Context(), "github_token", token); err != nil {
		if errors.Is(err, sqliteadapter.ErrEncryptionKeyNotSet) {
			fmt.Fprintf(w, `<span class="text-red-600 text-sm">Credential storage requires MYGITPANEL_SECRET_KEY to be set</span>`)
			return
		}
		h.logger.Error("failed to store github token", "error", err)
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: failed to save token</span>`)
		return
	}

	// Always store the validated username for auth checks (don't allow user overrides).
	if err := h.credStore.Set(r.Context(), "github_username", validatedUsername); err != nil {
		h.logger.Error("failed to store github username", "error", err)
		// Non-fatal: token was saved successfully; username storage failure is logged.
	}

	fmt.Fprintf(w, `<span class="text-green-600 text-sm">GitHub token: configured (%s)</span>`, validatedUsername)
}

// SaveJiraCredentials handles POST /app/settings/jira.
// It stores Jira credentials without validation (Phase 9 will add validation).
// The response is an HTML fragment injected into #cred-jira-status by HTMX.
func (h *Handler) SaveJiraCredentials(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: invalid form data</span>`)
		return
	}

	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	jiraURL := strings.TrimSpace(r.FormValue("jira_url"))
	jiraEmail := strings.TrimSpace(r.FormValue("jira_email"))
	jiraToken := strings.TrimSpace(r.FormValue("jira_token"))

	if h.credStore == nil {
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: credential storage is not configured</span>`)
		return
	}

	// Store all three Jira fields without validation.
	if err := h.credStore.Set(r.Context(), "jira_url", jiraURL); err != nil {
		if errors.Is(err, sqliteadapter.ErrEncryptionKeyNotSet) {
			fmt.Fprintf(w, `<span class="text-red-600 text-sm">Credential storage requires MYGITPANEL_SECRET_KEY to be set</span>`)
			return
		}
		h.logger.Error("failed to store jira_url", "error", err)
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: failed to save credentials</span>`)
		return
	}

	if err := h.credStore.Set(r.Context(), "jira_email", jiraEmail); err != nil {
		h.logger.Error("failed to store jira_email", "error", err)
	}

	if err := h.credStore.Set(r.Context(), "jira_token", jiraToken); err != nil {
		h.logger.Error("failed to store jira_token", "error", err)
	}

	fmt.Fprintf(w, `<span class="text-green-600 text-sm">Jira credentials saved</span>`)
}

// CreateReplyComment handles POST /app/prs/{owner}/{repo}/{number}/comments/{rootID}/reply.
// It creates a reply to an existing review thread and re-renders the thread via morph swap.
func (h *Handler) CreateReplyComment(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	numberStr := r.PathValue("number")
	rootIDStr := r.PathValue("rootID")

	number, err := strconv.Atoi(numberStr)
	if err != nil {
		http.Error(w, "invalid PR number", http.StatusBadRequest)
		return
	}

	rootID, err := strconv.ParseInt(rootIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid comment ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: invalid form data</p>`)
		return
	}

	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	body := strings.TrimSpace(r.FormValue("body"))
	commitSHA := r.FormValue("commit_sha")
	filePath := r.FormValue("path")

	if body == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: reply body cannot be empty</p>`)
		return
	}

	// Authenticate: check for GitHub token.
	if h.credStore == nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Configure a GitHub token in Settings to reply to comments.</p>`)
		return
	}

	token, err := h.credStore.Get(r.Context(), "github_token")
	if err != nil || token == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Configure a GitHub token in Settings to reply to comments.</p>`)
		return
	}

	repoFullName := owner + "/" + repo
	writer := h.writerFactory(token)

	if err := writer.CreateReplyComment(r.Context(), repoFullName, number, rootID, body, filePath, commitSHA); err != nil {
		h.logger.Error("failed to create reply comment", "repo", repoFullName, "pr", number, "error", err)
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: %s</p>`, err.Error())
		return
	}

	// Re-fetch PR and render the updated thread for morph swap targeting #thread-{rootID}.
	h.renderThreadAfterReply(w, r, repoFullName, number, rootID, owner, repo)
}

// renderThreadAfterReply fetches updated PR review data and renders just the
// updated thread component for morph swap targeting #thread-{rootID}.
func (h *Handler) renderThreadAfterReply(w http.ResponseWriter, r *http.Request, repoFullName string, prNumber int, rootID int64, owner, repo string) {
	pr, err := h.prStore.GetByNumber(r.Context(), repoFullName, prNumber)
	if err != nil || pr == nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: failed to load PR after reply</p>`)
		return
	}

	var summary *application.PRReviewSummary
	if h.reviewSvc != nil {
		summary, err = h.reviewSvc.GetPRReviewSummary(r.Context(), pr.ID, pr.HeadSHA)
		if err != nil {
			h.logger.Error("failed to get review summary after reply", "error", err)
		}
	}

	var botUsernames []string
	if summary != nil {
		botUsernames = summary.BotUsernames
	}

	detail := toPRDetailViewModel(*pr, summary, nil, botUsernames, h.authenticatedUsername(r.Context()))

	// Find the specific thread to re-render.
	for _, thread := range detail.Threads {
		if thread.RootComment.ID == rootID {
			comp := components.ReviewThread(thread, owner, repo, prNumber)
			if err := comp.Render(r.Context(), w); err != nil {
				h.logger.Error("failed to render review thread", "error", err)
			}
			return
		}
	}

	// Thread not found — fallback: re-render the whole reviews section.
	h.renderReviewsSection(w, r, detail, owner, repo)
}

// SubmitReview handles POST /app/prs/{owner}/{repo}/{number}/review.
// It submits a pull request review and re-renders the full reviews section.
func (h *Handler) SubmitReview(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	numberStr := r.PathValue("number")

	number, err := strconv.Atoi(numberStr)
	if err != nil {
		http.Error(w, "invalid PR number", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: invalid form data</p>`)
		return
	}

	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	body := r.FormValue("body")
	event := r.FormValue("event")
	commitSHA := r.FormValue("commit_sha")
	commentsJSON := r.FormValue("comments")

	// Validate event.
	switch event {
	case "APPROVE", "REQUEST_CHANGES", "COMMENT":
		// valid
	default:
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: invalid review event; must be APPROVE, REQUEST_CHANGES, or COMMENT</p>`)
		return
	}

	// Decode pending line comments (empty array and empty string are both valid).
	var lineComments []driven.DraftLineComment
	if commentsJSON != "" && commentsJSON != "null" {
		if err := json.Unmarshal([]byte(commentsJSON), &lineComments); err != nil {
			h.logger.Error("failed to decode line comments JSON", "error", err)
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: invalid pending comments format</p>`)
			return
		}
	}

	// Authenticate: check for GitHub token.
	if h.credStore == nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Configure a GitHub token in Settings to submit reviews.</p>`)
		return
	}

	token, err := h.credStore.Get(r.Context(), "github_token")
	if err != nil || token == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Configure a GitHub token in Settings to submit reviews.</p>`)
		return
	}

	repoFullName := owner + "/" + repo
	writer := h.writerFactory(token)

	req := driven.ReviewRequest{
		CommitID: commitSHA,
		Event:    event,
		Body:     body,
		Comments: lineComments,
	}

	if err := writer.SubmitReview(r.Context(), repoFullName, number, req); err != nil {
		h.logger.Error("failed to submit review", "repo", repoFullName, "pr", number, "error", err)
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: %s</p>`, err.Error())
		return
	}

	// Re-fetch and re-render the full reviews section for morph swap.
	h.renderReviewsSectionForPR(w, r, repoFullName, number, owner, repo)
}

// CreateIssueComment handles POST /app/prs/{owner}/{repo}/{number}/issue-comments.
// It creates a general PR comment and re-renders the full reviews section.
func (h *Handler) CreateIssueComment(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	numberStr := r.PathValue("number")

	number, err := strconv.Atoi(numberStr)
	if err != nil {
		http.Error(w, "invalid PR number", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: invalid form data</p>`)
		return
	}

	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	body := strings.TrimSpace(r.FormValue("body"))
	if body == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: comment body cannot be empty</p>`)
		return
	}

	// Authenticate: check for GitHub token.
	if h.credStore == nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Configure a GitHub token in Settings to post comments.</p>`)
		return
	}

	token, err := h.credStore.Get(r.Context(), "github_token")
	if err != nil || token == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Configure a GitHub token in Settings to post comments.</p>`)
		return
	}

	repoFullName := owner + "/" + repo
	writer := h.writerFactory(token)

	if err := writer.CreateIssueComment(r.Context(), repoFullName, number, body); err != nil {
		h.logger.Error("failed to create issue comment", "repo", repoFullName, "pr", number, "error", err)
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: %s</p>`, err.Error())
		return
	}

	// Re-fetch and re-render the full reviews section for morph swap.
	h.renderReviewsSectionForPR(w, r, repoFullName, number, owner, repo)
}

// ToggleDraftStatus handles POST /app/prs/{owner}/{repo}/{number}/draft-toggle.
// It converts a ready-for-review PR to draft (or vice-versa) and morphs the header section.
func (h *Handler) ToggleDraftStatus(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	numberStr := r.PathValue("number")

	number, err := strconv.Atoi(numberStr)
	if err != nil {
		http.Error(w, "invalid PR number", http.StatusBadRequest)
		return
	}

	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	// Authenticate: check for GitHub token.
	if h.credStore == nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Configure a GitHub token in Settings to toggle draft status.</p>`)
		return
	}

	token, err := h.credStore.Get(r.Context(), "github_token")
	if err != nil || token == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Configure a GitHub token in Settings to toggle draft status.</p>`)
		return
	}

	repoFullName := owner + "/" + repo

	pr, err := h.prStore.GetByNumber(r.Context(), repoFullName, number)
	if err != nil {
		h.logger.Error("failed to get PR for draft toggle", "repo", repoFullName, "number", number, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: failed to load PR data</p>`)
		return
	}
	if pr == nil {
		http.Error(w, "pull request not found", http.StatusNotFound)
		return
	}

	// Server-side author check: only the PR author can toggle draft status.
	authUser := h.authenticatedUsername(r.Context())
	if authUser == "" || pr.Author != authUser {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: only the PR author can toggle draft status</p>`)
		return
	}

	// Execute the appropriate mutation based on current draft state.
	writer := h.writerFactory(token)
	if pr.IsDraft {
		err = writer.MarkPullRequestReadyForReview(r.Context(), repoFullName, number)
	} else {
		err = writer.ConvertPullRequestToDraft(r.Context(), repoFullName, number)
	}
	if err != nil {
		h.logger.Error("failed to toggle draft status", "repo", repoFullName, "pr", number, "error", err)
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: %s</p>`, err.Error())
		return
	}

	// Fire-and-forget background refresh so the DB catches up with new draft state.
	if h.pollSvc != nil {
		go func() { //nolint:contextcheck // intentional background context for fire-and-forget
			if err := h.pollSvc.RefreshRepo(context.Background(), repoFullName); err != nil {
				h.logger.Error("async repo refresh after draft toggle failed", "repo", repoFullName, "error", err)
			}
		}()
	}

	// Re-fetch PR for rendering (optimistically flip draft state since refresh is async).
	updatedPR, err := h.prStore.GetByNumber(r.Context(), repoFullName, number)
	if err != nil || updatedPR == nil {
		updatedPR = pr // Fallback to pre-toggle PR if re-fetch fails.
	}

	// Optimistically flip the draft state in the view model so the UI reflects
	// the change immediately without waiting for the async poll to complete.
	flipped := *updatedPR
	if flipped.IsDraft == pr.IsDraft {
		flipped.IsDraft = !pr.IsDraft
	}

	detail := toPRDetailViewModel(flipped, nil, nil, nil, authUser)
	comp := components.PRDetailHeader(detail)
	if err := comp.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render PR detail header after draft toggle", "error", err)
	}
}

// renderReviewsSectionForPR fetches the PR and its review data, then renders
// the full PRReviewsSection component for a morph swap targeting #pr-reviews-section.
func (h *Handler) renderReviewsSectionForPR(w http.ResponseWriter, r *http.Request, repoFullName string, prNumber int, owner, repo string) {
	pr, err := h.prStore.GetByNumber(r.Context(), repoFullName, prNumber)
	if err != nil || pr == nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: failed to load PR data</p>`)
		return
	}

	var summary *application.PRReviewSummary
	if h.reviewSvc != nil {
		summary, err = h.reviewSvc.GetPRReviewSummary(r.Context(), pr.ID, pr.HeadSHA)
		if err != nil {
			h.logger.Error("failed to get review summary after submit", "error", err)
		}
	}

	var botUsernames []string
	if summary != nil {
		botUsernames = summary.BotUsernames
	}

	detail := toPRDetailViewModel(*pr, summary, nil, botUsernames, h.authenticatedUsername(r.Context()))
	h.renderReviewsSection(w, r, detail, owner, repo)
}

// authenticatedUsername returns the currently authenticated GitHub username.
// It checks the credential store first (dynamic credentials set via GUI), then
// falls back to the static username from configuration.
func (h *Handler) authenticatedUsername(ctx context.Context) string {
	if h.credStore != nil {
		username, err := h.credStore.Get(ctx, "github_username")
		if err == nil && username != "" {
			return username
		}
	}
	return h.username
}

// renderReviewsSection renders the PRReviewsSection component to the response writer.
func (h *Handler) renderReviewsSection(w http.ResponseWriter, r *http.Request, detail vm.PRDetailViewModel, owner, repo string) {
	comp := components.PRReviewsSection(detail, owner, repo)
	if err := comp.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render reviews section", "error", err)
	}
}
