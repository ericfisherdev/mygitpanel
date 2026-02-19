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
	reviewStore     driven.ReviewStore
	reviewSvc       *application.ReviewService
	healthSvc       *application.HealthService
	pollSvc         *application.PollService
	provider        *application.GitHubClientProvider
	logger          *slog.Logger
}

// NewHandler creates a Handler with all required dependencies.
func NewHandler(
	prStore driven.PRStore,
	repoStore driven.RepoStore,
	credentialStore driven.CredentialStore,
	repoSettings driven.RepoSettingsStore,
	ignoreStore driven.IgnoreStore,
	reviewStore driven.ReviewStore,
	reviewSvc *application.ReviewService,
	healthSvc *application.HealthService,
	pollSvc *application.PollService,
	provider *application.GitHubClientProvider,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		prStore:         prStore,
		repoStore:       repoStore,
		credentialStore: credentialStore,
		repoSettings:    repoSettings,
		ignoreStore:     ignoreStore,
		reviewStore:     reviewStore,
		reviewSvc:       reviewSvc,
		healthSvc:       healthSvc,
		pollSvc:         pollSvc,
		provider:        provider,
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

	// Filter out ignored PRs and count them.
	prs, ignoredCount := h.filterIgnoredPRs(r.Context(), prs)

	// Ensure CSRF cookie is set for mutating requests.
	csrfToken(w, r)

	credStatus := h.loadCredentialStatus(r.Context())

	cards := h.enrichPRCardsWithAttentionSignals(r.Context(), prs)
	data := vm.DashboardViewModel{
		Cards:            cards,
		Repos:            toRepoViewModels(repos),
		RepoNames:        extractRepoNames(repos),
		CredentialStatus: credStatus,
		IgnoredCount:     ignoredCount,
	}
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

	// Filter out ignored PRs.
	prs, _ = h.filterIgnoredPRs(r.Context(), prs)

	filtered := filterPRs(prs, query, status, repo)
	cards := h.enrichPRCardsWithAttentionSignals(r.Context(), filtered)
	component := partials.PRList(cards)

	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render search results", "error", err)
	}
}

