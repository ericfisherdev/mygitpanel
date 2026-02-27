// Package web implements the HTML GUI driving adapter using templ components.
package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ericfisherdev/mygitpanel/internal/adapter/driving/web/templates"
	"github.com/ericfisherdev/mygitpanel/internal/adapter/driving/web/templates/components"
	"github.com/ericfisherdev/mygitpanel/internal/adapter/driving/web/templates/pages"
	"github.com/ericfisherdev/mygitpanel/internal/adapter/driving/web/templates/partials"
	vm "github.com/ericfisherdev/mygitpanel/internal/adapter/driving/web/viewmodel"
	"github.com/ericfisherdev/mygitpanel/internal/application"
	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
	"github.com/ericfisherdev/mygitpanel/internal/validate"
)

// HTTP error message constants shared across all handlers.
const (
	errMsgInvalidPRNumber = "invalid PR number"
	errMsgCSRFInvalid     = "invalid CSRF token"
	errMsgInvalidFormData = "invalid form data"
	errMsgServiceUnavail  = "service unavailable"
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
	// jiraConnStore manages multi-connection Jira credential persistence.
	jiraConnStore driven.JiraConnectionStore
	// jiraClientFactory creates a JiraClient for a given connection, enabling
	// credential validation (Ping) without coupling to concrete adapter.
	jiraClientFactory func(conn model.JiraConnection) driven.JiraClient
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
	jiraConnStore driven.JiraConnectionStore,
	jiraClientFactory func(conn model.JiraConnection) driven.JiraClient,
) *Handler {
	return &Handler{
		prStore:           prStore,
		repoStore:         repoStore,
		reviewSvc:         reviewSvc,
		healthSvc:         healthSvc,
		pollSvc:           pollSvc,
		username:          username,
		logger:            logger,
		credStore:         credStore,
		thresholdStore:    thresholdStore,
		ignoreStore:       ignoreStore,
		writerFactory:     writerFactory,
		jiraConnStore:     jiraConnStore,
		jiraClientFactory: jiraClientFactory,
	}
}

// WithAttentionService injects AttentionService after construction to keep NewHandler's
// parameter list minimal and improve testability by allowing the service to be omitted in tests.
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
	data := h.buildDashboardViewModel(r.Context(), cards, repos, ignoredPRs, globalSettings)
	component := pages.Dashboard(data)
	layout := templates.Layout("ReviewHub", component, globalSettings, data.JiraConnections)

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
		http.Error(w, errMsgInvalidPRNumber, http.StatusBadRequest)
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

	// Jira enrichment (non-fatal — errors populate LoadError, never prevent the detail from rendering).
	detail.JiraCard = h.buildJiraCardVM(r.Context(), *pr, owner, repo, number)

	component := partials.PRDetailContent(detail)

	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render PR detail", "error", err)
	}
}

// buildJiraCardVM resolves the Jira connection for this repo and fetches
// the linked issue if a key exists. All errors are captured in LoadError;
// this method never returns an error — Jira failures must not block PR rendering.
func (h *Handler) buildJiraCardVM(ctx context.Context, pr model.PullRequest, owner, repo string, number int) vm.JiraCardViewModel {
	base := vm.JiraCardViewModel{
		Owner:   owner,
		Repo:    repo,
		Number:  number,
		JiraKey: pr.JiraKey,
	}

	if h.jiraConnStore == nil || h.jiraClientFactory == nil {
		return base // no Jira integration configured
	}

	conn, err := h.jiraConnStore.GetForRepo(ctx, pr.RepoFullName)
	if err != nil {
		h.logger.Error("jira: getForRepo failed", "repo", pr.RepoFullName, "error", err)
		return base
	}

	if conn.ID == 0 {
		return base // no connection assigned or defaulted for this repo
	}

	base.HasCredentials = true

	if pr.JiraKey == "" {
		return base // no key detected
	}

	client := h.jiraClientFactory(conn)
	issue, err := client.GetIssue(ctx, pr.JiraKey)
	if err != nil {
		base.LoadError = friendlyJiraError(err)
		return base
	}

	issueVM := &vm.JiraIssueVM{
		Key:         issue.Key,
		Summary:     issue.Summary,
		Description: issue.Description,
		Status:      issue.Status,
		Priority:    issue.Priority,
		Assignee:    issue.Assignee,
		JiraURL:     strings.TrimRight(conn.BaseURL, "/") + "/browse/" + issue.Key,
	}

	for _, c := range issue.Comments {
		issueVM.Comments = append(issueVM.Comments, vm.JiraCommentVM{
			Author:    c.Author,
			Body:      c.Body,
			CreatedAt: c.CreatedAt.Format("2 Jan 2006"),
		})
	}

	base.Issue = issueVM
	return base
}

