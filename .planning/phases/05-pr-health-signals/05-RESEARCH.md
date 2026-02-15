# Phase 5: PR Health Signals - Research

**Researched:** 2026-02-13
**Domain:** GitHub Checks API, Commit Status API, PR diff stats, merge conflict detection, staleness computation
**Confidence:** HIGH (all APIs verified against go-github source and GitHub official docs)

## Summary

Phase 5 adds CI/CD check status, diff stats, merge conflict detection, and staleness metrics to each PR. This requires data from four distinct GitHub API surfaces: (1) the Checks API for GitHub Actions and App-based check runs, (2) the Commit Statuses API for legacy CI integrations, (3) the single-PR GET endpoint for diff stats and mergeable status (these fields are NOT populated by the List endpoint), and (4) the Branch Protection API for required check identification.

The most significant architectural finding is that **diff stats (`additions`, `deletions`, `changed_files`) and `mergeable` status are only available from the single-PR GET endpoint** (`GET /repos/{owner}/{repo}/pulls/{number}`), not from the List PRs endpoint used by `FetchPullRequests`. This means a new `FetchPRDetail` method is needed on the GitHub adapter that calls `PullRequestsService.Get()` for each changed PR. Additionally, `mergeable` can be `null` when GitHub's background merge-check job hasn't completed yet, requiring a "unknown" status rather than treating null as false.

The second key finding is that **required vs optional check classification is not a field on check run objects**. It requires a separate call to the Branch Protection API (`GET /repos/{owner}/{repo}/branches/{branch}/protection/required_status_checks`) to get the list of required check context names, then cross-referencing against actual check runs. This endpoint requires "Administration" read permissions, which a standard PAT may not have -- hence the requirement says "when token permissions allow."

**Primary recommendation:** Implement in 3 plans: (1) domain model expansion (new fields on PullRequest, new CheckRun/CombinedStatus models) + new port methods + SQLite migration + adapter for check runs and combined status, (2) single-PR detail fetch for diff stats/mergeable + required checks detection from branch protection (best-effort) + health service for aggregation, (3) HTTP API expansion with health signal DTOs on PR responses + poll service integration.

## Standard Stack

### Core (already in project -- no new dependencies)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| google/go-github/v82 | v82.0.0 | GitHub REST API client | Already in use; has `ChecksService`, `RepositoriesService.GetCombinedStatus`, `PullRequestsService.Get`, `RepositoriesService.GetRequiredStatusChecks` |
| modernc.org/sqlite | v1.45.0 | Pure Go SQLite | Already in use; no CGO required |
| golang-migrate/migrate/v4 | v4.19.1 | Embedded SQL migrations | Already in use |
| stretchr/testify | v1.11.1 | Test assertions | Already in use |

### New (none needed)
No new dependencies. All required functionality is covered by existing `go-github/v82` and stdlib.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| go-github ChecksService | Raw HTTP to Checks API | No benefit; go-github already wraps it cleanly with pagination |
| SQLite for check runs | In-memory only (no persistence) | Persistence enables staleness comparison across polls and serves check data without re-fetching |
| Fetching branch protection per-poll | Caching branch protection rules | Branch protection rarely changes; cache in memory with TTL to reduce API calls |

**No new `go get` commands needed** -- all new functionality uses existing dependencies.

## Architecture Patterns

