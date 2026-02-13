---
phase: 04-review-intelligence
plan: 03
subsystem: api
tags: [enrichment, threading, suggestions, bot-detection, review-aggregation, hexagonal]

# Dependency graph
requires:
  - phase: 04-01
    provides: "Review, ReviewComment, IssueComment domain entities; ReviewStore, BotConfigStore ports; ReviewState enum"
  - phase: 04-02
    provides: "FetchReviews, FetchReviewComments, FetchIssueComments, FetchThreadResolution real implementations; HeadSHA on PullRequest"
provides:
  - "ReviewService with GetPRReviewSummary for complete enriched PR review state"
  - "CommentThread, Suggestion, PRReviewSummary exported types for HTTP handler"
  - "PollService fetches and stores reviews, review comments, issue comments, and thread resolution for changed PRs"
  - "Bot detection, nitpick detection, outdated review detection, review status aggregation"
affects: [04-04-PLAN]

# Tech tracking
tech-stack:
  added: []
  patterns: [pure-function-enrichment, partial-failure-tolerance, rate-limit-gated-fetch]

key-files:
  created:
    - internal/application/reviewservice.go
    - internal/application/reviewservice_test.go
  modified:
    - internal/application/pollservice.go
    - internal/application/pollservice_test.go
    - cmd/reviewhub/main.go

key-decisions:
  - "Enrichment helpers are unexported package-level functions (not methods) for direct testability within same package"
  - "fetchReviewData calls each fetch step independently -- partial failures are logged but do not abort the poll"
  - "Review data fetching gated on PR update detection (unchanged PRs skip review fetch to preserve rate limits)"
  - "GetByNumber used after Upsert to retrieve stored PR ID (avoids changing PRStore interface)"
  - "CodeRabbit awaiting detection compares review CommitID against headSHA per-review"

patterns-established:
  - "Pure enrichment functions: all helpers (groupIntoThreads, extractSuggestions, aggregateReviewStatus) are pure with no I/O"
  - "Partial failure tolerance: each independent fetch step logs errors and continues rather than aborting"
  - "Rate-limit gating: review data only fetched for PRs that actually changed (UpdatedAt or NeedsReview differs)"

# Metrics
duration: 5min
completed: 2026-02-13
---

# Phase 4 Plan 3: Review Enrichment Service and Poll Integration Summary

**ReviewService with threading, suggestion extraction, bot/outdated detection, and review status aggregation; PollService extended to fetch and store review data for changed PRs**

## Performance

- **Duration:** 5 min
- **Started:** 2026-02-13T05:13:45Z
- **Completed:** 2026-02-13T05:19:42Z
- **Tasks:** 2
- **Files modified:** 5 (2 created, 3 modified)

## Accomplishments
- Created ReviewService with GetPRReviewSummary that assembles complete enriched view: threads, suggestions, bot flags, outdated detection, review status aggregation, resolved/unresolved counts
- Implemented 8 pure enrichment helper functions: groupIntoThreads (with orphan handling), extractSuggestions (regex-based), isBotUser, isNitpickComment, isReviewOutdated, aggregateReviewStatus (latest-per-reviewer), computeBotFlags, isCoderabbitUser
- Extended PollService with fetchReviewData method that fetches reviews, review comments, issue comments, and thread resolution with independent partial-failure tolerance
- Added 18 new test cases (16 ReviewService + 2 PollService) covering all enrichment paths and edge cases

## Task Commits

Each task was committed atomically:

1. **Task 1: Create ReviewService with enrichment logic** - `6d931a4` (feat)
2. **Task 2: Extend PollService to fetch and store reviews/comments** - `c799820` (feat)

## Files Created/Modified
- `internal/application/reviewservice.go` - ReviewService with GetPRReviewSummary, CommentThread/Suggestion/PRReviewSummary types, all enrichment helper functions
- `internal/application/reviewservice_test.go` - 16 tests covering threading, suggestions, bot detection, nitpick detection, outdated detection, review status aggregation, full integration
- `internal/application/pollservice.go` - Added ReviewStore/BotConfigStore deps, fetchReviewData method, review data fetch after PR upsert
- `internal/application/pollservice_test.go` - Updated mocks for new constructor params, added 2 new tests for review data fetching and skip behavior
- `cmd/reviewhub/main.go` - Wired ReviewRepo and BotConfigRepo into PollService constructor

## Decisions Made
- Enrichment helpers are unexported package-level functions (not methods on ReviewService) for direct testability within the same package
- fetchReviewData uses independent fetch steps with logged-and-continue on failure -- partial data is better than none
- Review data fetching is gated on PR update detection to preserve GitHub API rate limits
- Used GetByNumber after Upsert to retrieve stored PR ID rather than changing the PRStore.Upsert interface to return IDs
- CodeRabbit awaiting detection checks CommitID per-review against headSHA

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- ReviewService.GetPRReviewSummary is ready for HTTP handler integration (Plan 04-04)
- CommentThread, Suggestion, PRReviewSummary types are exported for HTTP response building
- PollService now fetches and persists all review data automatically during poll cycles
- All enrichment is pure application-layer logic with no adapter dependencies
- Composition root already wires ReviewRepo and BotConfigRepo (Plan 04-04 only needs to wire ReviewService and update HTTP handler)

## Self-Check: PASSED

All 5 files verified present. Both commit hashes (6d931a4, c799820) confirmed in git log.

---
*Phase: 04-review-intelligence*
*Completed: 2026-02-13*
