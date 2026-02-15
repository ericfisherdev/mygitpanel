# Architecture Patterns: v2.0 Web GUI Integration

**Domain:** Web GUI with templ/HTMX/Alpine.js, Jira integration, GitHub review submission
**Researched:** 2026-02-14
**Overall confidence:** HIGH (existing hexagonal architecture is well-suited; new components follow established patterns)

---

## Current Architecture (v1.0 Baseline)

```
cmd/mygitpanel/main.go                    <-- Composition root
internal/
  domain/model/                            <-- Pure entities (no external deps)
  domain/port/driven/                      <-- Secondary ports (GitHubClient, PRStore, RepoStore, etc.)
  application/                             <-- Use cases (PollService, ReviewService, HealthService)
  adapter/driven/github/                   <-- GitHub REST+GraphQL adapter (go-github v82)
  adapter/driven/sqlite/                   <-- SQLite adapter (modernc.org/sqlite, WAL mode)
  adapter/driving/http/                    <-- HTTP REST adapter (stdlib net/http, Go 1.22+ routing)
  config/                                  <-- Env var loading, fail-fast validation
```

The existing architecture is hexagonal with strict dependency flow inward. The HTTP handler (`adapter/driving/http/`) is thin: it parses requests, calls port interfaces/application services, and writes JSON responses. This is the perfect seam for adding HTML rendering alongside JSON.

---

## Recommended Architecture (v2.0)

### High-Level Diagram

```
                    +-----------------------------------------+
                    |          Web GUI (templ + HTMX)          |  <-- NEW Driving Adapter
                    |  internal/adapter/driving/web/           |
                    +-----------+-----------------------------+
                                |
                    +-----------v-----------------------------+
                    |          REST API (JSON)                 |  <-- EXISTING Driving Adapter
                    |  internal/adapter/driving/http/          |  (unchanged)
                    +-----------+-----------------------------+
                                |
                    +-----------v-----------------------------+
                    |        Application Layer                 |
                    |  PollService | ReviewService |           |
                    |  HealthService | ReviewSubmitService*    |  <-- *NEW service
                    |  JiraService*                            |  <-- *NEW service
                    +---+--------+---------+------+-----------+
                        |        |         |      |
           +------------+  +----+----+  +--+---+  +----------+
           | GitHubClient|  | PRStore |  | Repo |  | JiraClient|  <-- *NEW port
           | (extended*) |  |         |  | Store|  |           |
           +------+------+  +---+----+  +--+---+  +-----+----+
                  |              |          |             |
           +------+------+  +---+----+  +--+---+  +-----+----+
           | github/     |  | sqlite/|  |sqlite/|  | jira/    |  <-- *NEW adapter
           | client.go   |  |        |  |       |  | client.go|
           +-------------+  +--------+  +-------+  +----------+
```

### New Components (Files to Create)

| Component | Location | Purpose |
|-----------|----------|---------|
| **Web driving adapter** | `internal/adapter/driving/web/` | templ handlers that render HTML partials |
| **templ components** | `internal/adapter/driving/web/components/` | Reusable templ UI components |
| **templ layouts** | `internal/adapter/driving/web/layouts/` | Base layout, page shells |
| **templ pages** | `internal/adapter/driving/web/pages/` | Full page compositions |
| **Jira port** | `internal/domain/port/driven/jiraclient.go` | Interface for Jira API operations |
| **Jira adapter** | `internal/adapter/driven/jira/` | Concrete Jira REST API client |
| **Jira domain model** | `internal/domain/model/jira.go` | JiraIssue, JiraComment entities |
| **ReviewSubmitService** | `internal/application/reviewsubmitservice.go` | Orchestrates GitHub review submission |
| **JiraService** | `internal/application/jiraservice.go` | Orchestrates Jira read + comment |
| **Static assets** | `static/` or `web/static/` | CSS (Tailwind output), JS (Alpine, HTMX, GSAP) |
| **Config additions** | `internal/config/config.go` | New env vars for Jira credentials |