### New/Modified Files
```
internal/
  domain/model/
    pullrequest.go       # Add Additions, Deletions, ChangedFiles, MergeableStatus fields
    checkstatus.go       # Already exists; expand or keep as-is. CheckRun is the per-run entity.
    enums.go             # Add MergeableStatus enum (mergeable/conflicted/unknown)
  domain/port/driven/
    githubclient.go      # Add FetchCheckRuns, FetchCombinedStatus, FetchPRDetail, FetchRequiredStatusChecks
    prstore.go           # (unchanged or add UpdateHealthSignals method)
    checkstore.go        # NEW: CheckStore port for check run persistence
  application/
    healthservice.go     # NEW: Aggregation logic (combine checks+statuses, compute combined CI status)
    pollservice.go       # Extend to call health signal fetching for changed PRs
  adapter/driven/github/
    client.go            # Add FetchCheckRuns, FetchCombinedStatus, FetchPRDetail, FetchRequiredStatusChecks
  adapter/driven/sqlite/
    checkrepo.go         # NEW: SQLite implementation of CheckStore
    prrepo.go            # Update Upsert to include new diff stats/mergeable fields
    migrations/
      000008_*.sql       # Add diff stats + mergeable columns to pull_requests
      000009_*.sql       # Add check_runs table
  adapter/driving/http/
    response.go          # Expand PRResponse with health signal DTOs
    handler.go           # Enrich GetPR response with health signals
```

### Pattern 1: Dual Check Source Aggregation (Checks API + Status API)
**What:** GitHub has two parallel systems for CI status reporting. The newer Checks API (used by GitHub Actions and modern integrations) and the older Commit Statuses API (used by legacy CI like Travis). A PR's true CI status requires querying BOTH and merging results.
**When to use:** Always -- CICD-01 explicitly requires combined status from both APIs.

```go
// Source: GitHub docs - https://docs.github.com/en/rest/checks/runs
// Source: GitHub docs - https://docs.github.com/en/rest/commits/statuses

// FetchCheckRuns calls Checks API: GET /repos/{owner}/{repo}/commits/{ref}/check-runs
// Returns individual check run objects with name, status, conclusion.
func (c *Client) FetchCheckRuns(ctx context.Context, repoFullName string, ref string) ([]model.CheckRun, error) {
    owner, repo, err := splitRepo(repoFullName)
    if err != nil {
        return nil, err
    }
    opts := &gh.ListCheckRunsOptions{ListOptions: gh.ListOptions{PerPage: 100}}
    var allRuns []model.CheckRun
    for {
        result, resp, err := c.gh.Checks.ListCheckRunsForRef(ctx, owner, repo, ref, opts)
        if err != nil {
            return nil, fmt.Errorf("listing check runs for %s@%s: %w", repoFullName, ref, err)
        }
        for _, cr := range result.CheckRuns {
            allRuns = append(allRuns, mapCheckRun(cr))
        }
        if resp.NextPage == 0 {
            break
        }
        opts.Page = resp.NextPage
    }
    return allRuns, nil
}

// FetchCombinedStatus calls Status API: GET /repos/{owner}/{repo}/commits/{ref}/status
// Returns the combined state (success/failure/pending) and individual statuses.
func (c *Client) FetchCombinedStatus(ctx context.Context, repoFullName string, ref string) (*model.CombinedStatus, error) {
    owner, repo, err := splitRepo(repoFullName)
    if err != nil {
        return nil, err
    }
    combined, _, err := c.gh.Repositories.GetCombinedStatus(ctx, owner, repo, ref, nil)
    if err != nil {
        return nil, fmt.Errorf("getting combined status for %s@%s: %w", repoFullName, ref, err)
    }
    return mapCombinedStatus(combined), nil
}
```

### Pattern 2: Single-PR Fetch for Detail-Only Fields
**What:** `Mergeable`, `Additions`, `Deletions`, and `ChangedFiles` are only populated by the single-PR GET endpoint, not by List. The existing `FetchPullRequests` uses List. A new method is needed.
**When to use:** For STAT-04 (diff stats) and STAT-05 (merge conflict status).