// friendlyJiraError maps Jira sentinel errors to user-friendly messages.
func friendlyJiraError(err error) string {
	switch {
	case errors.Is(err, driven.ErrJiraUnauthorized):
		return "Invalid credentials — update in Settings"
	case errors.Is(err, driven.ErrJiraNotFound):
		return "Issue not found in Jira"
	default:
		return "Jira instance unreachable — check connection in Settings"
	}
}

// CreateJiraComment posts a plain-text comment to the linked Jira issue.
// On success, re-renders the JiraCard component (morph swap target: #jira-card).
// Returns 422 HTML fragment on missing creds, unreachable Jira, or invalid key.
func (h *Handler) CreateJiraComment(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	numberStr := r.PathValue("number")

	number, err := strconv.Atoi(numberStr)
	if err != nil {
		http.Error(w, errMsgInvalidPRNumber, http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, errMsgInvalidFormData, http.StatusBadRequest)
		return
	}

	if !validateCSRF(r) {
		http.Error(w, errMsgCSRFInvalid, http.StatusForbidden)
		return
	}

	body := strings.TrimSpace(r.FormValue("body"))
	if body == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Comment body is required</span>`)
		return
	}

	if h.jiraConnStore == nil || h.jiraClientFactory == nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Jira integration not configured</span>`)
		return
	}

	repoFullName := owner + "/" + repo

	pr, err := h.prStore.GetByNumber(r.Context(), repoFullName, number)
	if err != nil || pr == nil {
		h.logger.Error("failed to get PR for jira comment", "repo", repoFullName, "number", number, "error", err)
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Pull request not found</span>`)
		return
	}

	if pr.JiraKey == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">No Jira issue linked to this PR</span>`)
		return
	}

	conn, err := h.jiraConnStore.GetForRepo(r.Context(), pr.RepoFullName)
	if err != nil {
		h.logger.Error("jira: getForRepo failed", "repo", pr.RepoFullName, "error", err)
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Failed to resolve Jira connection</span>`)
		return
	}

	if conn.ID == 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">No Jira connection configured for this repo</span>`)
		return
	}

	client := h.jiraClientFactory(conn)

	// Validate connectivity before posting.
	if err := client.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">%s</span>`, html.EscapeString(friendlyJiraError(err)))
		return
	}

	if err := client.AddComment(r.Context(), pr.JiraKey, body); err != nil {
		h.logger.Error("jira: add comment failed", "key", pr.JiraKey, "error", err)
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Failed to post comment: %s</span>`, html.EscapeString(friendlyJiraError(err)))
		return
	}

	// Re-render JiraCard with updated comment list.
	cardVM := h.buildJiraCardVM(r.Context(), *pr, owner, repo, number)
	// Force expanded state so the user sees their new comment.
	if err := components.JiraCard(cardVM).Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render jira card after comment", "error", err)
	}
}

// AddRepo adds a repo to the watch list via the GUI form and returns updated partials.
func (h *Handler) AddRepo(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, errMsgInvalidFormData, http.StatusBadRequest)
		return
	}

	if !validateCSRF(r) {
		http.Error(w, errMsgCSRFInvalid, http.StatusForbidden)
		return
	}

	fullName := strings.TrimSpace(r.FormValue("full_name"))

	if !validate.IsValidRepoName(fullName) {
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
		http.Error(w, errMsgCSRFInvalid, http.StatusForbidden)
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

	repoVMs := h.toRepoViewModels(r.Context(), repos)
	cards := h.toPRCardViewModelsWithSignals(r.Context(), prs)
	repoNames := extractRepoNames(repos)

	ignoredPRs, ignoredErr := h.prStore.ListIgnoredWithPRData(r.Context())
	if ignoredErr != nil {
		h.logger.Warn("failed to list ignored PRs for OOB swap", "error", ignoredErr)
		ignoredPRs = nil
	}

	// Fetch Jira connections for repo popover assignment dropdown.
	var jiraConnVMs []vm.JiraConnectionViewModel
	if h.jiraConnStore != nil {
		conns, jiraErr := h.jiraConnStore.List(r.Context())
		if jiraErr != nil {
			h.logger.Warn("failed to list jira connections for repo list", "error", jiraErr)
		} else {
			jiraConnVMs = h.toJiraConnectionViewModels(conns)
		}
	}

	// Primary target: repo list.
	repoListComp := partials.RepoList(repoVMs, jiraConnVMs)
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
		http.Error(w, errMsgCSRFInvalid, http.StatusForbidden)
		return
	}
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid PR ID", http.StatusBadRequest)
		return
	}

	if action == nil {
		http.Error(w, errMsgServiceUnavail, http.StatusServiceUnavailable)
		return
	}

	if err := action(r.Context(), id); err != nil {
		h.logger.Error(logMsg, "pr_id", id, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.renderPRListOOB(w, r)
}

// SaveGlobalThresholds handles POST /app/settings/thresholds/global.
// It parses and persists global threshold settings, then returns an OOB PR list refresh.
func (h *Handler) SaveGlobalThresholds(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: invalid form data</span>`)
		return
	}

	if !validateCSRF(r) {
		http.Error(w, errMsgCSRFInvalid, http.StatusForbidden)
		return
	}

	reviewCount, err := strconv.Atoi(r.FormValue("review_count_threshold"))
	if err != nil {
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: review_count_threshold must be a number</span>`)
		return
	}
	if reviewCount < 0 {
		reviewCount = 0
	}
	ageDays, err := strconv.Atoi(r.FormValue("age_urgency_days"))
	if err != nil {
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: age_urgency_days must be a number</span>`)
		return
	}
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

	if h.thresholdStore == nil {
		http.Error(w, errMsgServiceUnavail, http.StatusServiceUnavailable)
		return
	}

	if err := h.thresholdStore.SetGlobalSettings(r.Context(), settings); err != nil {
		h.logger.Error("failed to save global thresholds", "error", err)
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: failed to save settings</span>`)
		return
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
		http.Error(w, errMsgCSRFInvalid, http.StatusForbidden)
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
	switch r.FormValue("stale_review_enabled") {
	case "true":
		b := true
		threshold.StaleReviewEnabled = &b
	case "false":
		b := false
		threshold.StaleReviewEnabled = &b
		// "inherit" and "" → nil (no override)
	}
	switch r.FormValue("ci_failure_enabled") {
	case "true":
		b := true
		threshold.CIFailureEnabled = &b
	case "false":
		b := false
		threshold.CIFailureEnabled = &b
		// "inherit" and "" → nil (no override)
	}

	if h.thresholdStore == nil {
		http.Error(w, errMsgServiceUnavail, http.StatusServiceUnavailable)
		return
	}

	if err := h.thresholdStore.SetRepoThreshold(r.Context(), threshold); err != nil {
		h.logger.Error("failed to save repo threshold", "repo", repoFullName, "error", err)
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: failed to save settings</span>`)
		return
	}

	fmt.Fprintf(w, `<span class="text-green-600 text-sm">Saved</span>`)

	// OOB swap: refresh PR list with updated signals.
	h.renderPRListOOB(w, r)
}