// GetPRDetail renders the PR detail partial for HTMX swap into the main panel.
// Enrichment failures (review, health) are non-fatal: basic PR data is always shown.
func (h *Handler) GetPRDetail(w http.ResponseWriter, r *http.Request) {
	repoFullName, number, ok := parsePRParams(w, r)
	if !ok {
		return
	}

	detail, err := h.loadPRDetailViewModel(r.Context(), repoFullName, number)
	if err != nil {
		h.logger.Error("failed to get PR", "repo", repoFullName, "number", number, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if detail == nil {
		http.Error(w, "pull request not found", http.StatusNotFound)
		return
	}

	component := partials.PRDetailContent(*detail)

	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render PR detail", "error", err)
	}
}

// AddRepo adds a repo to the watch list via the GUI form and returns updated partials.
func (h *Handler) AddRepo(w http.ResponseWriter, r *http.Request) {
	if !parseFormWithCSRF(w, r) {
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

	_, _, fullName := parseRepoParams(r)

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

	// Filter out ignored PRs for the sidebar.
	prs, _ = h.filterIgnoredPRs(r.Context(), prs)

	repoVMs := toRepoViewModels(repos)
	cards := h.enrichPRCardsWithAttentionSignals(r.Context(), prs)
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
	if username == "" && h.provider != nil {
		username = h.provider.Username()
	}

	return buildCredentialStatusViewModel(token, username)
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

// parsePRParams extracts owner/repo/number path parameters and validates the PR number.
// Returns repoFullName, number, and true on success. On failure, writes a 400 response
// and returns false — the caller should return immediately.
func parsePRParams(w http.ResponseWriter, r *http.Request) (string, int, bool) {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	numberStr := r.PathValue("number")
	repoFullName := owner + "/" + repo

	number, err := strconv.Atoi(numberStr)
	if err != nil {
		http.Error(w, "invalid PR number", http.StatusBadRequest)
		return "", 0, false
	}

	return repoFullName, number, true
}

// parseRepoParams extracts owner/repo path parameters and builds the full name.
func parseRepoParams(r *http.Request) (owner, repo, repoFullName string) {
	owner = r.PathValue("owner")
	repo = r.PathValue("repo")
	repoFullName = owner + "/" + repo
	return owner, repo, repoFullName
}

// parseFormWithCSRF parses the request form and validates the CSRF token.
// Returns true on success. On failure, writes an appropriate error response
// and returns false — the caller should return immediately.
func parseFormWithCSRF(w http.ResponseWriter, r *http.Request) bool {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return false
	}
	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return false
	}
	return true
}

// loadPRDetailViewModel fetches a PR and enriches it with review summary and health/CI data.
// Returns nil view model (without error) when the PR is not found — the caller decides
// whether that's a 404 or 500.
func (h *Handler) loadPRDetailViewModel(ctx context.Context, repoFullName string, number int) (*vm.PRDetailViewModel, error) {
	pr, err := h.prStore.GetByNumber(ctx, repoFullName, number)
	if err != nil {
		return nil, fmt.Errorf("get PR: %w", err)
	}
	if pr == nil {
		return nil, nil
	}

	var summary *application.PRReviewSummary
	var botUsernames []string

	if h.reviewSvc != nil {
		summary, err = h.reviewSvc.GetPRReviewSummary(ctx, pr.ID, pr.HeadSHA)
		if err != nil {
			h.logger.Error("failed to get review summary", "error", err)
		}
		if summary != nil {
			botUsernames = summary.BotUsernames
		}
	}

	var checkRuns []model.CheckRun

	if h.healthSvc != nil {
		healthSummary, healthErr := h.healthSvc.GetPRHealthSummary(ctx, pr.ID, pr.RepoFullName, pr.Number)
		if healthErr != nil {
			h.logger.Error("failed to get health summary", "error", healthErr)
		}
		if healthSummary != nil {
			checkRuns = healthSummary.CheckRuns
		}
	}

	hasCredentials := h.provider != nil && h.provider.HasClient()
	currentUsername := ""
	if h.provider != nil {
		currentUsername = h.provider.Username()
	}
	detail := toPRDetailViewModelWithWriteCaps(*pr, summary, checkRuns, botUsernames, currentUsername, hasCredentials)
	return &detail, nil
}

// GetCredentialForm renders the credential form partial for HTMX swap.
// Loads current credentials to pre-fill form fields (token masked to last 4 chars).
func (h *Handler) GetCredentialForm(w http.ResponseWriter, r *http.Request) {
	if h.credentialStore == nil {
		h.logger.Error("credentialStore is nil in GetCredentialForm")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	creds, err := h.credentialStore.GetAll(r.Context(), "github")
	if err != nil {
		h.logger.Error("failed to get credentials", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	token := creds["token"]
	username := creds["username"]

	// If no stored creds, fall back to provider username for display.
	if username == "" && h.provider != nil {
		username = h.provider.Username()
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
	if !parseFormWithCSRF(w, r) {
		return
	}

	token := strings.TrimSpace(r.FormValue("token"))
	username := strings.TrimSpace(r.FormValue("username"))

	if token == "" || username == "" {
		http.Error(w, "token and username are required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	if h.credentialStore == nil {
		h.logger.Error("credentialStore is nil in SaveCredentials")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.credentialStore.Set(ctx, "github", "token", token); err != nil {
		h.logger.Error("failed to save token", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.credentialStore.Set(ctx, "github", "username", username); err != nil {
		h.logger.Error("failed to save username", "error", err)
		// Compensating rollback: remove the token we just stored so credentials
		// don't end up in a partially-written state (token present, username absent).
		if deleteErr := h.credentialStore.Delete(ctx, "github", "token"); deleteErr != nil {
			h.logger.Error("failed to roll back token after username save failure", "error", deleteErr)
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if h.provider == nil {
		h.logger.Error("provider is nil in SaveCredentials")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Hot-swap the GitHub client and username.
	newClient := githubadapter.NewClient(token, username)
	h.provider.Replace(newClient, username)

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
// If no provider or client is configured, it writes a 403 response and returns nil.
func (h *Handler) requireGitHubClient(w http.ResponseWriter) driven.GitHubClient {
	if h.provider == nil || !h.provider.HasClient() {
		http.Error(w, "GitHub credentials not configured", http.StatusForbidden)
		return nil
	}
	return h.provider.Get()
}

// SubmitReview submits a review (approve, request changes, or comment) on a PR.
func (h *Handler) SubmitReview(w http.ResponseWriter, r *http.Request) {
	if !parseFormWithCSRF(w, r) {
		return
	}

	client := h.requireGitHubClient(w)
	if client == nil {
		return
	}

	repoFullName, number, ok := parsePRParams(w, r)
	if !ok {
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
	if !parseFormWithCSRF(w, r) {
		return
	}

	client := h.requireGitHubClient(w)
	if client == nil {
		return
	}

	repoFullName, number, ok := parsePRParams(w, r)
	if !ok {
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
	if !parseFormWithCSRF(w, r) {
		return
	}

	client := h.requireGitHubClient(w)
	if client == nil {
		return
	}

	repoFullName, number, ok := parsePRParams(w, r)
	if !ok {
		return
	}

	commentIDStr := r.PathValue("id")

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
	if !parseFormWithCSRF(w, r) {
		return
	}

	client := h.requireGitHubClient(w)
	if client == nil {
		return
	}

	repoFullName, number, ok := parsePRParams(w, r)
	if !ok {
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

	// Refresh stored state synchronously so the re-render reflects the new draft status.
	if h.pollSvc != nil {
		if refreshErr := h.pollSvc.RefreshPR(r.Context(), repoFullName, number); refreshErr != nil {
			h.logger.Error("PR refresh after draft toggle failed", "repo", repoFullName, "pr", number, "error", refreshErr)
		}
	}

	h.logger.Info("draft status toggled", "repo", repoFullName, "pr", number, "is_draft", newDraftState)
	h.renderPRDetailRefresh(w, r, repoFullName, number)
}

// GetRepoSettings renders the per-repo settings form partial.
func (h *Handler) GetRepoSettings(w http.ResponseWriter, r *http.Request) {
	owner, repo, repoFullName := parseRepoParams(r)

	settings, err := h.repoSettings.GetSettings(r.Context(), repoFullName)
	if err != nil {
		h.logger.Error("failed to get repo settings", "repo", repoFullName, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	settingsVM := buildRepoSettingsViewModel(repoFullName, owner, repo, settings, false)
	component := components.RepoSettings(settingsVM)

	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render repo settings", "error", err)
	}
}

// SaveRepoSettings processes the repo settings form submission.
func (h *Handler) SaveRepoSettings(w http.ResponseWriter, r *http.Request) {
	if !parseFormWithCSRF(w, r) {
		return
	}

	owner, repo, repoFullName := parseRepoParams(r)

	requiredReviewCount, err := strconv.Atoi(r.FormValue("required_review_count"))
	if err != nil || requiredReviewCount < 0 {
		http.Error(w, "invalid required review count", http.StatusBadRequest)
		return
	}

	urgencyDays, err := strconv.Atoi(r.FormValue("urgency_days"))
	if err != nil || urgencyDays < 0 {
		http.Error(w, "invalid urgency days", http.StatusBadRequest)
		return
	}

	settings := model.RepoSettings{
		RepoFullName:        repoFullName,
		RequiredReviewCount: requiredReviewCount,
		UrgencyDays:         urgencyDays,
	}

	if err := h.repoSettings.SetSettings(r.Context(), settings); err != nil {
		h.logger.Error("failed to save repo settings", "repo", repoFullName, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("repo settings saved", "repo", repoFullName, "required_reviews", requiredReviewCount, "urgency_days", urgencyDays)

	settingsVM := buildRepoSettingsViewModel(repoFullName, owner, repo, &settings, true)
	component := components.RepoSettings(settingsVM)

	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render repo settings after save", "error", err)
	}
}

// defaultRequiredReviewCount is the default number of approvals required when no settings exist.
const defaultRequiredReviewCount = 2

// defaultUrgencyDays is the default number of inactive days before a PR is flagged as stale.
const defaultUrgencyDays = 7

// buildRepoSettingsViewModel creates a RepoSettingsViewModel, applying defaults when settings is nil.
func buildRepoSettingsViewModel(repoFullName, owner, repo string, settings *model.RepoSettings, saved bool) vm.RepoSettingsViewModel {
	requiredReviewCount := defaultRequiredReviewCount
	urgencyDays := defaultUrgencyDays

	if settings != nil {
		requiredReviewCount = settings.RequiredReviewCount
		urgencyDays = settings.UrgencyDays
	}

	return vm.RepoSettingsViewModel{
		RepoFullName:        repoFullName,
		Owner:               owner,
		Repo:                repo,
		RequiredReviewCount: requiredReviewCount,
		UrgencyDays:         urgencyDays,
		Saved:               saved,
	}
}

// filterIgnoredPRs removes ignored PRs from the slice and returns the filtered
// list plus the total ignored count.
func (h *Handler) filterIgnoredPRs(ctx context.Context, prs []model.PullRequest) ([]model.PullRequest, int) {
	if h.ignoreStore == nil {
		return prs, 0
	}

	ignored, err := h.ignoreStore.ListIgnored(ctx)
	if err != nil {
		h.logger.Error("failed to list ignored PRs", "error", err)
		return prs, 0
	}

	if len(ignored) == 0 {
		return prs, 0
	}

	ignoredSet := make(map[string]struct{}, len(ignored))
	for _, ig := range ignored {
		key := fmt.Sprintf("%s:%d", ig.RepoFullName, ig.PRNumber)
		ignoredSet[key] = struct{}{}
	}

	filtered := make([]model.PullRequest, 0, len(prs))
	for _, pr := range prs {
		key := fmt.Sprintf("%s:%d", pr.RepoFullName, pr.Number)
		if _, ok := ignoredSet[key]; !ok {
			filtered = append(filtered, pr)
		}
	}

	return filtered, len(ignored)
}

// enrichPRCardsWithAttentionSignals converts PRs to card view models and adds
// attention signal flags based on per-repo thresholds.
func (h *Handler) enrichPRCardsWithAttentionSignals(ctx context.Context, prs []model.PullRequest) []vm.PRCardViewModel {
	cards := toPRCardViewModels(prs)

	if h.repoSettings == nil || h.reviewStore == nil {
		return cards
	}

	// Cache settings per repo to avoid N+1 queries.
	settingsCache := make(map[string]*model.RepoSettings)

	for i, pr := range prs {
		settings, ok := settingsCache[pr.RepoFullName]
		if !ok {
			var err error
			settings, err = h.repoSettings.GetSettings(ctx, pr.RepoFullName)
			if err != nil {
				h.logger.Error("failed to get repo settings for attention signals", "repo", pr.RepoFullName, "error", err)
			}
			settingsCache[pr.RepoFullName] = settings
		}

		requiredReviews := defaultRequiredReviewCount
		urgencyDays := defaultUrgencyDays

		if settings != nil {
			requiredReviews = settings.RequiredReviewCount
			urgencyDays = settings.UrgencyDays
		}

		approvalCount, err := h.reviewStore.CountApprovals(ctx, pr.ID)
		if err != nil {
			h.logger.Error("failed to count approvals", "pr_id", pr.ID, "error", err)
		}

		cards[i].NeedsMoreReviews = approvalCount < requiredReviews
		cards[i].IsStale = pr.DaysSinceLastActivity() >= urgencyDays
		cards[i].ApprovalCount = approvalCount
		cards[i].RequiredReviewCount = requiredReviews
		cards[i].DaysInactive = pr.DaysSinceLastActivity()
	}

	return cards
}

// renderPRDetailRefresh re-fetches PR data and renders the PR detail content partial.
// Used after write operations to show updated state.
func (h *Handler) renderPRDetailRefresh(w http.ResponseWriter, r *http.Request, repoFullName string, number int) {
	detail, err := h.loadPRDetailViewModel(r.Context(), repoFullName, number)
	if err != nil {
		h.logger.Error("failed to reload PR after write", "repo", repoFullName, "number", number, "error", err)
		http.Error(w, "failed to reload PR data", http.StatusInternalServerError)
		return
	}
	if detail == nil {
		h.logger.Error("PR not found after write", "repo", repoFullName, "number", number)
		http.Error(w, "failed to reload PR data", http.StatusInternalServerError)
		return
	}

	component := partials.PRDetailContent(*detail)

	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render PR detail after write", "error", err)
	}
}

// IgnorePR adds a PR to the ignore list and re-renders the PR list sidebar.
func (h *Handler) IgnorePR(w http.ResponseWriter, r *http.Request) {
	if !parseFormWithCSRF(w, r) {
		return
	}

	repoFullName, number, ok := parsePRParams(w, r)
	if !ok {
		return
	}

	if err := h.ignoreStore.Ignore(r.Context(), repoFullName, number); err != nil {
		h.logger.Error("failed to ignore PR", "repo", repoFullName, "number", number, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("PR ignored", "repo", repoFullName, "number", number)

	// Re-render the PR list (ignored PR disappears).
	h.renderPRListRefresh(w, r)
}

// UnignorePR removes a PR from the ignore list and re-renders the ignored list.
func (h *Handler) UnignorePR(w http.ResponseWriter, r *http.Request) {
	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	repoFullName, number, ok := parsePRParams(w, r)
	if !ok {
		return
	}

	if err := h.ignoreStore.Unignore(r.Context(), repoFullName, number); err != nil {
		h.logger.Error("failed to unignore PR", "repo", repoFullName, "number", number, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("PR unignored", "repo", repoFullName, "number", number)

	// Re-render the ignored PRs page content.
	h.renderIgnoredPRsContent(w, r)
}

// ListIgnoredPRs renders the ignored PRs page.
func (h *Handler) ListIgnoredPRs(w http.ResponseWriter, r *http.Request) {
	// Ensure CSRF cookie is set.
	csrfToken(w, r)

	ignoredVMs := h.buildIgnoredPRViewModels(r.Context())

	isHTMX := r.Header.Get("HX-Request") == "true"
	component := pages.IgnoredPRs(ignoredVMs)

	if isHTMX {
		if err := component.Render(r.Context(), w); err != nil {
			h.logger.Error("failed to render ignored PRs partial", "error", err)
		}
		return
	}

	layout := templates.Layout("Ignored PRs - ReviewHub", component)
	if err := layout.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render ignored PRs page", "error", err)
	}
}

// buildIgnoredPRViewModels loads ignored PRs and enriches with basic PR data.
func (h *Handler) buildIgnoredPRViewModels(ctx context.Context) []vm.IgnoredPRViewModel {
	ignored, err := h.ignoreStore.ListIgnored(ctx)
	if err != nil {
		h.logger.Error("failed to list ignored PRs", "error", err)
		return nil
	}

	vms := make([]vm.IgnoredPRViewModel, 0, len(ignored))
	for _, ig := range ignored {
		title := fmt.Sprintf("#%d", ig.PRNumber)
		author := ""

		// Enrich with PR data if available.
		pr, err := h.prStore.GetByNumber(ctx, ig.RepoFullName, ig.PRNumber)
		if err != nil {
			h.logger.Error("failed to get PR for ignored list", "repo", ig.RepoFullName, "number", ig.PRNumber, "error", err)
		}
		if pr != nil {
			title = pr.Title
			author = pr.Author
		}

		parts := strings.SplitN(ig.RepoFullName, "/", 2)
		owner := parts[0]
		repo := ""
		if len(parts) == 2 {
			repo = parts[1]
		}

		vms = append(vms, vm.IgnoredPRViewModel{
			RepoFullName: ig.RepoFullName,
			PRNumber:     ig.PRNumber,
			Title:        title,
			Author:       author,
			IgnoredAt:    ig.IgnoredAt.UTC().Format("2006-01-02"),
			RestorePath:  fmt.Sprintf("/app/prs/%s/%s/%d/ignore", owner, repo, ig.PRNumber),
		})
	}

	return vms
}

// renderPRListRefresh re-fetches PRs (excluding ignored) and renders the PR list partial.
func (h *Handler) renderPRListRefresh(w http.ResponseWriter, r *http.Request) {
	prs, err := h.prStore.ListAll(r.Context())
	if err != nil {
		h.logger.Error("failed to list PRs for refresh", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	prs, _ = h.filterIgnoredPRs(r.Context(), prs)
	cards := h.enrichPRCardsWithAttentionSignals(r.Context(), prs)
	component := partials.PRList(cards)

	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render PR list refresh", "error", err)
	}
}

// renderIgnoredPRsContent re-renders the ignored PRs list content after an unignore.
func (h *Handler) renderIgnoredPRsContent(w http.ResponseWriter, r *http.Request) {
	ignoredVMs := h.buildIgnoredPRViewModels(r.Context())
	component := pages.IgnoredPRs(ignoredVMs)

	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render ignored PRs content", "error", err)
	}
}