```go
// Source: GitHub docs - https://docs.github.com/en/rest/pulls/pulls
// Note: "These fields are not populated by the List operation"

// FetchPRDetail calls GET /repos/{owner}/{repo}/pulls/{number}
// Returns diff stats (additions, deletions, changed_files) and mergeable status.
func (c *Client) FetchPRDetail(ctx context.Context, repoFullName string, prNumber int) (*model.PRDetail, error) {
    owner, repo, err := splitRepo(repoFullName)
    if err != nil {
        return nil, err
    }
    pr, _, err := c.gh.PullRequests.Get(ctx, owner, repo, prNumber)
    if err != nil {
        return nil, fmt.Errorf("getting PR detail for %s#%d: %w", repoFullName, prNumber, err)
    }
    return &model.PRDetail{
        Additions:    pr.GetAdditions(),
        Deletions:    pr.GetDeletions(),
        ChangedFiles: pr.GetChangedFiles(),
        Mergeable:    mapMergeable(pr.Mergeable),
    }, nil
}

// mapMergeable converts the nullable *bool to a MergeableStatus enum.
func mapMergeable(mergeable *bool) model.MergeableStatus {
    if mergeable == nil {
        return model.MergeableUnknown // Background job not yet complete
    }
    if *mergeable {
        return model.MergeableMergeable
    }
    return model.MergeableConflicted
}
```

### Pattern 3: Required Check Detection via Branch Protection (Best-Effort)
**What:** Check runs do not have a "required" field. Required status is determined by the branch protection configuration on the PR's base branch. This requires "Administration" read permissions.
**When to use:** CICD-03 -- "when token permissions allow."

```go
// Source: GitHub docs - https://docs.github.com/en/rest/branches/branch-protection
// Requires: "Administration" repository permissions (read)

// FetchRequiredStatusChecks calls GET /repos/{owner}/{repo}/branches/{branch}/protection/required_status_checks
// Returns nil, nil if branch is not protected or token lacks admin:read permission.
func (c *Client) FetchRequiredStatusChecks(ctx context.Context, repoFullName string, branch string) ([]string, error) {
    owner, repo, err := splitRepo(repoFullName)
    if err != nil {
        return nil, err
    }
    checks, resp, err := c.gh.Repositories.GetRequiredStatusChecks(ctx, owner, repo, branch)
    if err != nil {
        // 404 = branch not protected, 403 = token lacks permissions. Both are non-fatal.
        if resp != nil && (resp.StatusCode == 404 || resp.StatusCode == 403) {
            return nil, nil
        }
        return nil, fmt.Errorf("getting required status checks for %s/%s: %w", repoFullName, branch, err)
    }
    if checks.Checks == nil {
        return nil, nil
    }
    contexts := make([]string, 0, len(*checks.Checks))
    for _, check := range *checks.Checks {
        contexts = append(contexts, check.Context)
    }
    return contexts, nil
}
```

### Pattern 4: Combined CI Status Computation
**What:** The overall CI status for a PR must aggregate both Checks API results and Commit Status API results into a single `passing/failing/pending` value.
**When to use:** CICD-01.

```go
// Source: Design pattern matching GitHub UI behavior

func computeCombinedCIStatus(checkRuns []model.CheckRun, combinedStatus *model.CombinedStatus) model.CIStatus {
    // If no check data at all, status is unknown.
    if len(checkRuns) == 0 && combinedStatus == nil {
        return model.CIStatusUnknown
    }

    hasFailing := false
    hasPending := false

    // Check runs from Checks API
    for _, cr := range checkRuns {
        switch {
        case cr.Status == "completed" && cr.Conclusion == "failure":
            hasFailing = true
        case cr.Status == "completed" && cr.Conclusion == "cancelled":
            hasFailing = true
        case cr.Status == "completed" && cr.Conclusion == "timed_out":
            hasFailing = true
        case cr.Status == "completed" && cr.Conclusion == "action_required":
            hasFailing = true
        case cr.Status != "completed": // queued, in_progress, waiting, requested, pending
            hasPending = true
        // success, neutral, skipped are passing
        }
    }

    // Combined status from Status API
    if combinedStatus != nil {
        switch combinedStatus.State {
        case "failure":
            hasFailing = true
        case "pending":
            hasPending = true
        // "success" is passing
        }
    }

    if hasFailing {
        return model.CIStatusFailing
    }
    if hasPending {
        return model.CIStatusPending
    }
    return model.CIStatusPassing
}
```

