# Phase 4: Review Intelligence - Research

**Researched:** 2026-02-12
**Domain:** GitHub review comments, threading, code context formatting, bot detection
**Confidence:** HIGH (core APIs verified against go-github source and GitHub official docs)

## Summary

Phase 4 implements the core differentiator of ReviewHub: transforming raw GitHub review data into AI-agent-consumable structured output. This requires fetching three distinct data types from GitHub (reviews, review comments, and issue comments), persisting them in SQLite, and serving them through enriched API responses with code context, threading, suggestion extraction, and bot detection.

The most significant technical finding is that **thread resolution status (`isResolved`) is NOT available via the GitHub REST API**. It requires either the GraphQL API or a pragmatic workaround. The recommended approach is a lightweight GraphQL query using raw `net/http` (no new library dependency) to fetch only thread resolution data, keeping the bulk of data fetching on the existing REST API + go-github stack.

**Primary recommendation:** Implement in 4 plans: (1) domain model expansion + new store ports + migrations, (2) GitHub adapter implementation for reviews/comments with REST API + minimal GraphQL for thread resolution, (3) comment enrichment service (threading, suggestion extraction, bot detection, outdated detection), (4) HTTP API endpoints with enriched DTOs and bot configuration.

## Standard Stack

### Core (already in project)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| google/go-github/v82 | v82.0.0 | GitHub REST API client | Already in use; has PullRequestReview, PullRequestComment, IssueComment structs with all needed fields |
| modernc.org/sqlite | v1.45.0 | Pure Go SQLite | Already in use; no CGO required |
| golang-migrate/migrate/v4 | v4.19.1 | Embedded SQL migrations | Already in use |
| stretchr/testify | v1.11.1 | Test assertions | Already in use |

### New (minimal additions)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| (none needed) | - | GraphQL thread resolution | Use raw `net/http` POST to `api.github.com/graphql` -- single query, no library justified |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Raw HTTP for GraphQL | shurcooL/githubv4 | Full GraphQL client is overkill for a single query; adds dependency + oauth2 transitive dep. Raw HTTP is ~30 lines, easily testable |
| GraphQL for thread resolution | Skip thread resolution entirely | Requirement REVW-04 requires distinguishing resolved from open threads |
| GraphQL for ALL review data | REST API + targeted GraphQL | REST is already integrated with ETag caching + rate limiting; GraphQL costs "points" differently. Only use GraphQL for data REST cannot provide |

**No new `go get` commands needed** -- all new functionality uses existing dependencies plus stdlib `net/http` for the single GraphQL call.

## Architecture Patterns

### New Domain Model Entities and Expansions

The existing `Review` and `ReviewComment` domain models need field additions. A new `IssueComment` model is needed for PR-level (non-inline) comments. A `BotConfig` model is needed for configurable bot detection.

```text
internal/
  domain/model/
    review.go            # Add CommitID field for outdated detection
    reviewcomment.go     # Add StartLine, SubjectType, Suggestion fields
    issuecomment.go      # NEW: PR-level discussion comments
    botconfig.go         # NEW: Configurable bot usernames
    enums.go             # Add CommentType enum (inline vs general)
  domain/port/driven/
    githubclient.go      # FetchReviews/FetchReviewComments already stubbed; add FetchIssueComments, FetchThreadResolution
    prstore.go           # (unchanged)
    reviewstore.go       # NEW: ReviewStore port for reviews + review comments
    botconfigstore.go    # NEW: BotConfigStore port
  application/
    reviewservice.go     # NEW: Enrichment logic (threading, suggestions, bot detection, outdated)
    pollservice.go       # Extend to fetch reviews/comments during poll cycle
  adapter/driven/github/
    client.go            # Implement FetchReviews, FetchReviewComments, FetchIssueComments
    graphql.go           # NEW: Minimal GraphQL client for thread resolution only
  adapter/driven/sqlite/
    reviewrepo.go        # NEW: SQLite implementation of ReviewStore
    botconfigrepo.go     # NEW: SQLite implementation of BotConfigStore
    migrations/
      000003_*.sql       # Reviews table
      000004_*.sql       # Review comments table
      000005_*.sql       # Issue comments table
      000006_*.sql       # Bot config table
  adapter/driving/http/
    handler.go           # Extend GetPR to include enriched reviews/comments
    response.go          # Expand PRResponse with review/comment DTOs
    handler_botconfig.go # NEW: CRUD for bot configuration
```

