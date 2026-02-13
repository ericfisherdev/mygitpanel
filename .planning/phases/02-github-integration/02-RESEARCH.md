# Phase 2: GitHub Integration - Research

**Researched:** 2026-02-10
**Domain:** GitHub REST API interaction, polling engine, rate limiting, ETag conditional requests, PR discovery
**Confidence:** HIGH

## Summary

Phase 2 implements the GitHub API adapter (driven port), a polling engine, and the application service that orchestrates PR discovery. The system must fetch PRs authored by the configured user and PRs where the user (or their team) is requested as a reviewer, deduplicate across both queries, track rate limits, use ETags for conditional requests, handle pagination, and support manual refresh.

The standard approach is `google/go-github` v82 for the GitHub REST API client, combined with `gregjones/httpcache` (or the newer `bartventer/httpcache`) for transparent ETag-based conditional request handling at the HTTP transport layer. The go-github library provides typed access to all needed endpoints (`PullRequestsService.List`, response `Rate` struct, pagination via `Response.NextPage`), and its `PullRequest` struct includes `Draft`, `RequestedReviewers`, and `RequestedTeams` fields that map directly to our domain model. Rate limiting is handled by reading `Response.Rate` after each call and by the companion `gofri/go-github-ratelimit` v2 middleware for automatic secondary rate limit handling.

The polling engine follows the standard Go pattern: a `time.Ticker` inside a goroutine that selects on both the ticker channel and `ctx.Done()`. The engine iterates all watched repositories from `RepoStore.ListAll()`, calls the GitHub adapter for each, maps results to domain model structs, and upserts via `PRStore.Upsert()`. Manual refresh bypasses the ticker by sending on a dedicated channel.

**Primary recommendation:** Use `google/go-github` v82 with `gregjones/httpcache` for ETag caching. Build the GitHubClient adapter, a polling service (application layer), and evolve the existing port interface to support the richer discovery needs. Structure as three plans: (1) GitHub adapter implementing the port, (2) polling engine with rate limit tracking, (3) application service wiring PR discovery logic with deduplication.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `google/go-github` | v82.0.0 | Typed Go client for GitHub REST API | Google-maintained, covers all endpoints, handles pagination, rate limit structs, v82 released Jan 2025 |
| `gregjones/httpcache` | latest | RFC-compliant HTTP caching transport (ETag/If-None-Match) | Recommended by go-github docs for conditional requests; transparent to API client; in-memory cache sufficient |
| `gofri/go-github-ratelimit` | v2.0.2 | HTTP middleware for GitHub rate limit handling | Handles both primary and secondary rate limits; sleeps on secondary limits instead of erroring |
| `golang.org/x/oauth2` | latest | OAuth2 HTTP client for token auth | Required transitive dependency of go-github for authenticated requests |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `log/slog` (stdlib) | Go stdlib | Structured logging for poll cycles, rate limit status | Every poll cycle logs results, rate limit remaining |
| `time` (stdlib) | Go stdlib | `time.Ticker` for polling interval, `time.Time` for timestamp comparisons | Polling engine core |
| `sync` (stdlib) | Go stdlib | `sync.Mutex` for ETag store, `sync.WaitGroup` for graceful shutdown | Protecting shared state |
| `context` (stdlib) | Go stdlib | Cancellation propagation through poll cycles | Every GitHub API call |
| `strings` (stdlib) | Go stdlib | Splitting `repoFullName` into owner/name | Adapter mapping |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `gregjones/httpcache` | `bartventer/httpcache` | bartventer is RFC 9111 compliant and newer; gregjones is simpler, battle-tested with go-github, sufficient for our use case |
| `gregjones/httpcache` | `bored-engineer/github-conditional-http-transport` | Specialized for GitHub ETags with rotating tokens; overkill since we use a single long-lived PAT |
| `gregjones/httpcache` | Manual ETag tracking in adapter | More control but reimplements HTTP caching; go-github recommends transport-level caching |
| `gofri/go-github-ratelimit` | Manual rate limit checking | Library handles edge cases (secondary limits, Retry-After headers) that are easy to get wrong |
| `google/go-github` | Raw `net/http` calls | Would require manual JSON parsing, pagination, rate limit header extraction; go-github does all of this |

**Installation:**
```bash
go get github.com/google/go-github/v82
go get github.com/gregjones/httpcache
go get github.com/gofri/go-github-ratelimit/v2
```

## Architecture Patterns