### Pattern 5: Staleness Computation (Already Implemented)
**What:** The existing `PullRequest` model already has `DaysSinceOpened()` and `DaysSinceLastActivity()` methods, plus `OpenedAt` and `LastActivityAt` are already persisted.
**When to use:** STAT-03.

```go
// Source: Already in internal/domain/model/pullrequest.go
// No new code needed -- just expose in HTTP response DTOs.

func (pr PullRequest) DaysSinceOpened() int {
    return int(time.Since(pr.OpenedAt).Hours() / 24)
}

func (pr PullRequest) DaysSinceLastActivity() int {
    return int(time.Since(pr.LastActivityAt).Hours() / 24)
}
```

### Anti-Patterns to Avoid
- **Fetching check runs from HTTP handlers:** Check data should be fetched during poll cycle and persisted. Never call GitHub API from HTTP request path.
- **Treating `mergeable == null` as `false`:** Null means "not yet computed." Report as "unknown" to the consumer, not "conflicted."
- **Fetching single-PR detail for ALL PRs on every poll:** Only fetch for PRs that changed (already have UpdatedAt comparison). Even then, this is 1 extra API call per changed PR.
- **Hard-failing on branch protection 403/404:** Many repos don't have branch protection. Many tokens lack admin:read. Always treat as optional.
- **Mixing check runs and commit statuses in the same model:** They come from different APIs with different fields. Keep separate domain models, unify only at the aggregation layer.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Check run pagination | Manual URL building | go-github `Checks.ListCheckRunsForRef` with `ListOptions.Page` | Handles pagination, auth, error wrapping |
| Combined status aggregation from Status API | Custom HTTP to `/repos/{o}/{r}/commits/{ref}/status` | go-github `Repositories.GetCombinedStatus` | Returns parsed `CombinedStatus` struct |
| Single PR detail fetch | Custom HTTP to pulls endpoint | go-github `PullRequests.Get` | Returns full `PullRequest` with Mergeable, Additions, etc. |
| Branch protection required checks | Custom HTTP to branch protection | go-github `Repositories.GetRequiredStatusChecks` | Handles auth, returns parsed `RequiredStatusChecks` |
| Staleness computation | Complex date math | Already implemented `DaysSinceOpened()` and `DaysSinceLastActivity()` methods on PullRequest model | Already there and tested |
| Rate limiting for new API calls | Manual 429 handling | Existing httpcache + go-github-ratelimit transport stack | Already configured for all go-github calls |

