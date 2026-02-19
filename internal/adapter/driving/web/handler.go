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
	username       string
	logger         *slog.Logger
	credStore      driven.CredentialStore
	thresholdStore driven.ThresholdStore
	ignoreStore    driven.IgnoreStore
	ghWriter       driven.GitHubWriter
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
	ghWriter driven.GitHubWriter,
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
		ghWriter:       ghWriter,
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

	data := buildDashboardViewModel(prs, repos)
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

	detail := toPRDetailViewModel(*pr, summary, checkRuns, botUsernames)
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

// buildDashboardViewModel constructs the full view model for the dashboard page.
func buildDashboardViewModel(prs []model.PullRequest, repos []model.Repository) vm.DashboardViewModel {
	return vm.DashboardViewModel{
		Cards:     toPRCardViewModels(prs),
		Repos:     toRepoViewModels(repos),
		RepoNames: extractRepoNames(repos),
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

// SaveGitHubCredentials handles POST /app/settings/github.
// It validates the provided token against the GitHub API before storing it.
// The response is an HTML fragment injected into #cred-github-status by HTMX.
func (h *Handler) SaveGitHubCredentials(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: invalid form data</span>`)
		return
	}

	token := strings.TrimSpace(r.FormValue("github_token"))
	username := strings.TrimSpace(r.FormValue("github_username"))

	if token == "" {
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: GitHub token is required</span>`)
		return
	}

	if h.ghWriter == nil || h.credStore == nil {
		fmt.Fprintf(w, `<span class="text-red-600 text-sm">Error: credential storage is not configured</span>`)
		return
	}

	// Validate the token against the GitHub API.
	validatedUsername, err := h.ghWriter.ValidateToken(r.Context(), token)
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

	// Store the username (provided or from validation).
	storedUsername := validatedUsername
	if username != "" {
		storedUsername = username
	}
	if err := h.credStore.Set(r.Context(), "github_username", storedUsername); err != nil {
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