### Pattern 1: Three-Tier Comment Architecture
**What:** GitHub has three distinct comment types that must be fetched from different API endpoints and unified in the response.
**When to use:** Always -- this is fundamental to understanding GitHub's data model.

| Comment Type | GitHub API Endpoint | go-github Service | Domain Model |
|-------------|---------------------|-------------------|-------------|
| Reviews (top-level) | `GET /repos/{o}/{r}/pulls/{n}/reviews` | `PullRequestsService.ListReviews` | `model.Review` |
| Review Comments (inline) | `GET /repos/{o}/{r}/pulls/{n}/comments` | `PullRequestsService.ListComments` | `model.ReviewComment` |
| Issue Comments (general) | `GET /repos/{o}/{r}/issues/{n}/comments` | `IssuesService.ListComments` | `model.IssueComment` |

### Pattern 2: Outdated Review Detection via CommitID Comparison
**What:** A review is "outdated" when its `commit_id` does not match the PR's current `head.sha`. The go-github `PullRequestReview` struct has a `CommitID *string` field, and the `PullRequest` struct has `Head.SHA`.
**When to use:** REVW-05 requirement.

```go
// Source: go-github PullRequestReview.CommitID and PullRequest.Head.SHA
func isReviewOutdated(review model.Review, prHeadSHA string) bool {
    return review.CommitID != "" && review.CommitID != prHeadSHA
}
```

The PullRequest domain model needs a new `HeadSHA` field (persisted) to support this comparison without re-fetching from GitHub.

### Pattern 3: Comment Threading via InReplyTo
**What:** Review comments form threads via the `in_reply_to_id` field. The root comment has `InReplyToID == nil`, and replies reference the root comment's ID. GitHub only supports one level of threading -- replies to replies are flattened to the root.
**When to use:** CFMT-03 requirement.

```go
// Thread grouping in the enrichment service
type CommentThread struct {
    RootComment   model.ReviewComment
    Replies       []model.ReviewComment
    IsResolved    bool
}

func groupIntoThreads(comments []model.ReviewComment) []CommentThread {
    roots := map[int64]*CommentThread{}
    for _, c := range comments {
        if c.InReplyToID == nil {
            roots[c.ID] = &CommentThread{RootComment: c}
        }
    }
    for _, c := range comments {
        if c.InReplyToID != nil {
            if thread, ok := roots[*c.InReplyToID]; ok {
                thread.Replies = append(thread.Replies, c)
            }
        }
    }
    // ...
}
```

### Pattern 4: Suggestion Block Extraction via Regex
**What:** GitHub suggestion blocks are markdown code fences with `suggestion` as the language identifier. They can use 3+ backticks. Must extract the proposed code change.
**When to use:** CFMT-04 requirement.

```go
// Regex to extract suggestion blocks from comment body
// Handles ```suggestion, ````suggestion, etc.
var suggestionBlockRegex = regexp.MustCompile("(?s)`{3,}suggestion\\s*\n(.*?)\n`{3,}")

type Suggestion struct {
    OriginalBody string // Full comment body
    ProposedCode string // Extracted code from suggestion block
    FilePath     string // From the review comment's Path field
    StartLine    int    // From the review comment's StartLine/Line fields
    EndLine      int
}
```

### Pattern 5: Bot Detection via Configurable Username List
**What:** Bot reviews are detected by matching the reviewer's login against a configurable list of bot usernames. Default list includes "coderabbitai" and common bots.
**When to use:** REVW-02, CFMT-06, STAT-06, REPO-04 requirements.

```go
// BotConfig stored in SQLite, configurable via API
type BotConfig struct {
    ID       int64
    Username string // e.g., "coderabbitai", "github-actions[bot]", "copilot[bot]"
    AddedAt  time.Time
}

