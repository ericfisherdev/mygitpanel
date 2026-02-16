// Package web implements the HTML GUI driving adapter using templ components.
package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	githubadapter "github.com/ericfisherdev/mygitpanel/internal/adapter/driven/github"
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
	prStore         driven.PRStore
	repoStore       driven.RepoStore
	credentialStore driven.CredentialStore
	repoSettings    driven.RepoSettingsStore
	ignoreStore     driven.IgnoreStore
	reviewSvc       *application.ReviewService
	healthSvc       *application.HealthService
	pollSvc         *application.PollService
	provider        *application.GitHubClientProvider
	username        string
	logger          *slog.Logger
}

// NewHandler creates a Handler with all required dependencies.
func NewHandler(
	prStore driven.PRStore,
	repoStore driven.RepoStore,
	credentialStore driven.CredentialStore,
	repoSettings driven.RepoSettingsStore,
	ignoreStore driven.IgnoreStore,
	reviewSvc *application.ReviewService,
	healthSvc *application.HealthService,
	pollSvc *application.PollService,
	provider *application.GitHubClientProvider,
	username string,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		prStore:         prStore,
		repoStore:       repoStore,
		credentialStore: credentialStore,
		repoSettings:    repoSettings,
		ignoreStore:     ignoreStore,
		reviewSvc:       reviewSvc,
		healthSvc:       healthSvc,
		pollSvc:         pollSvc,
		provider:        provider,
		username:        username,
		logger:          logger,
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

	repos, err := h.repoStore.ListAll(r.Context())
	if err != nil {
		h.logger.Error("failed to list repos", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Ensure CSRF cookie is set for mutating requests.
	csrfToken(w, r)

	credStatus := h.loadCredentialStatus(r.Context())

	data := buildDashboardViewModel(prs, repos, credStatus)
	component := pages.Dashboard(data)
	layout := templates.Layout("ReviewHub", component)

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
	cards := toPRCardViewModels(filtered)
	component := partials.PRList(cards)

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

	hasCredentials := h.provider != nil && h.provider.HasClient()
	detail := toPRDetailViewModelWithWriteCaps(*pr, summary, checkRuns, botUsernames, h.username, hasCredentials)
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
	cards := toPRCardViewModels(prs)
	repoNames := extractRepoNames(repos)

	// Primary target: repo list.
	repoListComp := partials.RepoList(repoVMs)
	if err := repoListComp.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render repo list", "error", err)
		return
	}

	// OOB swap: PR list.
	prListComp := partials.PRListOOB(cards)
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

// loadCredentialStatus fetches credential state from the store and returns a view model.
func (h *Handler) loadCredentialStatus(ctx context.Context) vm.CredentialStatusViewModel {
	if h.credentialStore == nil {
		return vm.CredentialStatusViewModel{}
	}

	creds, err := h.credentialStore.GetAll(ctx, "github")
	if err != nil {
		h.logger.Error("failed to load credentials for dashboard", "error", err)
		return vm.CredentialStatusViewModel{}
	}

	token := creds["token"]
	username := creds["username"]
	if username == "" {
		username = h.username
	}

	return buildCredentialStatusViewModel(token, username)
}

// buildDashboardViewModel constructs the full view model for the dashboard page.
func buildDashboardViewModel(prs []model.PullRequest, repos []model.Repository, credStatus vm.CredentialStatusViewModel) vm.DashboardViewModel {
	return vm.DashboardViewModel{
		Cards:            toPRCardViewModels(prs),
		Repos:            toRepoViewModels(repos),
		RepoNames:        extractRepoNames(repos),
		CredentialStatus: credStatus,
	}
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

// GetCredentialForm renders the credential form partial for HTMX swap.
// Loads current credentials to pre-fill form fields (token masked to last 4 chars).
func (h *Handler) GetCredentialForm(w http.ResponseWriter, r *http.Request) {
	creds, err := h.credentialStore.GetAll(r.Context(), "github")
	if err != nil {
		h.logger.Error("failed to get credentials", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	token := creds["token"]
	username := creds["username"]

	// If no stored creds, fall back to handler username for display.
	if username == "" {
		username = h.username
	}

	status := buildCredentialStatusViewModel(token, username)

	component := components.CredentialForm(status)
	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render credential form", "error", err)
	}
}

// SaveCredentials processes the credential form submission, saves to SQLite,
// and hot-swaps the GitHub client in the provider.
func (h *Handler) SaveCredentials(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	token := strings.TrimSpace(r.FormValue("token"))
	username := strings.TrimSpace(r.FormValue("username"))

	if token == "" || username == "" {
		http.Error(w, "token and username are required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	if err := h.credentialStore.Set(ctx, "github", "token", token); err != nil {
		h.logger.Error("failed to save token", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.credentialStore.Set(ctx, "github", "username", username); err != nil {
		h.logger.Error("failed to save username", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Hot-swap the GitHub client.
	newClient := githubadapter.NewClient(token, username)
	h.provider.Replace(newClient)

	h.logger.Info("github credentials updated", "username", username)

	status := buildCredentialStatusViewModel(token, username)
	component := components.CredentialStatus(status)
	if err := component.Render(ctx, w); err != nil {
		h.logger.Error("failed to render credential status", "error", err)
	}
}

// buildCredentialStatusViewModel creates a CredentialStatusViewModel from raw credentials.
func buildCredentialStatusViewModel(token, username string) vm.CredentialStatusViewModel {
	configured := token != "" && username != ""
	masked := ""
	if token != "" {
		if len(token) > 4 {
			masked = "****" + token[len(token)-4:]
		} else {
			masked = "****"
		}
	}

	return vm.CredentialStatusViewModel{
		Configured:  configured,
		Username:    username,
		TokenMasked: masked,
	}
}

// requireGitHubClient returns the current GitHub client from the provider.
// If no client is configured, it writes a 403 response and returns nil.
func (h *Handler) requireGitHubClient(w http.ResponseWriter) driven.GitHubClient {
	client := h.provider.Get()
	if client == nil {
		http.Error(w, "GitHub credentials not configured", http.StatusForbidden)
		return nil
	}
	return client
}

// SubmitReview submits a review (approve, request changes, or comment) on a PR.
func (h *Handler) SubmitReview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	client := h.requireGitHubClient(w)
	if client == nil {
		return
	}

	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	numberStr := r.PathValue("number")
	repoFullName := owner + "/" + repo

	number, err := strconv.Atoi(numberStr)
	if err != nil {
		http.Error(w, "invalid PR number", http.StatusBadRequest)
		return
	}

	event := strings.TrimSpace(r.FormValue("event"))
	body := strings.TrimSpace(r.FormValue("body"))

	if event == "" {
		http.Error(w, "review event is required", http.StatusBadRequest)
		return
	}

	if err := client.CreateReview(r.Context(), repoFullName, number, event, body); err != nil {
		h.logger.Error("failed to submit review", "repo", repoFullName, "pr", number, "error", err)
		http.Error(w, "failed to submit review", http.StatusInternalServerError)
		return
	}

	h.logger.Info("review submitted", "repo", repoFullName, "pr", number, "event", event)
	h.renderPRDetailRefresh(w, r, repoFullName, number)
}

// AddComment adds a PR-level comment via the Issues API.
func (h *Handler) AddComment(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	client := h.requireGitHubClient(w)
	if client == nil {
		return
	}

	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	numberStr := r.PathValue("number")
	repoFullName := owner + "/" + repo

	number, err := strconv.Atoi(numberStr)
	if err != nil {
		http.Error(w, "invalid PR number", http.StatusBadRequest)
		return
	}

	body := strings.TrimSpace(r.FormValue("body"))
	if body == "" {
		http.Error(w, "comment body is required", http.StatusBadRequest)
		return
	}

	if err := client.CreateIssueComment(r.Context(), repoFullName, number, body); err != nil {
		h.logger.Error("failed to add comment", "repo", repoFullName, "pr", number, "error", err)
		http.Error(w, "failed to add comment", http.StatusInternalServerError)
		return
	}

	h.logger.Info("comment added", "repo", repoFullName, "pr", number)
	h.renderPRDetailRefresh(w, r, repoFullName, number)
}

// ReplyToComment replies to an existing review comment thread.
func (h *Handler) ReplyToComment(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	client := h.requireGitHubClient(w)
	if client == nil {
		return
	}

	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	numberStr := r.PathValue("number")
	commentIDStr := r.PathValue("id")
	repoFullName := owner + "/" + repo

	number, err := strconv.Atoi(numberStr)
	if err != nil {
		http.Error(w, "invalid PR number", http.StatusBadRequest)
		return
	}

	commentID, err := strconv.ParseInt(commentIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid comment ID", http.StatusBadRequest)
		return
	}

	body := strings.TrimSpace(r.FormValue("body"))
	if body == "" {
		http.Error(w, "reply body is required", http.StatusBadRequest)
		return
	}

	if err := client.ReplyToReviewComment(r.Context(), repoFullName, number, commentID, body); err != nil {
		h.logger.Error("failed to reply to comment", "repo", repoFullName, "pr", number, "comment", commentID, "error", err)
		http.Error(w, "failed to reply to comment", http.StatusInternalServerError)
		return
	}

	h.logger.Info("replied to comment", "repo", repoFullName, "pr", number, "comment", commentID)
	h.renderPRDetailRefresh(w, r, repoFullName, number)
}

// ToggleDraft toggles a PR's draft status via GraphQL mutation.
func (h *Handler) ToggleDraft(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	client := h.requireGitHubClient(w)
	if client == nil {
		return
	}

	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	numberStr := r.PathValue("number")
	repoFullName := owner + "/" + repo

	number, err := strconv.Atoi(numberStr)
	if err != nil {
		http.Error(w, "invalid PR number", http.StatusBadRequest)
		return
	}

	pr, err := h.prStore.GetByNumber(r.Context(), repoFullName, number)
	if err != nil {
		h.logger.Error("failed to get PR for draft toggle", "repo", repoFullName, "number", number, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if pr == nil {
		http.Error(w, "pull request not found", http.StatusNotFound)
		return
	}

	if pr.NodeID == "" {
		http.Error(w, "Node ID not available, try refreshing PR data", http.StatusBadRequest)
		return
	}

	newDraftState := !pr.IsDraft
	if err := client.SetDraftStatus(r.Context(), repoFullName, number, pr.NodeID, newDraftState); err != nil {
		h.logger.Error("failed to toggle draft status", "repo", repoFullName, "pr", number, "error", err)
		http.Error(w, "failed to toggle draft status", http.StatusInternalServerError)
		return
	}

	// Trigger a refresh to update stored state.
	if h.pollSvc != nil {
		go func() { //nolint:contextcheck // intentional background context for fire-and-forget
			if refreshErr := h.pollSvc.RefreshPR(context.Background(), repoFullName, number); refreshErr != nil {
				h.logger.Error("async PR refresh after draft toggle failed", "repo", repoFullName, "pr", number, "error", refreshErr)
			}
		}()
	}

	h.logger.Info("draft status toggled", "repo", repoFullName, "pr", number, "is_draft", newDraftState)
	h.renderPRDetailRefresh(w, r, repoFullName, number)
}

// renderPRDetailRefresh re-fetches PR data and renders the PR detail content partial.
// Used after write operations to show updated state.
func (h *Handler) renderPRDetailRefresh(w http.ResponseWriter, r *http.Request, repoFullName string, number int) {
	pr, err := h.prStore.GetByNumber(r.Context(), repoFullName, number)
	if err != nil || pr == nil {
		h.logger.Error("failed to reload PR after write", "repo", repoFullName, "number", number, "error", err)
		http.Error(w, "failed to reload PR data", http.StatusInternalServerError)
		return
	}

	// Enrich with review data (non-fatal).
	var summary *application.PRReviewSummary
	var botUsernames []string

	if h.reviewSvc != nil {
		summary, err = h.reviewSvc.GetPRReviewSummary(r.Context(), pr.ID, pr.HeadSHA)
		if err != nil {
			h.logger.Error("failed to get review summary after write", "error", err)
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
			h.logger.Error("failed to get health summary after write", "error", healthErr)
		}
		if healthSummary != nil {
			checkRuns = healthSummary.CheckRuns
		}
	}

	hasCredentials := h.provider != nil && h.provider.HasClient()
	detail := toPRDetailViewModelWithWriteCaps(*pr, summary, checkRuns, botUsernames, h.username, hasCredentials)
	component := partials.PRDetailContent(detail)

	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render PR detail after write", "error", err)
	}
}
