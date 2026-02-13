# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-10)

**Core value:** Review comments formatted with enough code context that an AI agent can understand and fix the code
**Current focus:** Phase 5 complete - PR Health Signals; Phase 6 next

## Current Position

Phase: 5 of 6 (PR Health Signals)
Plan: 3 of 3 in current phase
Status: Phase complete
Last activity: 2026-02-13 - Completed 05-03-PLAN.md

Progress: [████████████████░] ~93%

## Performance Metrics

**Velocity:**

- Total plans completed: 14
- Average duration: 6min
- Total execution time: 1.35 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01-foundation | 3/3 | 21min | 7min |
| 02-github-integration | 2/2 | 16min | 8min |
| 03-core-api | 2/2 | 12min | 6min |
| 04-review-intelligence | 4/4 | 19min | 4.8min |
| 05-pr-health-signals | 3/3 | 14min | 4.7min |

**Recent Trend:**

- Last 5 plans: 04-04 (5min), 05-01 (5min), 05-02 (6min), 05-03 (3min)
- Trend: stable to improving

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [Roadmap]: 6 phases derived from 49 requirements following hexagonal dependency rule (innermost first)
- [Roadmap]: Phase 4 (Review Intelligence) isolated as dedicated phase -- highest complexity, core differentiator
- [Roadmap]: Phases 4 and 5 are independent (both depend on Phase 3) but sequenced 4-before-5 to prioritize differentiator
- [01-01]: Domain model entities are pure Go structs with zero external dependencies (stdlib time only)
- [01-01]: Port interfaces use only context.Context and domain model types
- [01-01]: Config uses os.LookupEnv for fail-fast on missing required env vars
- [01-01]: testify v1.11.1 chosen for test assertions
- [01-02]: Pure Go SQLite via modernc.org/sqlite -- zero CGO, cross-platform
- [01-02]: Dual reader/writer pattern: writer MaxOpenConns(1), reader MaxOpenConns(4)
- [01-02]: WAL mode and pragmas set via DSN parameters (not PRAGMA statements)
- [01-02]: ON CONFLICT upsert to preserve auto-increment IDs
- [01-02]: Labels stored as JSON text column, not separate join table
- [01-03]: run() pattern separates exit code handling from application logic -- defers execute on all paths
- [01-03]: signal.NotifyContext for shutdown propagation instead of manual signal channel
- [01-03]: log/slog for structured logging -- token never logged, username logged for debugging
- [01-03]: 10s shutdown timeout pre-wired for future HTTP server drain
- [02-01]: Transport stack order: httpcache -> rate-limit -> go-github for ETag caching before rate limit middleware
- [02-01]: NewClientWithHTTPClient exported for test injection of httptest servers
- [02-01]: FetchReviews and FetchReviewComments stubbed as nil/nil returns for Phase 4
- [02-02]: RequestedReviewers/RequestedTeamSlugs as transient model fields -- populated during fetch, not persisted
- [02-02]: Channel-based refresh pattern for safe coordination with polling goroutine
- [02-02]: IsReviewRequestedFrom exported for external test access and future HTTP handler use
- [03-01]: NeedsReview placed in persisted fields section (not transient) -- queried from DB by HTTP API
- [03-01]: Unchanged skip now compares both UpdatedAt and NeedsReview to catch reviewer assignment without timestamp change
- [03-02]: Package named httphandler to avoid collision with stdlib http
- [03-02]: json.Marshal to bytes before WriteHeader to handle marshal errors before committing status
- [03-02]: Empty arrays (not null) for labels, reviews, comments in JSON responses
- [03-02]: Fire-and-forget goroutine with context.Background() for async refresh on AddRepo
- [03-02]: Recovery middleware innermost, logging outermost for panic-safe request logging
- [04-01]: GitHub review ID used as primary key (not autoincrement) for idempotent upsert on reviews table
- [04-01]: bot_config table seeded with 3 defaults: coderabbitai, github-actions[bot], copilot[bot]
- [04-01]: CommentType enum added for inline/general/file distinction
- [04-01]: in_reply_to_id uses nullable INTEGER with sql.NullInt64 for proper NULL handling
- [04-02]: GraphQL used only for thread resolution (isResolved) -- REST API does not expose this field
- [04-02]: All GraphQL error paths return empty map, never propagate errors -- supplementary data source
- [04-02]: NewClientWithHTTPClient gains token as 4th parameter; derives graphqlURL from baseURL for testability
- [04-02]: PRID=0 convention: adapter sets PRID to 0, caller assigns actual DB ID before persisting
- [04-03]: Enrichment helpers are unexported package-level functions (not methods) for direct testability within same package
- [04-03]: fetchReviewData calls each fetch step independently -- partial failures logged but do not abort poll
- [04-03]: Review data fetching gated on PR update detection (unchanged PRs skip review fetch for rate limits)
- [04-03]: GetByNumber after Upsert to retrieve stored PR ID (avoids changing PRStore interface)
- [04-03]: CodeRabbit awaiting detection compares review CommitID against headSHA per-review
- [04-04]: Review enrichment failure in GetPR is non-fatal -- returns basic PRResponse with empty enriched fields
- [04-04]: List endpoints skip enrichment by design -- lightweight responses without ReviewService overhead
- [04-04]: IsNitpickComment exported from application package for HTTP handler nitpick detection
- [04-04]: Bot config endpoints reuse UNIQUE constraint error detection pattern from repo endpoints
- [05-01]: Full replacement strategy for check runs (DELETE + INSERT in tx) rather than per-run upsert
- [05-01]: Empty MergeableStatus/CIStatus default to "unknown" in PRRepo.Upsert
- [05-01]: GitHub adapter and test mock get stub implementations for new GitHubClient methods to keep build green
- [05-02]: Both "canceled" and "cancelled" matched in CI status switch for GitHub API vs linter compatibility
- [05-02]: mapCombinedStatus returns nil when zero statuses and empty state (no CI configured)
- [05-02]: fetchHealthData returns early if FetchCheckRuns fails; continues independently on other failures
- [05-02]: Health data upserts run after review data upserts in poll cycle
- [05-03]: Health enrichment failure in GetPR is non-fatal -- same pattern as review enrichment
- [05-03]: CIStatus on detail endpoint overwritten by HealthService computation (more accurate than stored value)
- [05-03]: List endpoints show health fields from PR model only -- no HealthService call for lightweight responses

### Pending Todos

None.

### Blockers/Concerns

- [Research]: Phase 4 needs verification of go-github struct fields for review comments before planning (RESOLVED: verified in 04-02)
- [Research]: Thread resolution status (is_resolved) may require GraphQL -- REST availability unconfirmed (RESOLVED: confirmed GraphQL required, implemented in 04-02)

## Session Continuity

Last session: 2026-02-13T22:33:45Z
Stopped at: Completed 05-03-PLAN.md (Phase 5 complete)
Resume file: .planning/phases/06-docker-deployment/ (Phase 6 planning)