func isBotReview(reviewerLogin string, botUsernames []string) bool {
    login := strings.ToLower(reviewerLogin)
    for _, bot := range botUsernames {
        if strings.ToLower(bot) == login {
            return true
        }
    }
    return false
}
```

### Pattern 6: Coderabbit Nitpick Detection via Body Parsing
**What:** CodeRabbit marks nitpick comments in collapsible `<details>` sections with summary text containing "Nitpick comments". Individual inline comments from CodeRabbit that are nitpicks appear under this section. For inline review comments, the approach is to check if the comment author is a known bot AND the body contains nitpick indicators.
**When to use:** REVW-03, CFMT-06 requirements.

Based on analysis of the `coderabbit-review-helper` project, CodeRabbit's patterns are:
- **Bot username:** `coderabbitai` (appears as `coderabbitai[bot]` on GitHub)
- **Summary review markers:** `<!-- This is an auto-generated comment: summarize by coderabbit.ai -->`
- **Section headers in summary:** `"Nitpick comments"`, `"Outside diff range comments"`, `"Actionable comments posted:"`
- **Resolution markers:** Patterns like `Addressed in commit [sha]`, `Resolved in commit [sha]`
- **AI agent prompt section:** `"Prompt for AI Agents"` in collapsible details

For individual inline review comments from CodeRabbit, nitpick detection requires heuristic body parsing since CodeRabbit does not use a consistent machine-readable marker in individual inline comment bodies. The most reliable approach is: if the comment author is `coderabbitai[bot]` AND the parent summary review lists it under the "Nitpick comments" section, flag it as a nitpick.

**Pragmatic approach:** Since individual inline comments lack a reliable nitpick marker, use a simple body-text heuristic: check for common patterns like `**Nitpick**`, `nitpick:`, `(nitpick)`, or `[nitpick]` in the comment body. This is configurable and extensible.

### Anti-Patterns to Avoid
- **Fetching reviews/comments on every API request:** Fetch during poll cycle, persist to SQLite, serve from DB. Never call GitHub API from HTTP handlers.
- **Single monolithic comment table:** Reviews, review comments, and issue comments are structurally different. Use separate tables.
- **Storing raw go-github types:** Translate to domain types in the adapter. Never leak `*gh.PullRequestComment` into the domain.
- **Blocking poll cycle on GraphQL:** GraphQL thread resolution is supplementary. If it fails, log a warning and continue with `IsResolved = false` as default.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| GitHub API pagination | Manual page tracking | go-github's `ListOptions` + response `NextPage` | Already proven in FetchPullRequests, handles edge cases |
| Suggestion block parsing | Custom markdown parser | Regex on code fence pattern | Suggestion blocks are simple fenced code with known language tag |
| Rate limiting | Manual 429 handling | Existing httpcache + go-github-ratelimit transport stack | Already configured and tested |
| Review state aggregation | Custom state machine | Simple latest-review-per-reviewer query | GitHub already tracks this -- just read `state` from latest review per user |
| Comment body sanitization | HTML stripping/XSS protection | bluemonday HTML sanitizer (see `internal/adapter/driving/web/markdown.go`) | Phase 7 introduced Markdown-to-HTML rendering for the web GUI; all comment bodies are sanitized via bluemonday before display. The JSON API still serves raw Markdown for AI agent consumption. |

**Key insight:** Most complexity in this phase is data modeling and enrichment logic, not API integration. The go-github library already provides all the struct fields needed.

## Common Pitfalls

### Pitfall 1: Confusing Review Comments with Issue Comments
**What goes wrong:** Missing general PR-level comments because they live on the Issues API, not the Pull Requests API. Or double-counting comments that appear in both.
**Why it happens:** GitHub's data model splits PR comments across two APIs. A "comment on a PR" posted via the conversation tab is an Issue Comment, while a "comment on a line of code" is a Review Comment.
**How to avoid:** Fetch from both `PullRequestsService.ListComments` (inline) and `IssuesService.ListComments` (general). Never mix the two in the same table. The `subject_type` field on review comments distinguishes `line` vs `file` level.
**Warning signs:** Comments visible on GitHub PR page but missing from API response.

### Pitfall 2: Thread Resolution Not Available via REST
**What goes wrong:** Assuming `is_resolved` exists on the REST review comments endpoint. It does not.
**Why it happens:** The GitHub UI shows resolved/unresolved threads, leading developers to expect the REST API exposes this.
**How to avoid:** Use a targeted GraphQL query to fetch `reviewThreads { isResolved }` per PR. Fall back gracefully if GraphQL fails.
**Warning signs:** All threads appearing as "unresolved" in the API output.

### Pitfall 3: Outdated Review Detection Requires Head SHA Tracking
**What goes wrong:** Cannot determine if a review is outdated without knowing the PR's current head commit SHA.
**Why it happens:** The current domain model does not persist `HeadSHA`. The `PullRequestReview.CommitID` tells you which commit was reviewed, but you need the current head to compare.
**How to avoid:** Add `HeadSHA` to the PullRequest domain model and persist it. Update it during each poll cycle from `pr.GetHead().GetSHA()`.
**Warning signs:** All reviews appearing as "current" or all as "outdated".

### Pitfall 4: InReplyTo References Root Comment Only
**What goes wrong:** Trying to build deeply nested thread trees from `in_reply_to_id`.
**Why it happens:** Developers assume replies can reference other replies, creating a tree.
**How to avoid:** GitHub only supports one level of threading. `in_reply_to_id` always points to the root comment of a thread. Group by root ID, sort replies by timestamp.
**Warning signs:** Missing comments in thread grouping because they reference intermediate replies.

### Pitfall 5: Null Pointer Panics on go-github Getter Methods
**What goes wrong:** Accessing `.GetX()` on nil pointers when optional fields are absent.
**Why it happens:** All go-github struct fields are pointers. A review comment with no start_line has `StartLine == nil`.
**How to avoid:** Always use the generated `GetXxx()` helper methods (e.g., `comment.GetStartLine()`, `comment.GetSide()`). These return zero values for nil pointers. The existing codebase already follows this pattern in `mapPullRequest`.
**Warning signs:** Nil pointer panic in production on a PR with single-line comments (no StartLine).

### Pitfall 6: Rate Limit Explosion from Per-PR Review Fetching
**What goes wrong:** Fetching reviews + comments for every PR on every poll cycle burns through rate limits.
**Why it happens:** If you have 20 PRs across 5 repos, that's 20 * 3 = 60 additional API calls per poll cycle (reviews + review comments + issue comments per PR).
**How to avoid:** Only fetch reviews/comments for PRs that have changed since last poll (use `UpdatedAt` comparison, already implemented in `pollRepo`). Consider fetching reviews only on demand (when PR detail is requested) rather than during every poll.
**Warning signs:** Rate limit warnings appearing frequently in logs.

## Code Examples

### Mapping go-github PullRequestReview to Domain Model
```go
// Source: go-github PullRequestReview struct fields
// https://github.com/google/go-github/blob/master/github/pulls_reviews.go
func mapReview(r *gh.PullRequestReview, prID int64, botUsernames []string) model.Review {
    login := r.GetUser().GetLogin()
    return model.Review{
        ID:            r.GetID(),
        PRID:          prID,
        ReviewerLogin: login,
        State:         model.ReviewState(strings.ToLower(r.GetState())),
        Body:          r.GetBody(),
        CommitID:      r.GetCommitID(),
        SubmittedAt:   r.GetSubmittedAt().Time,
        IsBot:         isBotUser(login, botUsernames),
    }
}
```

### Mapping go-github PullRequestComment to Domain Model
```go
// Source: go-github PullRequestComment struct fields
// https://github.com/google/go-github/blob/master/github/pulls_comments.go
func mapReviewComment(c *gh.PullRequestComment, prID int64) model.ReviewComment {
    var inReplyTo *int64
    if c.InReplyTo != nil {
        inReplyTo = c.InReplyTo
    }

    return model.ReviewComment{
        ID:          c.GetID(),
        ReviewID:    c.GetPullRequestReviewID(),
        PRID:        prID,
        Author:      c.GetUser().GetLogin(),
        Body:        c.GetBody(),
        Path:        c.GetPath(),
        Line:        c.GetLine(),
        StartLine:   c.GetStartLine(),
        Side:        c.GetSide(),
        DiffHunk:    c.GetDiffHunk(),
        SubjectType: c.GetSubjectType(),
        CommitID:    c.GetCommitID(),
        IsResolved:  false, // Set later from GraphQL data
        IsOutdated:  false, // Set later from CommitID vs HeadSHA comparison
        InReplyToID: inReplyTo,
        CreatedAt:   c.GetCreatedAt().Time,
        UpdatedAt:   c.GetUpdatedAt().Time,
    }
}
```

### Minimal GraphQL Query for Thread Resolution
```go
// Source: GitHub GraphQL API docs + community discussion
// https://github.com/orgs/community/discussions/24854
const threadResolutionQuery = `
query($owner: String!, $repo: String!, $pr: Int!) {
    repository(owner: $owner, name: $repo) {
        pullRequest(number: $pr) {
            reviewThreads(first: 100) {
                nodes {
                    id
                    isResolved
                    comments(first: 1) {
                        nodes {
                            databaseId
                        }
                    }
                }
            }
        }
    }
}
`