**Key insight:** go-github v82 already has complete type-safe wrappers for every GitHub API endpoint needed in this phase. No raw HTTP calls required (unlike Phase 4's GraphQL).

## Common Pitfalls

### Pitfall 1: Mergeable Null Means "Computing," Not "Unknown Forever"
**What goes wrong:** Treating `mergeable == null` as permanent, displaying "unknown" when GitHub just needs a moment to compute.
**Why it happens:** The GitHub API starts a background merge-check job on the first GET request. Subsequent requests return the computed value.
**How to avoid:** Map null to `MergeableUnknown` enum value. The next poll cycle will re-fetch and likely get a non-null value. Do NOT retry immediately (waste of API calls). The polling interval (5min) is sufficient.
**Warning signs:** All PRs showing "unknown" merge status on first poll, resolving on second poll.

### Pitfall 2: Diff Stats Not Available from List Endpoint
**What goes wrong:** Expecting `Additions`, `Deletions`, `ChangedFiles` to be populated from `FetchPullRequests` (which uses List).
**Why it happens:** go-github's `PullRequest` struct has these fields, but GitHub's List endpoint does not populate them. Only the single-PR GET endpoint does.
**How to avoid:** Add a separate `FetchPRDetail` method that calls `PullRequests.Get()`. Call it during poll for changed PRs only.
**Warning signs:** All PRs showing 0 additions/deletions/changed_files.

### Pitfall 3: Two Separate Check Systems (Checks API vs Status API)
**What goes wrong:** Only querying one API and missing CI results from the other.
**Why it happens:** GitHub has two parallel systems. Modern integrations (GitHub Actions) use the Checks API. Older integrations (Travis, some Jenkins configs) use the Status API. Both can coexist on the same commit.
**How to avoid:** Always query BOTH `ListCheckRunsForRef` and `GetCombinedStatus`. Aggregate results in the health service.
**Warning signs:** CI status showing "passing" when the PR page on GitHub shows failing checks (or vice versa).

### Pitfall 4: Branch Protection 404 on Unprotected Branches
**What goes wrong:** Treating a 404 from GetRequiredStatusChecks as an error.
**Why it happens:** Unprotected branches return 404, not an empty response. This is expected behavior.
**How to avoid:** Check `resp.StatusCode == 404` and return nil (no required checks). Also handle 403 (insufficient permissions) gracefully.
**Warning signs:** Error logs filled with "branch protection not found" messages for repos without branch protection.

### Pitfall 5: Rate Limit Impact of Single-PR Fetches
**What goes wrong:** Calling `PullRequests.Get` for every PR on every poll cycle burns rate limits.
**Why it happens:** Single-PR GET + check runs + combined status = 3 extra API calls per PR per poll.
**How to avoid:** Only fetch health signals for PRs that changed since last poll (use existing `UpdatedAt` comparison from `pollRepo`). Consider caching branch protection rules in memory (they rarely change).
**Warning signs:** Rate limit warnings appearing after adding health signal fetching.

### Pitfall 6: Check Run Status vs Conclusion Confusion
**What goes wrong:** Using `status` field where `conclusion` is needed, or vice versa.
**Why it happens:** Check runs have two separate fields: `status` (queued/in_progress/completed) and `conclusion` (success/failure/neutral/cancelled/skipped/timed_out/action_required). Conclusion is only set when status is "completed."
**How to avoid:** First check `status`. If "completed", look at `conclusion`. If not "completed", treat as pending.
**Warning signs:** Check runs showing as "passing" while still in progress, or "failing" when they're queued.

## Code Examples

### Domain Model: New Fields on PullRequest
```go
// Source: go-github PullRequest struct fields verified at
// https://github.com/google/go-github/blob/master/github/pulls.go

// Add to existing PullRequest struct in pullrequest.go:
type PullRequest struct {
    // ... existing fields ...

    // Health signal fields (from single-PR GET endpoint)
    Additions       int
    Deletions       int
    ChangedFiles    int
    MergeableStatus MergeableStatus // mergeable/conflicted/unknown
    CIStatus        CIStatus        // passing/failing/pending/unknown (computed)
}
```

### Domain Model: CheckRun Entity
```go
// Represents an individual CI/CD check run from the Checks API.
type CheckRun struct {
    ID         int64
    PRID       int64  // Foreign key to pull_requests
    Name       string
    Status     string // queued, in_progress, completed, waiting, requested, pending
    Conclusion string // success, failure, neutral, cancelled, skipped, timed_out, action_required
    IsRequired bool   // From branch protection cross-reference
    DetailsURL string
    StartedAt  time.Time
    CompletedAt time.Time
}
```

### Domain Model: MergeableStatus Enum
```go
// Add to enums.go:
type MergeableStatus string

const (
    MergeableMergeable  MergeableStatus = "mergeable"
    MergeableConflicted MergeableStatus = "conflicted"
    MergeableUnknown    MergeableStatus = "unknown"
)
```

### Mapping go-github CheckRun to Domain Model
```go
// Source: go-github CheckRun struct
// https://github.com/google/go-github/blob/master/github/checks.go

func mapCheckRun(cr *gh.CheckRun) model.CheckRun {
    var startedAt, completedAt time.Time
    if cr.StartedAt != nil {
        startedAt = cr.StartedAt.Time
    }
    if cr.CompletedAt != nil {
        completedAt = cr.CompletedAt.Time
    }

    return model.CheckRun{
        ID:          cr.GetID(),
        Name:        cr.GetName(),
        Status:      cr.GetStatus(),
        Conclusion:  cr.GetConclusion(),
        IsRequired:  false, // Set later from branch protection data
        DetailsURL:  cr.GetDetailsURL(),
        StartedAt:   startedAt,
        CompletedAt: completedAt,
    }
}
```

### Mapping go-github CombinedStatus to Domain Model
```go
// Source: go-github CombinedStatus struct
// https://github.com/google/go-github/blob/master/github/repos_statuses.go

type CombinedStatus struct {
    State    string           // "success", "failure", "pending"
    Statuses []CommitStatus   // Individual statuses
}

type CommitStatus struct {
    Context     string // CI service identifier (e.g., "ci/circleci")
    State       string // success, failure, pending, error
    Description string
    TargetURL   string
}

func mapCombinedStatus(cs *gh.CombinedStatus) *model.CombinedStatus {
    statuses := make([]model.CommitStatus, 0, len(cs.Statuses))
    for _, s := range cs.Statuses {
        statuses = append(statuses, model.CommitStatus{
            Context:     s.GetContext(),
            State:       s.GetState(),
            Description: s.GetDescription(),
            TargetURL:   s.GetTargetURL(),
        })
    }
    return &model.CombinedStatus{
        State:    cs.GetState(),
        Statuses: statuses,
    }
}
```

### SQLite Migration: Diff Stats + Mergeable on pull_requests
```sql
-- 000008_add_health_signals.up.sql
ALTER TABLE pull_requests ADD COLUMN additions INTEGER NOT NULL DEFAULT 0;
ALTER TABLE pull_requests ADD COLUMN deletions INTEGER NOT NULL DEFAULT 0;
ALTER TABLE pull_requests ADD COLUMN changed_files INTEGER NOT NULL DEFAULT 0;
ALTER TABLE pull_requests ADD COLUMN mergeable_status TEXT NOT NULL DEFAULT 'unknown';
ALTER TABLE pull_requests ADD COLUMN ci_status TEXT NOT NULL DEFAULT 'unknown';
```

### SQLite Migration: Check Runs Table
```sql
-- 000009_add_check_runs.up.sql
CREATE TABLE IF NOT EXISTS check_runs (
    id INTEGER PRIMARY KEY,  -- GitHub check run ID, not autoincrement
    pr_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT '',
    conclusion TEXT NOT NULL DEFAULT '',
    is_required INTEGER NOT NULL DEFAULT 0,
    details_url TEXT NOT NULL DEFAULT '',
    started_at DATETIME,
    completed_at DATETIME,
    FOREIGN KEY (pr_id) REFERENCES pull_requests(id) ON DELETE CASCADE
);

CREATE INDEX idx_check_runs_pr_id ON check_runs(pr_id);
```

### HTTP Response DTO Expansion
```go
// Add to PRResponse in response.go:
type PRResponse struct {
    // ... existing fields ...

    // Health signals
    DaysSinceOpened       int                   `json:"days_since_opened"`
    DaysSinceLastActivity int                   `json:"days_since_last_activity"`
    Additions             int                   `json:"additions"`
    Deletions             int                   `json:"deletions"`
    ChangedFiles          int                   `json:"changed_files"`
    MergeableStatus       string                `json:"mergeable_status"`
    CIStatus              string                `json:"ci_status"`
    CheckRuns             []CheckRunResponse    `json:"check_runs"`
    CommitStatuses        []CommitStatusResponse `json:"commit_statuses"`
}

type CheckRunResponse struct {
    ID         int64  `json:"id"`
    Name       string `json:"name"`
    Status     string `json:"status"`
    Conclusion string `json:"conclusion"`
    IsRequired bool   `json:"is_required"`
    DetailsURL string `json:"details_url"`
}

type CommitStatusResponse struct {
    Context     string `json:"context"`
    State       string `json:"state"`
    Description string `json:"description"`
    TargetURL   string `json:"target_url"`
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Status API only (contexts) | Checks API + Status API both | GitHub Checks API GA ~2019 | Must query both for complete CI picture |
| `contexts` array in branch protection | `checks` array with `{context, app_id}` | ~2021 | `contexts` deprecated; `checks` is current |
| `mergeable` as immediate boolean | `mergeable` as nullable (null = computing) | Always been this way but often misunderstood | Must handle three states, not two |

**Deprecated/outdated:**
- `required_status_checks.contexts` is deprecated in favor of `required_status_checks.checks` array. Both are still returned by the API but `checks` is the current field to use.

## Key Technical Decisions for Planner

### 1. When to Fetch Health Signals: During Poll or On-Demand
**Recommendation: During poll for changed PRs only.** Health signals should be fetched alongside review data when a PR's `UpdatedAt` has changed. This keeps data reasonably fresh (within poll interval) without extra API calls on HTTP requests. The additional API calls per changed PR are: 1 for PR detail (diff stats + mergeable), 1 for check runs, 1 for combined status = 3 extra calls per changed PR.

### 2. Staleness: Compute vs Store
**Recommendation: Compute at response time.** `DaysSinceOpened()` and `DaysSinceLastActivity()` already exist as methods on the domain model and compute from `OpenedAt`/`LastActivityAt`. These are inherently time-dependent values that should NOT be persisted (they'd be stale immediately). Compute them when building the HTTP response DTO.

### 3. Check Runs: Persist to SQLite or In-Memory Only
**Recommendation: Persist to SQLite.** Check runs should be stored so the HTTP handler can serve them from the DB without calling GitHub. The check_runs table uses the GitHub check run ID as primary key for idempotent upsert. Delete all check runs for a PR before inserting fresh ones on each poll (full replacement strategy, simpler than per-run upsert for stale check cleanup).

### 4. Required Checks: Best-Effort with Graceful Degradation
**Recommendation: Attempt branch protection read, gracefully degrade.** Call `GetRequiredStatusChecks` for the PR's base branch. If 404 (no protection) or 403 (no admin:read permission), return all checks with `is_required = false`. Cache required check contexts per branch in memory during a poll cycle to avoid redundant API calls for PRs targeting the same base branch.

### 5. Combined CI Status: Persist the Computed Value
**Recommendation: Store `ci_status` on the pull_requests table.** Compute the combined CI status in the health service (aggregating checks + statuses) and persist the single `CIStatus` enum value. This allows the list endpoint to show CI status without fetching check run details.

### 6. Commit Statuses: Persist or Ephemeral
**Recommendation: Do NOT persist individual commit statuses to a separate table.** The `CombinedStatus.State` (success/failure/pending) is sufficient for the aggregated CI status. For the detail view, include individual commit statuses from the CombinedStatus alongside check runs. Store only the aggregated `ci_status` on pull_requests. If individual commit statuses need to be shown in the detail endpoint, fetch them from the CombinedStatus during the poll cycle and persist alongside check runs (or store as JSON column).

**Refined recommendation:** Persist individual commit statuses in the same `check_runs` table with a `source` discriminator column (`source = 'check'` vs `source = 'status'`), or create a separate `commit_statuses` table. The simpler approach is a JSON column on `pull_requests` for commit statuses since they're few per PR (typically 0-5).

## Open Questions

1. **Rate limit impact of FetchPRDetail per changed PR**
   - What we know: Each changed PR needs 1 additional API call for `PullRequests.Get()`. With ETag caching, unchanged PRs won't consume rate limit.
   - What's unclear: For a user tracking 30+ PRs with frequent updates, could this push rate limits too high?
   - Recommendation: Monitor in logs. If rate limit is a concern, consider making `FetchPRDetail` optional or reducing frequency (every Nth poll). The existing poll-only-on-change optimization already limits this.

2. **Branch protection caching strategy**
   - What we know: Branch protection rules rarely change. Multiple PRs target the same base branch.
   - What's unclear: Best TTL for in-memory cache. Whether to cache globally or per-poll-cycle.
   - Recommendation: Cache per-poll-cycle (simple map in health service, cleared at start of each poll). No TTL complexity needed.

3. **Commit statuses storage strategy**
   - What we know: Combined status gives us the aggregate state. Individual statuses have context, state, description, target_url.
   - What's unclear: Whether consumers need individual commit statuses listed alongside check runs, or just the aggregate.
   - Recommendation: Start by persisting individual commit statuses in a simple model similar to check_runs. This satisfies CICD-02 ("lists individual check runs with name, status, and conclusion") which should include both check runs and commit statuses as the unified view.

## Sources

### Primary (HIGH confidence)
- [go-github checks.go](https://github.com/google/go-github/blob/master/github/checks.go) - `ChecksService.ListCheckRunsForRef`, `CheckRun` struct fields verified
- [go-github repos_statuses.go](https://github.com/google/go-github/blob/master/github/repos_statuses.go) - `RepositoriesService.GetCombinedStatus`, `CombinedStatus`/`RepoStatus` struct fields verified
- [go-github pulls.go](https://github.com/google/go-github/blob/master/github/pulls.go) - `PullRequestsService.Get`, `PullRequest.Mergeable/Additions/Deletions/ChangedFiles` fields verified. Note: "not populated by the List operation"
- [go-github repos.go](https://github.com/google/go-github/blob/master/github/repos.go) - `RepositoriesService.GetRequiredStatusChecks`, `RequiredStatusChecks` struct verified
- [GitHub REST API - Check Runs](https://docs.github.com/en/rest/checks/runs) - Endpoint, permissions ("Checks" read), response structure confirmed
- [GitHub REST API - Commit Statuses](https://docs.github.com/en/rest/commits/statuses) - Combined status endpoint, state values (failure/pending/success) confirmed
- [GitHub REST API - Pull Requests](https://docs.github.com/en/rest/pulls/pulls) - Single PR GET populates Mergeable/diff stats; List does not. Mergeable null means background job computing.
- [GitHub REST API - Branch Protection](https://docs.github.com/en/rest/branches/branch-protection) - Required status checks endpoint, "Administration" read permission required

### Secondary (MEDIUM confidence)
- [GitHub About Status Checks](https://docs.github.com/articles/about-status-checks) - Explanation of checks vs statuses, required vs optional
- [go-github Issue #625](https://github.com/google/go-github/issues/625) - Branch protection 404 for unprotected branches (confirmed expected behavior)
- [go-github Issue #830](https://github.com/google/go-github/issues/830) - Mergeable null handling confirmed

### Tertiary (LOW confidence)
- Rate limit impact of per-PR detail fetches -- not empirically tested, estimated from API call counts

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries already in use, go-github v82 has all needed service methods
- Architecture (Checks + Status APIs): HIGH - Both APIs verified with struct fields and endpoints
- Architecture (single-PR fetch for diff stats): HIGH - Verified that List does not populate these fields; Get does
- Architecture (required checks detection): HIGH - Endpoint and permissions verified; graceful degradation documented
- Pitfalls: HIGH - Mergeable null behavior, dual check systems, branch protection 404 all verified from official docs
- Staleness: HIGH - Already implemented in domain model, just needs DTO exposure

**Research date:** 2026-02-13
**Valid until:** 2026-03-13 (stable domain -- GitHub API changes are slow and backward-compatible)