// DeleteRepoThreshold handles DELETE /app/settings/thresholds/repo/{owner}/{repo}.
// It removes the per-repo override and returns a success fragment + OOB PR list swap.
func (h *Handler) DeleteRepoThreshold(w http.ResponseWriter, r *http.Request) {
	if !validateCSRF(r) {
		http.Error(w, errMsgCSRFInvalid, http.StatusForbidden)
		return
	}
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	repoFullName := owner + "/" + repo

	if h.thresholdStore == nil {
		http.Error(w, errMsgServiceUnavail, http.StatusServiceUnavailable)
		return
	}

	if err := h.thresholdStore.DeleteRepoThreshold(r.Context(), repoFullName); err != nil {
		h.logger.Error("failed to delete repo threshold", "repo", repoFullName, "error", err)
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: failed to reset</span>`)
		return
	}

	fmt.Fprintf(w, `<span class="text-green-600 text-sm">Reset to global defaults</span>`)

	// OOB swap: refresh PR list with updated signals.
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

// buildDashboardViewModel constructs the full view model for the dashboard page.
func (h *Handler) buildDashboardViewModel(ctx context.Context, cards []vm.PRCardViewModel, repos []model.Repository, ignoredPRs []model.PullRequest, globalSettings model.GlobalSettings) vm.DashboardViewModel {
	ignoredCards := make([]vm.PRCardViewModel, 0, len(ignoredPRs))
	for _, pr := range ignoredPRs {
		// Ignored PRs show with zero-value attention signals in the ignore list.
		ignoredCards = append(ignoredCards, toPRCardViewModel(pr, model.AttentionSignals{}))
	}

	// Fetch Jira connections for the dashboard (settings drawer pre-population).
	var jiraConnVMs []vm.JiraConnectionViewModel
	if h.jiraConnStore != nil {
		conns, err := h.jiraConnStore.List(ctx)
		if err != nil {
			h.logger.Warn("failed to list jira connections for dashboard", "error", err)
		} else {
			jiraConnVMs = h.toJiraConnectionViewModels(conns)
		}
	}

	return vm.DashboardViewModel{
		Cards:           cards,
		Repos:           h.toRepoViewModels(ctx, repos),
		RepoNames:       extractRepoNames(repos),
		IgnoredPRs:      ignoredCards,
		GlobalSettings:  globalSettings,
		JiraConnections: jiraConnVMs,
	}
}

// toPRCardViewModelsWithSignals converts PRs to card view models, computing attention signals for each.
// Thresholds are resolved once per unique repo to avoid N+1 DB lookups. On signal computation
// failure, falls back to zero-value signals (non-fatal).
func (h *Handler) toPRCardViewModelsWithSignals(ctx context.Context, prs []model.PullRequest) []vm.PRCardViewModel {
	// Pre-fetch thresholds once per unique repo.
	thresholdsByRepo := make(map[string]model.EffectiveThresholds, len(prs))
	if h.attentionSvc != nil {
		for _, pr := range prs {
			if _, seen := thresholdsByRepo[pr.RepoFullName]; !seen {
				thresholdsByRepo[pr.RepoFullName] = h.attentionSvc.EffectiveThresholdsFor(ctx, pr.RepoFullName)
			}
		}
	}

	cards := make([]vm.PRCardViewModel, 0, len(prs))
	for _, pr := range prs {
		var signals model.AttentionSignals
		if h.attentionSvc != nil {
			var err error
			signals, err = h.attentionSvc.SignalsForPR(ctx, pr, thresholdsByRepo[pr.RepoFullName])
			if err != nil {
				h.logger.Warn("failed to compute attention signals, using zero-value", "pr_id", pr.ID, "error", err)
			}
		}
		cards = append(cards, toPRCardViewModel(pr, signals))
	}
	return cards
}

// requireGitHubToken retrieves and validates the stored GitHub token.
// It writes an HTML error fragment and returns "" when the token is unavailable;
// callers must return immediately when the result is "".
// action describes the operation (e.g. "reply to comments") and is HTML-escaped in error output.
func (h *Handler) requireGitHubToken(w http.ResponseWriter, r *http.Request, action string) string {
	safeAction := html.EscapeString(action)
	if h.credStore == nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Configure a GitHub token in Settings to %s.</p>`, safeAction)
		return ""
	}
	token, err := h.credStore.Get(r.Context(), "github_token")
	if errors.Is(err, driven.ErrEncryptionKeyNotSet) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Credential storage requires MYGITPANEL_SECRET_KEY to be set.</p>`)
		return ""
	}
	if err != nil || token == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Configure a GitHub token in Settings to %s.</p>`, safeAction)
		return ""
	}
	if h.writerFactory == nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">GitHub write operations are not configured.</p>`)
		return ""
	}
	return token
}

