---
phase: 03-core-api
plan: 01
subsystem: database
tags: [sqlite, migration, domain-model, needs-review, polling]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: SQLite adapter with PRRepo, domain model, port interfaces
  - phase: 02-github-integration
    provides: PollService with IsReviewRequestedFrom logic
provides:
  - NeedsReview persisted field on PullRequest model
  - ListNeedingReview query method on PRStore interface
  - SQLite migration 000002 adding needs_review column with index
  - Poll service sets NeedsReview based on review request status
affects: [03-core-api plan 02 (HTTP API needs-attention endpoint)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Boolean columns as INTEGER 0/1 with Go bool conversion (same as IsDraft)"
    - "Unchanged skip includes NeedsReview comparison for edge case correctness"

key-files:
  created:
    - internal/adapter/driven/sqlite/migrations/000002_add_needs_review.up.sql
    - internal/adapter/driven/sqlite/migrations/000002_add_needs_review.down.sql
  modified:
    - internal/domain/model/pullrequest.go
    - internal/domain/port/driven/prstore.go
    - internal/adapter/driven/sqlite/prrepo.go
    - internal/adapter/driven/sqlite/prrepo_test.go
    - internal/application/pollservice.go
    - internal/application/pollservice_test.go

key-decisions:
  - "NeedsReview placed in persisted fields section (not transient) -- queried from DB by HTTP API"
  - "Unchanged skip now compares both UpdatedAt and NeedsReview to catch reviewer assignment without timestamp change"

patterns-established:
  - "Boolean persistence pattern: Go bool -> int 0/1 for INSERT, int -> bool != 0 for SELECT (matches IsDraft)"

# Metrics
duration: 5min
completed: 2026-02-11
---

# Phase 3 Plan 1: Needs Review Persistence Summary

**Persisted NeedsReview boolean on PullRequest with SQLite migration, ListNeedingReview query, and poll service integration**

## Performance

- **Duration:** 5 min
- **Started:** 2026-02-11T18:33:15Z
- **Completed:** 2026-02-11T18:38:23Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- Added NeedsReview bool to PullRequest domain model as a persisted field
- Added ListNeedingReview to PRStore port interface and SQLite adapter implementation
- Created migration 000002 with needs_review column and index
- Poll service now sets NeedsReview = true for review-requested PRs, false for authored-only PRs
- Updated unchanged skip to compare NeedsReview for edge case correctness

## Task Commits

Each task was committed atomically:

1. **Task 1: Add needs_review to domain model, port interface, and SQLite migration** - `c1df4a9` (feat)
2. **Task 2: Update SQLite adapter and poll service to persist needs_review** - `c07922b` (feat)

## Files Created/Modified
- `internal/domain/model/pullrequest.go` - Added NeedsReview bool field to PullRequest struct
- `internal/domain/port/driven/prstore.go` - Added ListNeedingReview method to PRStore interface
- `internal/adapter/driven/sqlite/migrations/000002_add_needs_review.up.sql` - ALTER TABLE adds needs_review column with index
- `internal/adapter/driven/sqlite/migrations/000002_add_needs_review.down.sql` - Rollback drops index and column
- `internal/adapter/driven/sqlite/prrepo.go` - Updated Upsert, scanPR, all SELECT queries, added ListNeedingReview
- `internal/adapter/driven/sqlite/prrepo_test.go` - Added TestPRRepo_ListNeedingReview test
- `internal/application/pollservice.go` - Set pr.NeedsReview in pollRepo, updated unchanged skip
- `internal/application/pollservice_test.go` - Added NeedsReview assertions, added ListNeedingReview to mock

## Decisions Made
- NeedsReview placed in persisted fields section of PullRequest struct (after BaseBranch, before Labels) -- this field is queried from DB by the HTTP API, unlike the transient RequestedReviewers/RequestedTeamSlugs fields
- Updated the unchanged skip comparison to include NeedsReview alongside UpdatedAt -- handles edge case where a reviewer is added but GitHub's UpdatedAt timestamp does not change

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- NeedsReview field is persisted and queryable via ListNeedingReview
- Plan 02 (HTTP API) can now serve the "needs attention" endpoint from database queries
- All existing tests pass with no regressions (34 total tests)

---
*Phase: 03-core-api*
*Completed: 2026-02-11*