### Recommended Project Structure (Phase 2 additions)
```
internal/
|-- domain/
|   |-- model/
|   |   +-- (existing -- no changes needed)
|   +-- port/
|       |-- driven/
|       |   |-- githubclient.go    # EVOLVE: add methods for review-requested PRs, rate limit info
|       |   |-- prstore.go         # existing, no changes
|       |   +-- repostore.go       # existing, no changes
|       +-- driving/
|           +-- pollservice.go     # NEW: driving port for polling (Start, RefreshRepo, RefreshPR)
|
|-- adapter/
|   +-- driven/
|       |-- sqlite/               # existing, no changes
|       +-- github/
|           +-- client.go          # NEW: go-github adapter implementing GitHubClient port
|
+-- application/
    +-- pollservice.go             # NEW: orchestrates polling, deduplication, persistence
```

### Pattern 1: GitHub Adapter as Driven Port Implementation

**What:** The go-github client is wrapped in an adapter struct that implements the `GitHubClient` port interface. The adapter translates between go-github types (`*github.PullRequest`) and domain model types (`model.PullRequest`). No go-github types leak into the domain.

**When to use:** Always -- this is the hexagonal architecture boundary.

**Example:**
```go
// internal/adapter/driven/github/client.go
package github

import (
    "context"
    "fmt"
    "strings"

    gh "github.com/google/go-github/v82/github"
    "github.com/efisher/reviewhub/internal/domain/model"
)

type Client struct {
    gh       *gh.Client
    username string
}

func NewClient(token, username string) *Client {
    // Transport stack: httpcache -> rate limiter -> default transport
    // (see Code Examples section for full wiring)
    client := gh.NewClient(nil).WithAuthToken(token)
    return &Client{gh: client, username: username}
}

func (c *Client) FetchPullRequests(ctx context.Context, repoFullName string) ([]model.PullRequest, error) {
    owner, repo := splitRepo(repoFullName)
    opts := &gh.PullRequestListOptions{
        State: "open",
        Sort:  "updated",
        ListOptions: gh.ListOptions{PerPage: 100},
    }

    var allPRs []model.PullRequest
    for {
        prs, resp, err := c.gh.PullRequests.List(ctx, owner, repo, opts)
        if err != nil {
            return nil, fmt.Errorf("list PRs for %s: %w", repoFullName, err)
        }
        for _, pr := range prs {
            allPRs = append(allPRs, mapPullRequest(pr, repoFullName))
        }
        if resp.NextPage == 0 {
            break
        }
        opts.Page = resp.NextPage
    }
    return allPRs, nil
}

func splitRepo(fullName string) (owner, repo string) {
    parts := strings.SplitN(fullName, "/", 2)
    return parts[0], parts[1]
}

func mapPullRequest(pr *gh.PullRequest, repoFullName string) model.PullRequest {
    labels := make([]string, 0, len(pr.Labels))
    for _, l := range pr.Labels {
        labels = append(labels, l.GetName())
    }

    status := model.PRStatusOpen
    if pr.GetMergedAt() != nil && !pr.GetMergedAt().IsZero() {
        status = model.PRStatusMerged
    } else if pr.GetState() == "closed" {
        status = model.PRStatusClosed
    }

    return model.PullRequest{
        Number:         pr.GetNumber(),
        RepoFullName:   repoFullName,
        Title:          pr.GetTitle(),
        Author:         pr.GetUser().GetLogin(),
        Status:         status,
        IsDraft:        pr.GetDraft(),
        URL:            pr.GetHTMLURL(),
        Branch:         pr.GetHead().GetRef(),
        BaseBranch:     pr.GetBase().GetRef(),
        Labels:         labels,
        OpenedAt:       pr.GetCreatedAt().Time,
        UpdatedAt:      pr.GetUpdatedAt().Time,
        LastActivityAt: pr.GetUpdatedAt().Time,
    }
}
```

### Pattern 2: Polling Engine with Ticker + Manual Refresh Channel

**What:** A goroutine runs a `time.Ticker` and selects on three channels: ticker (scheduled poll), refresh channel (manual trigger), and `ctx.Done()` (shutdown). The polling logic is in the application service; the engine just schedules it.

**When to use:** For the configurable-interval polling requirement (POLL-01) and manual refresh (POLL-04).