// validateJiraBaseURL parses and validates a Jira base URL.
// It requires HTTPS scheme, a non-empty hostname, and rejects hosts that
// resolve to loopback, unspecified, link-local, multicast, or private addresses
// to prevent SSRF attacks.
func validateJiraBaseURL(ctx context.Context, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("base URL must use HTTPS scheme (got %q)", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("base URL must include a hostname")
	}
	// Reject literal IP addresses that are in restricted ranges before DNS lookup.
	if ip := net.ParseIP(host); ip != nil {
		return validateIP(ip)
	}
	// Resolve hostname and reject any address in a restricted range.
	addrs, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		return fmt.Errorf("cannot resolve base URL hostname %q: %w", host, err)
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if err := validateIP(ip); err != nil {
			return fmt.Errorf("base URL hostname %q resolves to disallowed address: %w", host, err)
		}
	}
	return nil
}

// validateIP returns an error if ip is loopback, unspecified, link-local,
// multicast, or a private (RFC1918 / RFC4193) address.
func validateIP(ip net.IP) error {
	switch {
	case ip.IsLoopback():
		return fmt.Errorf("loopback address not allowed")
	case ip.IsUnspecified():
		return fmt.Errorf("unspecified address not allowed")
	case ip.IsLinkLocalUnicast(), ip.IsLinkLocalMulticast():
		return fmt.Errorf("link-local address not allowed")
	case ip.IsMulticast():
		return fmt.Errorf("multicast address not allowed")
	case ip.IsPrivate():
		return fmt.Errorf("private address not allowed")
	}
	return nil
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
func (h *Handler) toRepoViewModels(ctx context.Context, repos []model.Repository) []vm.RepoViewModel {
	vms := make([]vm.RepoViewModel, 0, len(repos))
	for _, r := range repos {
		var assignedID int64
		if h.jiraConnStore != nil {
			conn, err := h.jiraConnStore.GetForRepo(ctx, r.FullName)
			if err != nil {
				h.logger.Warn("failed to get jira mapping for repo", "repo", r.FullName, "error", err)
			} else {
				assignedID = conn.ID
			}
		}
		vms = append(vms, vm.RepoViewModel{
			FullName:                 r.FullName,
			Owner:                    r.Owner,
			Name:                     r.Name,
			DeletePath:               fmt.Sprintf("/app/repos/%s/%s", r.Owner, r.Name),
			AssignedJiraConnectionID: assignedID,
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
		http.Error(w, errMsgCSRFInvalid, http.StatusForbidden)
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
		h.logger.Error("github token validation failed", "error", err)
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: %s</span>`, html.EscapeString(err.Error()))
		return
	}

	// Store the validated token.
	if err := h.credStore.Set(r.Context(), "github_token", token); err != nil {
		if errors.Is(err, driven.ErrEncryptionKeyNotSet) {
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

	fmt.Fprintf(w, `<span class="text-green-600 text-sm">GitHub token: configured (%s)</span>`, html.EscapeString(validatedUsername))
}

// SaveJiraCredentials is a deprecated stub that returns 410 Gone.
// The single-connection endpoint has been replaced by multi-connection handlers in Phase 9.
func (h *Handler) SaveJiraCredentials(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "replaced by /app/settings/jira/connections in Phase 9", http.StatusGone)
}

// CreateJiraConnection handles POST /app/settings/jira/connections.
// It validates the connection via Ping before persisting and returns the updated connection list HTML fragment.
func (h *Handler) CreateJiraConnection(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: invalid form data</span>`)
		return
	}

	if !validateCSRF(r) {
		http.Error(w, errMsgCSRFInvalid, http.StatusForbidden)
		return
	}

	displayName := strings.TrimSpace(r.FormValue("display_name"))
	baseURL := strings.TrimSpace(r.FormValue("base_url"))
	email := strings.TrimSpace(r.FormValue("email"))
	token := strings.TrimSpace(r.FormValue("token"))

	if displayName == "" || baseURL == "" || email == "" || token == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">All fields are required</span>`)
		return
	}

	if err := validateJiraBaseURL(r.Context(), baseURL); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Invalid base URL: %s</span>`, html.EscapeString(err.Error()))
		return
	}

	if h.jiraClientFactory == nil || h.jiraConnStore == nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Jira integration not configured</span>`)
		return
	}

	conn := model.JiraConnection{
		DisplayName: displayName,
		BaseURL:     baseURL,
		Email:       email,
		Token:       token,
	}

	// Validate credentials by pinging the Jira instance.
	client := h.jiraClientFactory(conn)
	if err := client.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		switch {
		case errors.Is(err, driven.ErrJiraUnauthorized):
			fmt.Fprintf(w, `<span class="text-red-600 text-sm">Invalid credentials — check email and API token</span>`)
		case errors.Is(err, driven.ErrJiraUnavailable):
			fmt.Fprintf(w, `<span class="text-red-600 text-sm">Could not reach Jira instance — check base URL</span>`)
		default:
			h.logger.Error("jira ping failed", "error", err)
			fmt.Fprintf(w, `<span class="text-red-600 text-sm">Connection validation failed: %s</span>`, html.EscapeString(err.Error()))
		}
		return
	}

	if _, err := h.jiraConnStore.Create(r.Context(), conn); err != nil {
		h.logger.Error("failed to create jira connection", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: failed to save connection</span>`)
		return
	}

	h.renderJiraConnectionList(w, r)
}

// DeleteJiraConnection handles DELETE /app/settings/jira/connections/{id}.
// It removes the connection and returns the updated connection list HTML fragment.
func (h *Handler) DeleteJiraConnection(w http.ResponseWriter, r *http.Request) {
	if h.jiraConnStore == nil {
		http.Error(w, "Jira integration not configured", http.StatusUnprocessableEntity)
		return
	}
	h.jiraConnectionByID(w, r, "delete jira connection", h.jiraConnStore.Delete)
}

// SetDefaultJiraConnection handles POST /app/settings/jira/connections/{id}/default.
// It marks the connection as the default and returns the updated connection list HTML fragment.
func (h *Handler) SetDefaultJiraConnection(w http.ResponseWriter, r *http.Request) {
	if h.jiraConnStore == nil {
		http.Error(w, "Jira integration not configured", http.StatusUnprocessableEntity)
		return
	}
	h.jiraConnectionByID(w, r, "set default jira connection", h.jiraConnStore.SetDefault)
}

// jiraConnectionByID is a shared handler for operations that parse a connection ID path param,
// validate CSRF, execute a store operation, and render the updated connection list.
func (h *Handler) jiraConnectionByID(w http.ResponseWriter, r *http.Request, opName string, storeFn func(ctx context.Context, id int64) error) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid connection ID", http.StatusBadRequest)
		return
	}

	if !validateCSRF(r) {
		http.Error(w, errMsgCSRFInvalid, http.StatusForbidden)
		return
	}

	if err := storeFn(r.Context(), id); err != nil {
		h.logger.Error("failed to "+opName, "error", err, "id", id)
		http.Error(w, "failed to "+opName, http.StatusInternalServerError)
		return
	}

	h.renderJiraConnectionList(w, r)
}

// SaveJiraRepoMapping handles POST /app/settings/jira/repo-mapping.
// It assigns a Jira connection to a repo or clears the mapping when connectionID is 0.
func (h *Handler) SaveJiraRepoMapping(w http.ResponseWriter, r *http.Request) {
	if h.jiraConnStore == nil {
		http.Error(w, "Jira integration not configured", http.StatusUnprocessableEntity)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, errMsgInvalidFormData, http.StatusBadRequest)
		return
	}

	if !validateCSRF(r) {
		http.Error(w, errMsgCSRFInvalid, http.StatusForbidden)
		return
	}

	repoFullName := strings.TrimSpace(r.FormValue("repo_full_name"))
	connIDStr := r.FormValue("jira_connection_id")

	if repoFullName == "" {
		http.Error(w, "repo_full_name is required", http.StatusBadRequest)
		return
	}

	var connectionID int64
	if connIDStr != "" {
		var err error
		connectionID, err = strconv.ParseInt(connIDStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid jira_connection_id", http.StatusBadRequest)
			return
		}
	}

	if err := h.jiraConnStore.SetRepoMapping(r.Context(), repoFullName, connectionID); err != nil {
		h.logger.Error("failed to set jira repo mapping", "error", err, "repo", repoFullName)
		http.Error(w, "failed to save mapping", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, `<span class="text-green-600 text-xs">Saved</span>`)
}

// renderJiraConnectionList fetches all Jira connections and renders the connection list fragment.
func (h *Handler) renderJiraConnectionList(w http.ResponseWriter, r *http.Request) {
	conns, err := h.jiraConnStore.List(r.Context())
	if err != nil {
		h.logger.Error("failed to list jira connections", "error", err)
		http.Error(w, "failed to load connections", http.StatusInternalServerError)
		return
	}

	connVMs := h.toJiraConnectionViewModels(conns)
	if err := components.JiraConnectionList(connVMs).Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render jira connection list", "error", err)
	}
}

// toJiraConnectionViewModels maps domain JiraConnections to view models.
func (h *Handler) toJiraConnectionViewModels(conns []model.JiraConnection) []vm.JiraConnectionViewModel {
	vms := make([]vm.JiraConnectionViewModel, 0, len(conns))
	for _, c := range conns {
		vms = append(vms, vm.JiraConnectionViewModel{
			ID:          c.ID,
			DisplayName: c.DisplayName,
			BaseURL:     c.BaseURL,
			Email:       c.Email,
			IsDefault:   c.IsDefault,
		})
	}
	return vms
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
		http.Error(w, errMsgInvalidPRNumber, http.StatusBadRequest)
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
		http.Error(w, errMsgCSRFInvalid, http.StatusForbidden)
		return
	}

	body := strings.TrimSpace(r.FormValue("body"))

	if body == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: reply body cannot be empty</p>`)
		return
	}

	token := h.requireGitHubToken(w, r, "reply to comments")
	if token == "" {
		return
	}

	repoFullName := owner + "/" + repo
	writer := h.writerFactory(token)

	if err := writer.CreateReplyComment(r.Context(), repoFullName, number, rootID, body); err != nil {
		h.logger.Error("failed to create reply comment", "repo", repoFullName, "pr", number, "error", err)
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: %s</p>`, html.EscapeString(err.Error()))
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

// parsePRWriteRequest extracts owner, repo, and PR number from the path, parses the
// form body, and validates the CSRF token. It writes error responses and returns false
// if any step fails — callers must return immediately when ok is false.
func (h *Handler) parsePRWriteRequest(w http.ResponseWriter, r *http.Request) (owner, repo string, number int, ok bool) {
	owner = r.PathValue("owner")
	repo = r.PathValue("repo")
	numberStr := r.PathValue("number")

	number, err := strconv.Atoi(numberStr)
	if err != nil {
		http.Error(w, errMsgInvalidPRNumber, http.StatusBadRequest)
		return "", "", 0, false
	}

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: invalid form data</p>`)
		return "", "", 0, false
	}

	if !validateCSRF(r) {
		http.Error(w, errMsgCSRFInvalid, http.StatusForbidden)
		return "", "", 0, false
	}

	return owner, repo, number, true
}