### Existing Components to Modify

| Component | File | Change |
|-----------|------|--------|
| **GitHubClient port** | `internal/domain/port/driven/githubclient.go` | Add `SubmitReview`, `CreateReviewComment`, `ReplyToComment` methods |
| **GitHub adapter** | `internal/adapter/driven/github/client.go` | Implement new write methods using `PullRequests.CreateReview` etc. |
| **Composition root** | `cmd/mygitpanel/main.go` | Wire new services, register web routes, serve static assets |
| **Config** | `internal/config/config.go` | Add Jira URL, Jira token, Jira email env vars |
| **HTTP ServeMux** | `internal/adapter/driving/http/handler.go` | Mount web adapter routes alongside `/api/v1/` routes |

**Critical rule:** The existing JSON API (`/api/v1/*`) remains completely untouched. The web GUI is an additional driving adapter that calls the same ports and application services.

---

## Component Boundaries

### 1. Web Driving Adapter (`internal/adapter/driving/web/`)

This is the core new driving adapter. It follows the same pattern as the existing HTTP handler but renders templ components instead of JSON.

```
internal/adapter/driving/web/
  handler.go              <-- Handler struct, constructor, route handlers
  routes.go               <-- Route registration (RegisterRoutes)
  viewmodel.go            <-- Domain-to-view-model mapping
  viewmodel/viewmodel.go  <-- View model struct definitions
  markdown.go             <-- Markdown-to-HTML rendering with bluemonday XSS sanitization
  csrf.go                 <-- CSRF token generation and validation
  embed.go                <-- go:embed for static assets
  templates/
    layout.templ           <-- HTML skeleton (head, body, scripts)
    pages/
      dashboard.templ      <-- Dashboard page composition
    components/
      pr_card.templ        <-- PR card component
      pr_detail.templ      <-- PR detail panel
      sidebar.templ        <-- Collapsible sidebar
      search_bar.templ     <-- Search and filter bar
      theme_toggle.templ   <-- Dark/light theme toggle
      repo_manager.templ   <-- Repo add/remove management
    partials/
      pr_list.templ        <-- PR list (HTMX swap target)
      pr_detail_content.templ  <-- PR detail content (HTMX partial)
      repo_list.templ      <-- Repo list (HTMX partial)
  static/
    css/input.css          <-- Tailwind CSS input (compiled via standalone CLI)
    css/output.css         <-- Generated (gitignored)
    js/animations.js       <-- GSAP animation initializers
    js/csrf.js             <-- CSRF token header injection for HTMX
    js/stores.js           <-- Alpine.js stores (theme persistence)
    vendor/                <-- Vendored JS: htmx, alpine, gsap, extensions
```

**Key design decisions:**

1. **WebHandler depends on the same ports and services as Handler.** No duplication of business logic.
2. **Each handler method decides:** render a full page (initial load) or an HTMX partial (subsequent interactions), based on the `HX-Request` header.
3. **templ components are the "view" layer.** They receive plain Go structs (view models), not domain models directly. This prevents templ files from depending on the domain package.

```go
// internal/adapter/driving/web/handler.go
type WebHandler struct {
    prStore        driven.PRStore
    repoStore      driven.RepoStore
    botConfigStore driven.BotConfigStore
    reviewSvc      *application.ReviewService
    healthSvc      *application.HealthService
    pollSvc        *application.PollService
    reviewSubmitSvc *application.ReviewSubmitService  // NEW
    jiraSvc        *application.JiraService           // NEW
    username       string
    logger         *slog.Logger
}
```

### 2. HTMX Endpoint Pattern

HTMX endpoints return HTML fragments, not full pages. The pattern:

```go
// handler_pr.go
func (h *WebHandler) PRList(w http.ResponseWriter, r *http.Request) {
    prs, err := h.prStore.ListAll(r.Context())
    if err != nil {
        // Render error partial
        components.ErrorBanner("Failed to load PRs").Render(r.Context(), w)
        return
    }

    viewModels := toViewModels(prs) // Convert domain -> view models

    if r.Header.Get("HX-Request") == "true" {
        // HTMX partial: just the PR list fragment
        components.PRList(viewModels).Render(r.Context(), w)
        return
    }

    // Full page load: wrap in layout
    pages.Dashboard(viewModels).Render(r.Context(), w)
}
```

HTMX routes live under a separate prefix to avoid collision with the JSON API:

```
GET  /{$}                                       -> Dashboard (full page)
GET  /app/prs/{owner}/{repo}/{number}            -> PR detail (HTMX partial)
GET  /app/prs/search                             -> Search PRs (HTMX partial)
POST /app/repos                                  -> Add repo (HTMX form)
DELETE /app/repos/{owner}/{repo}                 -> Remove repo (HTMX)
GET  /static/*                                   -> Embedded static assets
```

### 3. Alpine.js Integration (Client-Side State)

Alpine.js handles **local UI state only** -- no data fetching (that is HTMX's job):

- **Theme toggle:** `x-data="{ dark: false }"` with localStorage persistence
- **Modal state:** `x-data="{ showReviewForm: false }"` for review submission dialog
- **Tab state:** `x-data="{ activeTab: 'threads' }"` for PR detail tabs
- **Dropdown menus:** `x-data="{ open: false }"` for filter dropdowns
- **Toast notifications:** `x-data="{ toasts: [] }"` with HTMX `afterSwap` events
- **Client-side filtering:** `x-data="{ filter: '' }"` on already-loaded lists

Alpine.js state does NOT survive HTMX swaps by default. Use the `alpine-morph` HTMX extension when replacing content that contains Alpine state. For most cases, keep Alpine state on **parent elements** that are outside HTMX swap targets.

```html
<!-- Alpine state lives OUTSIDE the HTMX swap target -->
<div x-data="{ activeTab: 'threads' }">
    <nav>
        <button @click="activeTab = 'threads'" :class="activeTab === 'threads' && 'active'">Threads</button>
        <button @click="activeTab = 'checks'" :class="activeTab === 'checks' && 'active'">Checks</button>
    </nav>
    <!-- HTMX swaps happen INSIDE this target -->
    <div id="pr-detail-content"
         hx-get="/prs/owner/repo/123/threads"
         hx-trigger="load">
    </div>
</div>
```

### 4. GitHubClient Port Extension (Review Submission)

Add write methods to the existing `driven.GitHubClient` interface:

```go
// internal/domain/port/driven/githubclient.go -- additions
type GitHubClient interface {
    // ... existing read methods ...

    // SubmitReview creates and submits a review on a pull request.
    // event must be one of: "APPROVE", "REQUEST_CHANGES", "COMMENT".
    SubmitReview(ctx context.Context, repoFullName string, prNumber int, body string, event string) error

    // CreateReviewComment creates a new inline comment on a PR diff.
    CreateReviewComment(ctx context.Context, repoFullName string, prNumber int, comment model.NewReviewComment) error

    // ReplyToComment creates a reply to an existing review comment thread.
    ReplyToComment(ctx context.Context, repoFullName string, prNumber int, commentID int64, body string) error
}
```

The adapter implementation uses `go-github`'s existing methods:

- `PullRequests.CreateReview(ctx, owner, repo, number, &PullRequestReviewRequest{...})` -- for full review submission
- `PullRequests.CreateComment(ctx, owner, repo, number, &PullRequestComment{...})` -- for inline comments
- `PullRequests.CreateCommentInReplyTo(ctx, owner, repo, number, body, replyToID)` -- for thread replies

These methods already exist in go-github v82. No new dependencies needed.

### 5. Jira Port and Adapter

**New driven port:**

```go
// internal/domain/port/driven/jiraclient.go
type JiraClient interface {
    // GetIssue retrieves a Jira issue by key (e.g., "PROJ-123").
    GetIssue(ctx context.Context, issueKey string) (*model.JiraIssue, error)

    // SearchIssuesByPR finds Jira issues linked to a PR (by branch name or PR URL).
    SearchIssuesByPR(ctx context.Context, prURL string, branchName string) ([]model.JiraIssue, error)

    // AddComment adds a comment to a Jira issue.
    AddComment(ctx context.Context, issueKey string, body string) error
}
```

**New domain model:**

```go
// internal/domain/model/jira.go
type JiraIssue struct {
    Key         string     // e.g., "PROJ-123"
    Summary     string
    Status      string     // e.g., "In Progress"
    Assignee    string
    IssueType   string     // e.g., "Story", "Bug"
    Priority    string     // e.g., "High"
    URL         string     // Link to Jira web UI
    Labels      []string
    UpdatedAt   time.Time
}
```

**Adapter recommendation:** Use `ctreminiom/go-atlassian` because:
1. Actively maintained, cloud-first design matching modern Jira deployments
2. Interface-driven architecture (aligns with hexagonal style)
3. Supports Jira v2/v3 REST APIs
4. Built-in OAuth 2.0 support
5. Inspired by go-github's service pattern (familiar to this codebase)

Alternative: `andygrunwald/go-jira` if self-hosted Jira is required (also actively maintained).

### 6. New Application Services

**ReviewSubmitService** -- Orchestrates review submission with validation:

```go
// internal/application/reviewsubmitservice.go
type ReviewSubmitService struct {
    ghClient driven.GitHubClient
    prStore  driven.PRStore
}

func (s *ReviewSubmitService) SubmitReview(ctx context.Context, repoFullName string, prNumber int, body string, event string) error {
    // 1. Validate PR exists in our store
    // 2. Validate event is one of APPROVE/REQUEST_CHANGES/COMMENT
    // 3. Delegate to ghClient.SubmitReview
    // 4. Optionally trigger a refresh to pick up the new review
}

func (s *ReviewSubmitService) ReplyToComment(ctx context.Context, repoFullName string, prNumber int, commentID int64, body string) error {
    // 1. Delegate to ghClient.ReplyToComment
    // 2. Trigger refresh
}
```

**JiraService** -- Orchestrates Jira lookups with caching:

```go
// internal/application/jiraservice.go
type JiraService struct {
    jiraClient driven.JiraClient
}

func (s *JiraService) GetLinkedIssues(ctx context.Context, prURL string, branchName string) ([]model.JiraIssue, error) {
    // Search by PR URL first, fall back to branch name pattern
    // Branch pattern: extract "PROJ-123" from "feat/PROJ-123-description"
}

func (s *JiraService) AddComment(ctx context.Context, issueKey string, body string) error {
    return s.jiraClient.AddComment(ctx, issueKey, body)
}
```

---

## Data Flow

### Flow 1: Dashboard Page Load (Full Page)

```
Browser GET /
  -> WebHandler.Dashboard()
    -> prStore.ListAll()         // existing port
    -> prStore.ListNeedingReview() // existing port
    -> Convert to view models
    -> pages.Dashboard(viewModels).Render(ctx, w)
  <- Full HTML page with head/scripts/nav/content
```

### Flow 2: PR List Refresh (HTMX Partial)

```
HTMX GET /prs (HX-Request: true)
  -> WebHandler.PRList()
    -> prStore.ListAll()
    -> Convert to view models
    -> components.PRList(viewModels).Render(ctx, w)
  <- HTML fragment (just the <div id="pr-list"> content)
  -> HTMX swaps into #pr-list target
```

### Flow 3: Submit Review (HTMX POST)

```
HTMX POST /prs/owner/repo/123/review
  Body: event=APPROVE&body=LGTM
  -> WebHandler.SubmitReview()
    -> reviewSubmitSvc.SubmitReview(ctx, "owner/repo", 123, "LGTM", "APPROVE")
      -> ghClient.SubmitReview(ctx, ...)  // calls GitHub API
      -> pollSvc.RefreshPR(ctx, ...)      // trigger re-poll to pick up new review
    -> components.ReviewSubmitSuccess().Render(ctx, w)
  <- HTML fragment with success toast
  -> HTMX triggers refresh of review threads via HX-Trigger header
```

### Flow 4: Jira Sidebar Load (HTMX Partial)

```
HTMX GET /prs/owner/repo/123/jira (lazy loaded)
  -> WebHandler.JiraSidebar()
    -> Get PR from prStore to get branch name and URL
    -> jiraSvc.GetLinkedIssues(ctx, prURL, branchName)
    -> components.JiraSidebar(issues).Render(ctx, w)
  <- HTML fragment with linked Jira issues
```

---

## Route Architecture

The composition root mounts both adapters on the same `http.ServeMux`:

```go
// cmd/mygitpanel/main.go -- additions
func run() error {
    // ... existing setup ...

    // NEW: Create review submit service
    reviewSubmitSvc := application.NewReviewSubmitService(ghClient, prStore, pollSvc)

    // NEW: Create Jira client and service (optional, nil if not configured)
    var jiraSvc *application.JiraService
    if cfg.JiraURL != "" {
        jiraClient := jiraadapter.NewClient(cfg.JiraURL, cfg.JiraEmail, cfg.JiraToken)
        jiraSvc = application.NewJiraService(jiraClient)
    }

    // Existing JSON API handler (unchanged)
    h := httphandler.NewHandler(prStore, repoStore, botConfigStore, reviewSvc, healthSvc, pollSvc, cfg.GitHubUsername, slog.Default())

    // NEW: Web GUI handler
    wh := webhandler.NewWebHandler(prStore, repoStore, botConfigStore, reviewSvc, healthSvc, pollSvc, reviewSubmitSvc, jiraSvc, cfg.GitHubUsername, slog.Default())

    mux := http.NewServeMux()

    // JSON API routes (existing, unchanged)
    apiMux := httphandler.NewServeMux(h, slog.Default())
    mux.Handle("/api/", apiMux)

    // Web GUI routes (new)
    webMux := webhandler.NewServeMux(wh, slog.Default())
    mux.Handle("/", webMux)

    // Static assets
    mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

    // ... rest unchanged ...
}
```

**Route separation strategy:** `/api/v1/*` routes hit the JSON adapter. All other routes hit the web adapter. No ambiguity.

---

## Patterns to Follow

### Pattern 1: View Models (Decouple templ from Domain)

**What:** Create lightweight structs in the web adapter package that templ components accept. Map domain models to view models in the handler before rendering.

**When:** Always. templ files must never import `domain/model/`.

**Why:** Prevents the view layer from dictating domain model shape. Allows adding display-only fields (formatted dates, color classes, computed labels) without polluting domain models.

```go
// internal/adapter/driving/web/viewmodel.go
type PRViewModel struct {
    Number         int
    Repository     string
    Title          string
    Author         string
    Status         string
    StatusColor    string   // "green", "red", "yellow" -- display concern
    NeedsReview    bool
    URL            string
    Labels         []string
    OpenedAgo      string   // "3 days ago" -- display concern
    UpdatedAgo     string
    ReviewStatus   string
    CIStatusIcon   string   // "check-circle", "x-circle" -- display concern
}
```

### Pattern 2: HTMX Partial vs Full Page (HX-Request Header)

**What:** Every page handler checks for `HX-Request: true` header. If present, render just the content partial. If absent, render the full page with layout.

**When:** On every GET handler that serves a page.

```go
func (h *WebHandler) renderPage(w http.ResponseWriter, r *http.Request, page, partial templ.Component) {
    if r.Header.Get("HX-Request") == "true" {
        partial.Render(r.Context(), w)
        return
    }
    page.Render(r.Context(), w)
}
```

### Pattern 3: HTMX Response Headers for Cascading Updates

**What:** After a mutation (submit review, add repo), set `HX-Trigger` response headers to tell other HTMX elements on the page to refresh.

**When:** On every POST/DELETE handler.

```go
func (h *WebHandler) SubmitReview(w http.ResponseWriter, r *http.Request) {
    // ... submit review ...

    // Tell the page to refresh the review threads and PR status
    w.Header().Set("HX-Trigger", "refreshThreads, refreshPRStatus")
    components.ReviewSubmitSuccess().Render(r.Context(), w)
}
```

### Pattern 4: Jira Branch Name Extraction

**What:** Extract Jira issue keys from PR branch names using regex pattern `[A-Z]+-\d+`.

**When:** When loading Jira sidebar for a PR.

```go
var jiraKeyPattern = regexp.MustCompile(`[A-Z]+-\d+`)

func extractJiraKeys(branchName string) []string {
    return jiraKeyPattern.FindAllString(branchName, -1)
}
```

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Sharing Handler Struct Between JSON and HTML

**What:** Putting both JSON and HTML rendering methods on the same Handler struct.
**Why bad:** Violates SRP. The JSON handler and web handler have different dependencies (web needs ReviewSubmitService, JiraService). Mixing them creates a god struct.
**Instead:** Separate `Handler` (JSON) and `WebHandler` (HTML) structs, each in their own package.

### Anti-Pattern 2: templ Components Calling Ports Directly

**What:** Passing port interfaces (e.g., `PRStore`) into templ components and having them fetch data.
**Why bad:** Breaks the hexagonal boundary. templ components are part of the driving adapter layer -- they should only receive pre-fetched data.
**Instead:** Handler fetches data via ports/services, maps to view models, passes view models to templ components.

### Anti-Pattern 3: Duplicating Business Logic in Web Handlers

**What:** Re-implementing review enrichment, status aggregation, or attention signal logic in the web handler.
**Why bad:** The logic already exists in `ReviewService`, `HealthService`, etc. Duplicating it creates drift.
**Instead:** Call the existing application services. If a new use case is needed, add it to the application layer.

### Anti-Pattern 4: HTMX Swapping Alpine.js State Containers

**What:** Using `hx-swap` targets that contain `x-data` attributes.
**Why bad:** HTMX replaces the DOM element, destroying Alpine's reactive state.
**Instead:** Keep `x-data` on elements **outside** HTMX swap targets. Or use the `alpine-morph` HTMX extension for cases where this is unavoidable.

### Anti-Pattern 5: Storing Jira Credentials in SQLite

**What:** Adding a settings table for Jira URL/token and managing them through the UI.
**Why bad:** Secrets in the database create a security surface. The app already uses env vars for GitHub credentials.
**Instead:** Jira credentials come from env vars, same as GitHub. The UI shows connection status, not credential management.

---

## Build Tool Integration

### templ Code Generation

templ files (`.templ`) compile to Go code (`.go` files with `_templ.go` suffix). This is a build-time step:

```bash
# Install templ CLI
go install github.com/a-h/templ/cmd/templ@latest

# Generate Go code from .templ files
templ generate

# Watch mode for development
templ generate --watch
```

Generated `_templ.go` files **should be committed** to the repository. This ensures `go build` works without the templ CLI installed.

### Tailwind CSS Build

```bash
# Install Tailwind CLI (standalone binary, no Node.js)
curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64
chmod +x tailwindcss-linux-x64

# Build CSS
./tailwindcss-linux-x64 -i static/css/input.css -o static/css/output.css --minify

# Watch mode for development
./tailwindcss-linux-x64 -i static/css/input.css -o static/css/output.css --watch
```

Tailwind scans templ files for class names. Configure `tailwind.config.js`:

```js
module.exports = {
  content: ["./internal/adapter/driving/web/**/*.templ"],
  // ...
}
```

### Static Asset Embedding

For Docker deployment, embed static assets and serve them from the binary:

```go
//go:embed static
var staticFS embed.FS
```

### Development Workflow

Use `air` for hot reload that orchestrates templ generate + go build:

```toml
# .air.toml
[build]
cmd = "templ generate && go build -o ./tmp/main ./cmd/mygitpanel"
include_ext = ["go", "templ"]
```

---

## Scalability Considerations

| Concern | Current (v1.0) | With GUI (v2.0) | Notes |
|---------|----------------|------------------|-------|
| Concurrent requests | Low (API only) | Medium (browser users) | stdlib net/http handles concurrency well |
| Response size | Small (JSON) | Medium (HTML fragments) | HTMX partials keep responses small |
| Build complexity | `go build` only | templ generate + Tailwind + go build | Use `air` for dev, Makefile for CI |
| Static assets | None | CSS + JS (~50KB total) | Embed in binary for Docker |
| GitHub API calls | Read only | Read + Write (reviews) | Same rate limits; writes are user-triggered (infrequent) |
| Jira API calls | None | Read + Write (comments) | Lazy-loaded per PR detail view |
| Database queries | Per API request | Per page load + HTMX partials | Same queries, same connection pool |

---

## Dependency Summary

### New Dependencies to Add

| Package | Purpose | Confidence |
|---------|---------|-----------|
| `github.com/a-h/templ` | Type-safe HTML templating | HIGH -- well-established, v0.3+ is stable |
| `github.com/ctreminiom/go-atlassian` | Jira REST API client | MEDIUM -- well-maintained, cloud-focused |

### Client-Side Libraries (CDN or vendored)

| Library | Version | Size (gzipped) | Purpose |
|---------|---------|----------------|---------|
| HTMX | 2.x | ~14KB | Partial page updates |
| Alpine.js | 3.x | ~15KB | Client-side UI state |
| GSAP | 3.x | ~25KB | Animations |
| Tailwind CSS | 4.x | Output varies | Utility-first CSS |

### No New Go Dependencies Needed For

- GitHub review submission: already covered by `go-github v82` (existing dependency)
- Static file serving: stdlib `net/http.FileServer` + `embed`
- Route handling: stdlib `net/http.ServeMux` with Go 1.22+ method routing (existing pattern)

---

## Sources

- [templ Project Structure](https://templ.guide/project-structure/project-structure/) -- Official templ docs on component organization
- [templ Template Composition](https://templ.guide/syntax-and-usage/template-composition/) -- Component passing and children patterns
- [go-github pulls_reviews.go](https://github.com/google/go-github/blob/master/github/pulls_reviews.go) -- CreateReview, SubmitReview, CreateCommentInReplyTo methods
- [GitHub REST API: Pull Request Reviews](https://docs.github.com/en/rest/pulls/reviews) -- Official API reference
- [ctreminiom/go-atlassian](https://github.com/ctreminiom/go-atlassian) -- Jira Go client library
- [andygrunwald/go-jira](https://github.com/andygrunwald/go-jira) -- Alternative Jira client
- [hexaGO](https://github.com/edlingao/hexaGO) -- Hexagonal architecture template with Go/templ/HTMX/SQLite
- [HTMX and Alpine.js Integration](https://www.infoworld.com/article/3856520/htmx-and-alpine-js-how-to-combine-two-great-lean-front-ends.html) -- Combining HTMX and Alpine.js patterns
- [Full-Stack Go App with HTMX and Alpine.js](https://ntorga.com/full-stack-go-app-with-htmx-and-alpinejs/) -- Go-specific integration guide
- [Using Alpine.js In HTMX](https://www.bennadel.com/blog/4787-using-alpine-js-in-htmx.htm) -- Alpine state preservation with HTMX swaps