**Example:**
```go
// internal/application/pollservice.go
package application

import (
    "context"
    "log/slog"
    "time"

    "github.com/efisher/reviewhub/internal/domain/port/driven"
)

type PollService struct {
    ghClient  driven.GitHubClient
    prStore   driven.PRStore
    repoStore driven.RepoStore
    username  string
    interval  time.Duration
    refreshCh chan refreshRequest
}

type refreshRequest struct {
    repoFullName string   // empty = all repos
    prNumber     int      // 0 = all PRs in repo
    done         chan error
}

func NewPollService(
    ghClient driven.GitHubClient,
    prStore driven.PRStore,
    repoStore driven.RepoStore,
    username string,
    interval time.Duration,
) *PollService {
    return &PollService{
        ghClient:  ghClient,
        prStore:   prStore,
        repoStore: repoStore,
        username:  username,
        interval:  interval,
        refreshCh: make(chan refreshRequest),
    }
}

func (s *PollService) Start(ctx context.Context) {
    // Run immediately on start, then on interval
    s.pollAll(ctx)

    ticker := time.NewTicker(s.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            slog.Info("polling stopped")
            return
        case <-ticker.C:
            s.pollAll(ctx)
        case req := <-s.refreshCh:
            req.done <- s.handleRefresh(ctx, req)
        }
    }
}

func (s *PollService) RefreshRepo(ctx context.Context, repoFullName string) error {
    done := make(chan error, 1)
    s.refreshCh <- refreshRequest{repoFullName: repoFullName, done: done}
    select {
    case err := <-done:
        return err
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

### Pattern 3: PR Discovery with Deduplication

**What:** For each watched repository, fetch all open PRs, then identify which are authored by the user and which request the user's review. A PR can appear in both categories (authored + review-requested-for-team). Deduplication uses the PR number + repo as composite key (already enforced by the upsert).

**When to use:** In the poll cycle for every repository.

**Example:**
```go
func (s *PollService) pollRepo(ctx context.Context, repoFullName string) error {
    // FetchPullRequests gets ALL open PRs for the repo
    prs, err := s.ghClient.FetchPullRequests(ctx, repoFullName)
    if err != nil {
        return err
    }

    // Filter: authored by user OR review requested from user/team
    // Deduplication is inherent: same PR number can't be upserted twice
    // with different data -- upsert overwrites
    for _, pr := range prs {
        if err := s.prStore.Upsert(ctx, pr); err != nil {
            slog.Error("upsert PR failed", "repo", repoFullName, "number", pr.Number, "error", err)
        }
    }
    return nil
}
```

### Pattern 4: Transport Stack (ETag Caching + Rate Limiting)

**What:** Layer HTTP transports: innermost is `http.DefaultTransport`, wrapped by `httpcache.Transport` for ETag caching, wrapped by `go-github-ratelimit` for secondary rate limit handling. Pass the resulting `http.Client` to `github.NewClient()`.

**When to use:** When constructing the GitHub client in the composition root or adapter constructor.

**Example:**
```go
import (
    "net/http"

    gh "github.com/google/go-github/v82/github"
    "github.com/gregjones/httpcache"
    "github.com/gofri/go-github-ratelimit/v2/github_ratelimit"
)