// SubmitReview handles POST /app/prs/{owner}/{repo}/{number}/review.
// It submits a pull request review and re-renders the full reviews section.
func (h *Handler) SubmitReview(w http.ResponseWriter, r *http.Request) {
	owner, repo, number, ok := h.parsePRWriteRequest(w, r)
	if !ok {
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

	token := h.requireGitHubToken(w, r, "submit reviews")
	if token == "" {
		return
	}

	repoFullName := owner + "/" + repo

	// Resolve the current HEAD SHA from the store to avoid GitHub 422s caused by
	// a stale commit_sha baked into the form when the PR received new commits.
	if pr, fetchErr := h.prStore.GetByNumber(r.Context(), repoFullName, number); fetchErr == nil && pr != nil {
		commitSHA = pr.HeadSHA
	}

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
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: %s</p>`, html.EscapeString(err.Error()))
		return
	}

	// Re-fetch and re-render the full reviews section for morph swap.
	h.renderReviewsSectionForPR(w, r, repoFullName, number, owner, repo)
}

// CreateIssueComment handles POST /app/prs/{owner}/{repo}/{number}/issue-comments.
// It creates a general PR comment and re-renders the full reviews section.
func (h *Handler) CreateIssueComment(w http.ResponseWriter, r *http.Request) {
	owner, repo, number, ok := h.parsePRWriteRequest(w, r)
	if !ok {
		return
	}

	body := strings.TrimSpace(r.FormValue("body"))
	if body == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: comment body cannot be empty</p>`)
		return
	}

	token := h.requireGitHubToken(w, r, "post comments")
	if token == "" {
		return
	}

	repoFullName := owner + "/" + repo
	writer := h.writerFactory(token)

	if err := writer.CreateIssueComment(r.Context(), repoFullName, number, body); err != nil {
		h.logger.Error("failed to create issue comment", "repo", repoFullName, "pr", number, "error", err)
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: %s</p>`, html.EscapeString(err.Error()))
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
		http.Error(w, errMsgInvalidPRNumber, http.StatusBadRequest)
		return
	}

	if !validateCSRF(r) {
		http.Error(w, errMsgCSRFInvalid, http.StatusForbidden)
		return
	}

	token := h.requireGitHubToken(w, r, "toggle draft status")
	if token == "" {
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
	if authUser == "" || !strings.EqualFold(pr.Author, authUser) {
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
		fmt.Fprintf(w, `<p class="text-red-600 text-sm">Error: %s</p>`, html.EscapeString(err.Error()))
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