// ThreadResolution maps a review comment's database ID to its resolution status.
type ThreadResolution struct {
    CommentID  int64
    IsResolved bool
}
```

### Suggestion Block Extraction
```go
import "regexp"

// Matches ```suggestion ... ``` blocks (with 3+ backticks)
var suggestionRegex = regexp.MustCompile("(?s)`{3,}suggestion[^\n]*\n(.*?)\n`{3,}")

func extractSuggestion(body string) (proposedCode string, hasSuggestion bool) {
    matches := suggestionRegex.FindStringSubmatch(body)
    if len(matches) < 2 {
        return "", false
    }
    return matches[1], true
}
```

### Coderabbit Nitpick Detection
```go
import "strings"

// Heuristic patterns for nitpick detection in comment bodies.
var nitpickPatterns = []string{
    "**nitpick",
    "[nitpick]",
    "(nitpick)",
    "nitpick:",
    "nitpick (non-blocking)",
}

func isNitpickComment(author, body string, botUsernames []string) bool {
    if !isBotUser(author, botUsernames) {
        return false
    }
    lower := strings.ToLower(body)
    for _, pattern := range nitpickPatterns {
        if strings.Contains(lower, pattern) {
            return true
        }
    }
    return false
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `position` field for line targeting | `line`/`start_line`/`side`/`start_side` fields | GitHub API comfort-fade preview (now stable) | Position is deprecated; use line-based fields |
| Single-line comments only | Multi-line comments with `start_line` + `line` range | ~2020 | Comments can span line ranges |
| No `subject_type` field | `subject_type: "line"` or `"file"` | Added post-comfort-fade | Can distinguish file-level from line-level comments |

**Deprecated/outdated:**
- `position` and `original_position` fields on review comments: Deprecated in favor of `line`/`original_line`. Still returned by API but should not be used for new code.

## Key Technical Decisions for Planner

### 1. When to Fetch Reviews: Poll vs On-Demand
**Recommendation: Hybrid.** Fetch review metadata (Review objects with state) during the poll cycle for ALL PRs (needed for STAT-02, STAT-06 flags). Fetch full review comments (inline + issue comments + thread resolution) only when the PR detail endpoint is hit OR when the PR has been updated since last comment fetch. This balances rate limit usage with data freshness.

### 2. GraphQL Thread Resolution: Inline or Separate Call
**Recommendation: Separate, optional call.** Make the GraphQL query a separate method on the GitHub adapter. Call it after fetching review comments. If it fails (token lacks GraphQL scope, rate limited, etc.), log a warning and default all threads to `IsResolved = false`. The existing PAT token should work for both REST and GraphQL.

### 3. Domain Model: HeadSHA as Persisted Field
**Recommendation: Add `HeadSHA string` to PullRequest model.** Persist it in the pull_requests table. Update during each poll cycle from `pr.GetHead().GetSHA()`. This enables outdated review detection without extra API calls.

### 4. Review State Aggregation for PR Status (STAT-02)
**Recommendation: Compute from stored reviews.** The PR's review-derived status is computed by looking at the latest review from each non-bot reviewer. If any reviewer has `changes_requested` as their latest, PR status is "changes requested". If all reviewers approve, "approved". Otherwise "pending". Store as a computed field during poll, not on-demand.

### 5. Migration Strategy
**Recommendation: 4 migrations (reviews, review_comments, issue_comments, bot_config).** Each migration adds one table. The PR table gets an ALTER for `head_sha`. Foreign keys reference pull_requests.

## Open Questions

1. **CodeRabbit nitpick format consistency**
   - What we know: CodeRabbit summary reviews have a "Nitpick comments" section. Individual inline comments may or may not contain "nitpick" text.
   - What's unclear: Whether CodeRabbit's inline comment format is stable across versions, or if it varies by configuration.
   - Recommendation: Use heuristic body-text matching for now. The bot configuration endpoint (REPO-04) allows adding/removing bot usernames, making this extensible. Flag as LOW confidence detection -- false negatives are acceptable, false positives are not.

2. **GraphQL rate limit interaction with REST**
   - What we know: GraphQL and REST share the same 5,000 points/hour budget (for PATs). GraphQL allows 2,000 points/minute vs REST's 900.
   - What's unclear: The exact "point cost" of the reviewThreads query for a PR with many threads.
   - Recommendation: A single reviewThreads query per PR should cost ~1-5 points. Even with 50 PRs, this is negligible. Monitor in logs.

3. **Pagination for review threads in GraphQL**
   - What we know: The query uses `first: 100` for review threads. Most PRs have fewer than 100 threads.
   - What's unclear: Behavior for PRs with 100+ review threads (unlikely but possible).
   - Recommendation: Start with `first: 100`. Add pagination later if needed. Log a warning if `hasNextPage` is true.

## Sources

### Primary (HIGH confidence)
- [go-github pulls_comments.go](https://github.com/google/go-github/blob/master/github/pulls_comments.go) - PullRequestComment struct with all fields verified
- [go-github pulls_reviews.go](https://github.com/google/go-github/blob/master/github/pulls_reviews.go) - PullRequestReview struct with CommitID field verified
- [GitHub REST API - Pull Request Review Comments](https://docs.github.com/en/rest/pulls/comments) - Confirmed subject_type, line fields, NO is_resolved field
- [GitHub REST API - Pull Request Reviews](https://docs.github.com/en/rest/pulls/reviews) - Review state, commit_id, submitted_at fields confirmed
- [GitHub REST API - Issue Comments](https://docs.github.com/en/rest/issues/comments) - General PR comments endpoint confirmed
- [go-github issues_comments.go](https://github.com/google/go-github/blob/master/github/issues_comments.go) - IssueComment struct verified

### Secondary (MEDIUM confidence)
- [GitHub GraphQL reviewThreads](https://github.com/orgs/community/discussions/24854) - Confirmed isResolved available via GraphQL, with query examples
- [GitHub community discussion on thread resolution](https://github.com/orgs/community/discussions/9175) - Confirmed REST API lacks resolution status
- [shurcooL/githubv4 README](https://github.com/shurcooL/githubv4/blob/main/README.md) - GraphQL Go client API reviewed, decided against adding as dependency
- [coderabbit-review-helper](https://github.com/obra/coderabbit-review-helper) - CodeRabbit bot patterns: username `coderabbitai`, section markers, resolution detection regex

### Tertiary (LOW confidence)
- CodeRabbit nitpick detection patterns - Based on third-party tool analysis, not official CodeRabbit documentation. Individual inline comment nitpick markers are heuristic.
- [GitHub GraphQL rate limits](https://docs.github.com/en/graphql/overview/rate-limits-and-query-limits-for-the-graphql-api) - Point costs for specific queries not verified empirically.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries already in use, struct fields verified against source code
- Architecture (REST API): HIGH - Three comment types, field mappings, threading model all verified
- Architecture (GraphQL thread resolution): MEDIUM - Query works per community examples, but not tested against this project's token
- Bot detection (CodeRabbit): MEDIUM - Username and summary patterns confirmed from third-party tool; individual nitpick detection is heuristic
- Suggestion extraction: HIGH - Markdown code fence format is well-documented and stable
- Outdated review detection: HIGH - CommitID field on reviews + HeadSHA on PRs is standard GitHub pattern

**Research date:** 2026-02-12
**Valid until:** 2026-03-12 (stable domain -- GitHub API changes are slow and backward-compatible)