func NewClient(token, username string) *Client {
    // Layer 1: ETag caching via httpcache (in-memory)
    cachingTransport := httpcache.NewMemoryCacheTransport()

    // Layer 2: Rate limit handling (wraps the caching transport)
    rateLimitClient := github_ratelimit.NewClient(
        cachingTransport.Client(),
    )

    // Layer 3: go-github client with auth token
    client := gh.NewClient(rateLimitClient).WithAuthToken(token)

    return &Client{gh: client, username: username}
}
```

### Anti-Patterns to Avoid

- **Leaking go-github types into domain:** Never import `github.com/google/go-github` in `domain/model/` or `domain/port/`. The adapter maps `*github.PullRequest` to `model.PullRequest`. Port interfaces use only domain types.
- **Polling in the adapter:** The GitHub adapter is a simple API client. The polling schedule, deduplication logic, and persistence orchestration belong in the application service.
- **Single-threaded pagination:** Do not fetch page 1, process it, fetch page 2, process it. Fetch ALL pages first, then process. This avoids partial updates if pagination fails midway.
- **Ignoring rate limit headers:** Every `Response` from go-github contains `Rate.Remaining` and `Rate.Reset`. Log these. Back off when `Remaining` is low (e.g., < 100).
- **Storing ETags in SQLite:** ETags are HTTP caching concerns. Use `httpcache` in-memory transport -- the cache lives in the process and is rebuilt on restart (which is fine for a polling app that immediately re-fetches on startup).
- **Blocking the poll loop on manual refresh:** The refresh channel pattern ensures manual refresh executes within the same goroutine as polling, preventing concurrent GitHub API calls that could race on rate limits.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| GitHub API client | Raw HTTP + JSON parsing | `google/go-github` v82 | Typed structs for 100+ endpoints, pagination helpers, rate limit parsing, error types |
| ETag conditional requests | Manual `If-None-Match` header tracking | `gregjones/httpcache` as HTTP transport | Transparent to API client, handles cache storage, 304 responses, cache invalidation |
| Secondary rate limit handling | Manual `Retry-After` header parsing + sleep | `gofri/go-github-ratelimit` v2 | Handles both primary and secondary limits, exponential backoff, callback support |
| Pagination | Manual `Link` header parsing | go-github `Response.NextPage` + loop pattern | `Response.NextPage` is populated from `Link` headers automatically |
| go-github pointer dereferencing | `if pr.Draft != nil { *pr.Draft }` everywhere | go-github `GetXxx()` methods | `pr.GetDraft()` returns zero value if nil, safe to call on nil pointers |

**Key insight:** go-github already solved all the hard GitHub API problems (pagination, rate limit parsing, pointer safety, typed errors). The adapter's job is purely to map between go-github types and domain model types.

## Common Pitfalls

### Pitfall 1: go-github Pointer Fields Panic on Nil Dereference
**What goes wrong:** go-github uses `*bool`, `*string`, `*int` for all fields (matching GitHub API's omitempty behavior). Direct dereference like `*pr.Draft` panics if the field is nil.
**Why it happens:** GitHub API omits fields that are null/default. go-github represents this with pointer fields.
**How to avoid:** Always use the `GetXxx()` helper methods: `pr.GetDraft()`, `pr.GetTitle()`, `pr.GetUser().GetLogin()`. These return zero values for nil pointers and are safe to chain.
**Warning signs:** Nil pointer dereference panics during PR mapping, especially on fields that are optional in the API response.
**Confidence:** HIGH

### Pitfall 2: Rate Limit Exhaustion During Full Sync
**What goes wrong:** First startup discovers many repos with many PRs. Each repo requires at least one API call (more with pagination). Rate limit budget (5000/hour) is consumed quickly.
**Why it happens:** No rate limit awareness during the initial full sync.
**How to avoid:** Check `Response.Rate.Remaining` after each API call. If remaining drops below a threshold (e.g., 100), log a warning and delay subsequent requests. The `gofri/go-github-ratelimit` middleware handles secondary limits automatically, but primary limit awareness must be explicit.
**Warning signs:** 403 responses with `x-ratelimit-remaining: 0`; `github.RateLimitError` returned from API calls.
**Confidence:** HIGH

### Pitfall 3: Conditional Requests (304) Still Count as API Calls in Some Cases
**What goes wrong:** Developer assumes ETags eliminate all rate limit impact. While 304 responses do NOT count against the primary rate limit when authenticated, the request still costs network round-trip time.
**Why it happens:** Misunderstanding of GitHub's conditional request behavior.
**How to avoid:** ETags save rate limit budget but not network time. Still respect the polling interval. Use `updated_at` timestamp comparisons in addition to ETags to skip processing unchanged PRs.
**Warning signs:** Network traffic on every poll cycle even when nothing changed (expected with ETags, but processing should be skipped for unchanged PRs).
**Confidence:** HIGH (verified via GitHub docs: "Making a conditional request does not count against your primary rate limit if a 304 response is returned")

### Pitfall 4: List PRs Endpoint Does Not Filter by Author or Reviewer
**What goes wrong:** Developer expects `PullRequestListOptions` to have an `Author` or `ReviewRequested` filter. It does not -- the list endpoint only filters by state, head, base, and sort.
**Why it happens:** The GitHub REST API `GET /repos/{owner}/{repo}/pulls` returns ALL open PRs. Filtering by author or reviewer must be done client-side after fetching, or by using the Search API instead.
**How to avoid:** Two approaches: (A) Fetch all open PRs per repo and filter client-side (simpler, works for repos with < 1000 open PRs). (B) Use the Search API (`GET /search/issues?q=...`) to query across repos. Approach A is recommended for our use case since we poll specific watched repos with manageable PR counts. The adapter fetches all PRs and returns them; the application service filters by author/reviewer.
**Warning signs:** Searching for author filter in `PullRequestListOptions` and not finding it; implementing a search API query when it is not needed.
**Confidence:** HIGH (verified: `PullRequestListOptions` has only State, Head, Base, Sort, Direction fields)

### Pitfall 5: Team Review Requests Require Checking `RequestedTeams` Field
**What goes wrong:** Developer only checks `RequestedReviewers` (individual users) and misses `RequestedTeams` (team review requests).
**Why it happens:** The two fields are separate arrays in the GitHub API response.
**How to avoid:** Check both `pr.RequestedReviewers` (for individual user match) AND `pr.RequestedTeams` (for team slug match). The user must configure which teams they belong to, or the system must query the user's team memberships.
**Warning signs:** PRs with team review requests not appearing in the review-requested list; only individually-requested reviews showing up.
**Confidence:** HIGH (verified: go-github `PullRequest` struct has both `RequestedReviewers []*User` and `RequestedTeams []*Team`)

### Pitfall 6: Ticker Drift and Missed Polls
**What goes wrong:** If a poll cycle takes longer than the poll interval, the next tick fires immediately after the current one finishes, but subsequent ticks may be lost (Go's `time.Ticker` drops ticks that arrive while the receiver is not ready).
**Why it happens:** Slow API calls (rate limit backoff, many repos, many pages) cause a poll cycle to exceed the interval.
**How to avoid:** This is acceptable behavior -- we want at most one poll running at a time. The ticker-select pattern naturally handles this because the goroutine blocks in `pollAll()` and only checks the ticker when it returns. Log the poll cycle duration and warn if it exceeds the interval.
**Warning signs:** Poll frequency lower than configured interval; slog showing poll duration > interval.
**Confidence:** HIGH

### Pitfall 7: go-github v82 Breaking Changes
**What goes wrong:** Code examples from older tutorials use method names or types that changed in v82.
**Why it happens:** go-github increments major version frequently (v80 Dec 2024, v81 Jan 2025, v82 Jan 2025). Each version may rename methods or change struct fields.
**How to avoid:** Use v82 import path consistently: `github.com/google/go-github/v82/github`. Check pkg.go.dev for v82 specifically. Key v82 changes: `Repository.Permissions` is now a struct (not `map[string]bool`), `Git.ListMatchingRefs` signature changed.
**Warning signs:** Compile errors after `go get`; type assertion failures on `Permissions` field.
**Confidence:** MEDIUM (v82 changes verified from release notes; specific field details may need validation at implementation time)

## Code Examples

### Full Transport Stack Wiring
```go
// internal/adapter/driven/github/client.go
package github

