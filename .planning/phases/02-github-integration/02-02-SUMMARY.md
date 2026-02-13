---
phase: 02-github-integration
plan: 02
subsystem: application
tags: [polling, pr-discovery, filtering, deduplication, composition-root]

requires:
  - phase: 01-foundation
    provides: domain model, port interfaces, SQLite adapters, config, main entrypoint
  - phase: 02-01
    provides: GitHub client adapter with FetchPullRequests, transport stack
provides:
  - PollService with Start/RefreshRepo/RefreshPR for periodic PR discovery
  - PR filtering by author and review request (individual + team)
  - Deduplication via single-pass iteration + upsert
  - Unchanged PR skip via UpdatedAt comparison
  - Stale PR cleanup for PRs no longer in open list
  - REVIEWHUB_GITHUB_TEAMS config for team-based review detection
  - Fully wired composition root with polling goroutine
affects: [03-01, 03-02]

tech-stack:
  added: []
  patterns: [application-service, channel-based-refresh, goroutine-lifecycle]

key-files:
  created:
    - internal/application/pollservice.go
    - internal/application/pollservice_test.go
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
    - internal/domain/model/pullrequest.go
    - internal/adapter/driven/github/client.go
    - cmd/reviewhub/main.go

key-decisions:
  - "RequestedReviewers/RequestedTeamSlugs as transient fields on model.PullRequest -- populated during fetch, not persisted to SQLite"
  - "IsReviewRequestedFrom exported for external test access and future use by driving adapters"
  - "Channel-based refresh pattern: RefreshRepo/RefreshPR send requests via buffered channel, processed in Start select loop"

patterns-established:
  - "Application service pattern: PollService depends only on port interfaces, not concrete adapters"
  - "Channel-based manual trigger: refreshRequest struct with done channel for synchronous completion signaling"
  - "Single-pass PR iteration: one loop over fetched PRs handles author filter, reviewer filter, deduplication, and unchanged skip"

duration: 8min
completed: 2026-02-11
---

# Phase 2 Plan 2: Poll Service & Composition Root Summary

**Polling engine with PR discovery filtering by author/reviewer/team, UpdatedAt skip, stale cleanup, and fully wired composition root**

## Performance

- **Duration:** 8 min
- **Started:** 2026-02-11T15:49:40Z
- **Completed:** 2026-02-11T15:57:37Z
- **Tasks:** 3
- **Files modified:** 7

## Accomplishments
- PollService orchestrates periodic PR discovery with configurable interval
- PRs filtered by author match OR review request (individual user + team slugs)
- Deduplication inherent via single-pass iteration + upsert (no duplicate writes)
- Unchanged PRs skipped when UpdatedAt matches stored value
- Stale open PRs cleaned up when no longer returned by GitHub
- Manual refresh via RefreshRepo/RefreshPR bypasses polling interval
- Composition root fully wired: GitHub client, poll service, graceful shutdown

## Task Commits

Each task was committed atomically:

1. **Task 1: Add teams config and create poll service** - `7fbcce2` (feat)
2. **Task 2: Wire poll service into composition root** - `5b3086c` (feat)
3. **Task 3: Full phase verification** - verification only, no commit needed

## Files Created/Modified
- `internal/application/pollservice.go` - Polling engine with Start/RefreshRepo/RefreshPR, PR discovery and filtering logic
- `internal/application/pollservice_test.go` - 8 test functions (13 test cases including subtests) covering all filtering/dedup/cleanup scenarios
- `internal/config/config.go` - Added GitHubTeams field, REVIEWHUB_GITHUB_TEAMS env var parsing
- `internal/config/config_test.go` - Added 2 tests for teams config (with values, empty)
- `internal/domain/model/pullrequest.go` - Added RequestedReviewers/RequestedTeamSlugs transient fields
- `internal/adapter/driven/github/client.go` - Map RequestedReviewers and RequestedTeams from go-github response
- `cmd/reviewhub/main.go` - Wire GitHub client and PollService, remove placeholder references

## Decisions Made
- **Transient model fields:** Added RequestedReviewers/RequestedTeamSlugs to model.PullRequest as transient fields (populated during GitHub fetch, not persisted to SQLite). This keeps the port interface clean while enabling reviewer filtering without a separate return type.
- **Exported IsReviewRequestedFrom:** Made the helper exported for external test package access and potential future use by driving adapters (HTTP handlers).
- **Channel-based refresh:** RefreshRepo/RefreshPR use a channel + done-signal pattern to safely coordinate with the polling goroutine's select loop, avoiding concurrent pollRepo calls.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Phase 2 complete: GitHub adapter + poll service fully operational
- 41 total tests passing across all packages
- Hexagonal dependency rule verified: zero go-github imports in domain layer
- Ready for Phase 3 (HTTP API endpoints) which will expose PR data and trigger manual refreshes
- RefreshRepo/RefreshPR methods ready for HTTP handler wiring

---
*Phase: 02-github-integration*
*Completed: 2026-02-11*