import (
    "context"
    "fmt"
    "log/slog"
    "net/http"
    "strings"

    gh "github.com/google/go-github/v82/github"
    "github.com/gregjones/httpcache"
    "github.com/gofri/go-github-ratelimit/v2/github_ratelimit"
    "github.com/efisher/reviewhub/internal/domain/model"
)

type Client struct {
    gh       *gh.Client
    username string
}

func NewClient(token, username string) *Client {
    // 1. ETag caching: wraps default transport with in-memory cache
    cacheTransport := httpcache.NewMemoryCacheTransport()

    // 2. Rate limit middleware: wraps caching transport
    rateLimitClient := github_ratelimit.NewClient(
        cacheTransport.Client(),
    )

    // 3. go-github client with auth
    client := gh.NewClient(rateLimitClient).WithAuthToken(token)

    return &Client{gh: client, username: username}
}
```

### Paginated PR Fetch with Rate Limit Logging
```go
func (c *Client) FetchPullRequests(ctx context.Context, repoFullName string) ([]model.PullRequest, error) {
    owner, repo := splitRepo(repoFullName)
    opts := &gh.PullRequestListOptions{
        State:       "open",
        Sort:        "updated",
        Direction:   "desc",
        ListOptions: gh.ListOptions{PerPage: 100},
    }

    var result []model.PullRequest
    for {
        prs, resp, err := c.gh.PullRequests.List(ctx, owner, repo, opts)
        if err != nil {
            return nil, fmt.Errorf("list PRs for %s page %d: %w", repoFullName, opts.Page, err)
        }

        slog.Debug("github API call",
            "endpoint", fmt.Sprintf("repos/%s/pulls", repoFullName),
            "page", opts.Page,
            "count", len(prs),
            "rate_remaining", resp.Rate.Remaining,
            "rate_limit", resp.Rate.Limit,
        )

        for _, pr := range prs {
            result = append(result, mapPullRequest(pr, repoFullName))
        }

        if resp.NextPage == 0 {
            break
        }
        opts.Page = resp.NextPage
    }
    return result, nil
}
```

### Mapping go-github PullRequest to Domain Model
```go
func mapPullRequest(pr *gh.PullRequest, repoFullName string) model.PullRequest {
    labels := make([]string, 0, len(pr.Labels))
    for _, l := range pr.Labels {
        labels = append(labels, l.GetName())
    }

    status := model.PRStatusOpen
    if pr.GetMergedAt() != nil && !pr.GetMergedAt().IsZero() {
        status = model.PRStatusMerged
    } else if pr.GetState() == "closed" {
        status = model.PRStatusClosed
    }

    return model.PullRequest{
        Number:         pr.GetNumber(),
        RepoFullName:   repoFullName,
        Title:          pr.GetTitle(),
        Author:         pr.GetUser().GetLogin(),
        Status:         status,
        IsDraft:        pr.GetDraft(),
        URL:            pr.GetHTMLURL(),
        Branch:         pr.GetHead().GetRef(),
        BaseBranch:     pr.GetBase().GetRef(),
        Labels:         labels,
        OpenedAt:       pr.GetCreatedAt().Time,
        UpdatedAt:      pr.GetUpdatedAt().Time,
        LastActivityAt: pr.GetUpdatedAt().Time,
    }
}
```

### Checking Review Requests (Individual + Team)
```go
// IsReviewRequestedFrom checks if a PR has a pending review request for the user
// either directly or via team membership.
func isReviewRequestedFrom(pr *gh.PullRequest, username string, teamSlugs []string) bool {
    // Check individual review requests
    for _, reviewer := range pr.RequestedReviewers {
        if strings.EqualFold(reviewer.GetLogin(), username) {
            return true
        }
    }
    // Check team review requests
    for _, team := range pr.RequestedTeams {
        for _, slug := range teamSlugs {
            if strings.EqualFold(team.GetSlug(), slug) {
                return true
            }
        }
    }
    return false
}
```

### Polling Engine with Manual Refresh
```go
func (s *PollService) Start(ctx context.Context) {
    // Immediate poll on startup
    if err := s.pollAll(ctx); err != nil {
        slog.Error("initial poll failed", "error", err)
    }

    ticker := time.NewTicker(s.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            slog.Info("poll service stopped")
            return
        case <-ticker.C:
            if err := s.pollAll(ctx); err != nil {
                slog.Error("poll cycle failed", "error", err)
            }
        case req := <-s.refreshCh:
            req.done <- s.handleRefresh(ctx, req)
        }
    }
}

func (s *PollService) pollAll(ctx context.Context) error {
    start := time.Now()
    repos, err := s.repoStore.ListAll(ctx)
    if err != nil {
        return fmt.Errorf("list repos: %w", err)
    }

    for _, repo := range repos {
        if ctx.Err() != nil {
            return ctx.Err()
        }
        if err := s.pollRepo(ctx, repo.FullName); err != nil {
            slog.Error("poll repo failed", "repo", repo.FullName, "error", err)
            // Continue to next repo, don't fail the entire cycle
        }
    }

    slog.Info("poll cycle complete",
        "repos", len(repos),
        "duration", time.Since(start),
    )
    return nil
}
```

### Rate Limit Budget Tracking
```go
// RateLimitInfo holds the current rate limit state from the most recent API call.
type RateLimitInfo struct {
    Remaining int
    Limit     int
    Reset     time.Time
}

// After each go-github call, the Response.Rate field contains current limits:
// resp.Rate.Remaining -- requests left
// resp.Rate.Limit     -- max per hour
// resp.Rate.Reset     -- when window resets (Timestamp wrapping time.Time)

func logRateLimit(resp *gh.Response) {
    if resp == nil {
        return
    }
    slog.Info("rate limit status",
        "remaining", resp.Rate.Remaining,
        "limit", resp.Rate.Limit,
        "reset", resp.Rate.Reset.Time,
    )
    if resp.Rate.Remaining < 100 {
        slog.Warn("rate limit running low",
            "remaining", resp.Rate.Remaining,
            "reset_in", time.Until(resp.Rate.Reset.Time),
        )
    }
}
```

## GitHubClient Port Interface Evolution

The existing `GitHubClient` port interface needs to evolve for Phase 2. The current interface was a placeholder:

```go
// Current (Phase 1 placeholder)
type GitHubClient interface {
    FetchPullRequests(ctx context.Context, repoFullName string) ([]model.PullRequest, error)
    FetchReviews(ctx context.Context, repoFullName string, prNumber int) ([]model.Review, error)
    FetchReviewComments(ctx context.Context, repoFullName string, prNumber int) ([]model.ReviewComment, error)
}
```

**Recommendation:** Keep `FetchPullRequests` as-is for Phase 2 -- it returns ALL open PRs for a repo, and the application service handles filtering/deduplication. `FetchReviews` and `FetchReviewComments` are Phase 4 concerns and can remain unimplemented stubs for now. Add a method for rate limit status if the application service needs it:

```go
// Evolved for Phase 2
type GitHubClient interface {
    FetchPullRequests(ctx context.Context, repoFullName string) ([]model.PullRequest, error)
    FetchReviews(ctx context.Context, repoFullName string, prNumber int) ([]model.Review, error)
    FetchReviewComments(ctx context.Context, repoFullName string, prNumber int) ([]model.ReviewComment, error)
}
```

The port interface itself does NOT need to change for Phase 2. The `FetchPullRequests` method signature is sufficient. Rate limit tracking, ETag caching, and pagination are implementation details of the adapter (hidden behind the port). The application service does not need to know about rate limits -- the adapter handles them transparently via the transport stack.

However, the go-github PullRequest response includes `RequestedReviewers` and `RequestedTeams` which are not part of our domain `model.PullRequest`. The application service needs this information for deduplication (DISC-02, DISC-04, DISC-05). Two approaches:

**Approach A (Recommended):** The adapter's `FetchPullRequests` returns `model.PullRequest` as-is. The adapter ALSO returns metadata about review requests. This requires either enriching the model or returning a wrapper type.

**Approach B:** The application service makes a separate call per PR to check review requests via `GET /repos/{owner}/{repo}/pulls/{number}/requested_reviewers`. This is expensive (one call per PR).

**Recommended approach:** Since the `List PRs` endpoint already returns `requested_reviewers` and `requested_teams` inline, the adapter can classify PRs during mapping. Add a classification field to the application layer (not the domain model) -- or simply have the adapter filter and return two slices:

```go
// Application-layer concept, not a domain port change
type PRDiscoveryResult struct {
    AuthoredPRs        []model.PullRequest
    ReviewRequestedPRs []model.PullRequest
    AllPRs             []model.PullRequest  // deduplicated union
}
```

This keeps the domain model pure and moves classification to the application service where it belongs.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| go-github v60-v68 | go-github v82 | Jan 2025 | v82 has breaking changes to `Repository.Permissions` (struct, not map), `Git.ListMatchingRefs` |
| Manual ETag tracking | `httpcache` transport wrapping | go-github recommendation since 2015 | Transparent caching, no custom code |
| Manual rate limit sleep | `gofri/go-github-ratelimit` v2 | v2 released Mar 2025 | Handles both primary and secondary limits as middleware |
| `github.NewClient(tc)` with oauth2 | `github.NewClient(nil).WithAuthToken(token)` | go-github ~v60+ | Simpler auth setup, no explicit oauth2 client needed |
| go-github per-page iteration | go-github `ListIter` (Go 1.23+) | go-github recent versions | Range-based iteration over paginated results |

**Deprecated/outdated:**
- Using `oauth2.NewClient` for PAT auth: `WithAuthToken()` is simpler and recommended
- `gregjones/httpcache` has not been actively maintained, but still works and is the go-github documented recommendation; `bartventer/httpcache` is a modern alternative if issues arise
- Direct `*github.PullRequest` field access (e.g., `*pr.Draft`): Always use `GetDraft()` helpers to avoid nil panics

## Open Questions

1. **Team membership for DISC-04**
   - What we know: go-github returns `RequestedTeams` on each PR with team `Slug` and `Name`. We can check if the user's team is listed.
   - What's unclear: How does the user configure which teams they belong to? The GitHub API can query team memberships (`GET /user/teams`) but that requires additional scopes.
   - Recommendation: For v1, accept a `REVIEWHUB_GITHUB_TEAMS` environment variable (comma-separated team slugs). The adapter compares `RequestedTeams[].Slug` against this list. In the future, auto-detect via API.
   - Confidence: MEDIUM

2. **Closed/merged PR cleanup**
   - What we know: We fetch `state=open` PRs. PRs that were open last poll but are now closed/merged will not appear in the next fetch.
   - What's unclear: Should we delete them from the database, mark them as closed, or keep them indefinitely?
   - Recommendation: During each poll, compare fetched PR numbers against stored PRs for the repo. Any stored PR not in the fetch result should be re-fetched individually to check if it was closed/merged, then updated in the database. This prevents stale open PRs from lingering.
   - Confidence: MEDIUM

3. **httpcache memory usage**
   - What we know: `httpcache.NewMemoryCacheTransport()` stores all cached responses in memory.
   - What's unclear: How much memory will this consume with many repos and PRs?
   - Recommendation: For v1 with a single user and a moderate number of repos (< 50), in-memory cache is fine. Each cached response is a few KB. Monitor memory if scaling. Switch to `httpcache.NewDiskCacheTransport()` if needed.
   - Confidence: MEDIUM

4. **go-github v82 ListIter availability**
   - What we know: Recent go-github versions added `ListIter` methods for range-based pagination (Go 1.23+). Our `go.mod` has `go 1.25.4`.
   - What's unclear: Whether `PullRequestsService.ListIter` specifically exists in v82 or is named differently.
   - Recommendation: Use the manual pagination loop pattern (proven, works on all versions). If `ListIter` is available, it is a nice simplification but not required. Validate at implementation time.
   - Confidence: LOW

## Sources

### Primary (HIGH confidence)
- [google/go-github GitHub repo](https://github.com/google/go-github) -- v82.0.0, import path, `WithAuthToken()`, pagination pattern
- [go-github pulls.go source](https://github.com/google/go-github/blob/master/github/pulls.go) -- `PullRequest` struct fields (`Draft`, `RequestedReviewers`, `RequestedTeams`), `PullRequestListOptions`, `List` method
- [GitHub REST API: List Pull Requests](https://docs.github.com/en/rest/pulls/pulls) -- Endpoint parameters, response fields, `requested_reviewers`, `requested_teams`, `draft`
- [GitHub REST API: Review Requests](https://docs.github.com/en/rest/pulls/review-requests) -- `GET /repos/{owner}/{repo}/pulls/{number}/requested_reviewers` response format
- [GitHub REST API: Rate Limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api) -- Headers, 5000/hour authenticated, 403/429 responses
- [GitHub REST API: Best Practices](https://docs.github.com/en/rest/guides/best-practices-for-using-the-rest-api) -- Conditional requests, ETag, 304 not counting against rate limit

### Secondary (MEDIUM confidence)
- [gregjones/httpcache](https://github.com/gregjones/httpcache) -- In-memory and disk cache transports, RFC-compliant HTTP caching
- [gofri/go-github-ratelimit](https://github.com/gofri/go-github-ratelimit) -- v2.0.2, primary + secondary rate limit middleware
- [bartventer/httpcache](https://github.com/bartventer/httpcache) -- RFC 9111 compliant alternative, `fscache` and `memcache` backends
- [bored-engineer/github-conditional-http-transport](https://github.com/bored-engineer/github-conditional-http-transport) -- Specialized GitHub ETag transport (evaluated, not recommended for our use case)
- [go-github issue #5: Conditional requests](https://github.com/google/go-github/issues/5) -- Maintainer decision to use transport-level caching

### Tertiary (LOW confidence)
- Training data for Go polling patterns with `time.Ticker` and channel-based manual trigger -- well-established patterns but code examples are illustrative, not from a specific source
- go-github `ListIter` availability in v82 -- inferred from release notes, needs validation at implementation time

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- go-github v82 verified via releases page, httpcache recommended in go-github docs, gofri/go-github-ratelimit verified on pkg.go.dev
- Architecture: HIGH -- hexagonal patterns established in Phase 1, adapter/port/service layering is the same pattern
- GitHub API behavior: HIGH -- endpoints, response fields, rate limit headers, ETag behavior all verified via official GitHub docs
- Pitfalls: HIGH -- go-github pointer fields, rate limit exhaustion, missing team review requests are well-documented issues
- Polling engine: MEDIUM -- pattern is standard Go but code examples are illustrative, not from a verified source
- Team membership detection: MEDIUM -- `RequestedTeams` field verified, but env-var-based team config is a design decision not a technical finding

**Research date:** 2026-02-10
**Valid until:** 2026-03-10 (30 days -- go-github versions increment monthly but API patterns are stable)
