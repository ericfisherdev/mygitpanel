---
phase: 04-review-intelligence
plan: 01
subsystem: database
tags: [sqlite, migrations, domain-model, ports-adapters, hexagonal]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: "SQLite dual reader/writer DB, migration framework, domain model base types"
  - phase: 02-github-integration
    provides: "GitHubClient port interface with FetchReviews/FetchReviewComments stubs"
provides:
  - "Review, ReviewComment, IssueComment, BotConfig domain entities"
  - "ReviewStore port interface (8 methods) for review/comment persistence"
  - "BotConfigStore port interface (4 methods) for bot username management"
  - "GitHubClient expanded with FetchIssueComments and FetchThreadResolution"
  - "PullRequest.HeadSHA field for outdated review detection"
  - "5 SQLite migrations (reviews, review_comments, issue_comments, bot_config tables; head_sha column)"
  - "ReviewRepo and BotConfigRepo SQLite adapters with tests"
affects: [04-02-PLAN, 04-03-PLAN, 04-04-PLAN]

# Tech tracking
tech-stack:
  added: []
  patterns: [upsert-with-nullable-fields, boolean-int-mapping, scan-helper-per-entity]

key-files:
  created:
    - internal/domain/model/issuecomment.go
    - internal/domain/model/botconfig.go
    - internal/domain/port/driven/reviewstore.go
    - internal/domain/port/driven/botconfigstore.go
    - internal/adapter/driven/sqlite/reviewrepo.go
    - internal/adapter/driven/sqlite/reviewrepo_test.go
    - internal/adapter/driven/sqlite/botconfigrepo.go
    - internal/adapter/driven/sqlite/botconfigrepo_test.go
    - internal/adapter/driven/sqlite/migrations/000003_add_reviews.up.sql
    - internal/adapter/driven/sqlite/migrations/000004_add_review_comments.up.sql
    - internal/adapter/driven/sqlite/migrations/000005_add_issue_comments.up.sql
    - internal/adapter/driven/sqlite/migrations/000006_add_bot_config.up.sql
    - internal/adapter/driven/sqlite/migrations/000007_add_head_sha.up.sql
  modified:
    - internal/domain/model/review.go
    - internal/domain/model/reviewcomment.go
    - internal/domain/model/enums.go
    - internal/domain/model/pullrequest.go
    - internal/domain/port/driven/githubclient.go
    - internal/adapter/driven/github/client.go
    - internal/application/pollservice_test.go

key-decisions:
  - "CommentType enum added for inline/general/file distinction"
  - "bot_config table seeded with 3 defaults: coderabbitai, github-actions[bot], copilot[bot]"
  - "reviews table uses GitHub review ID as PK (not autoincrement) for idempotent upsert"
  - "in_reply_to_id is nullable INTEGER mapped via sql.NullInt64 for thread chaining"

patterns-established:
  - "scanReview/scanReviewComment/scanIssueComment helpers follow same scanner interface pattern as prrepo.go"
  - "Nullable FK fields (in_reply_to_id) use sql.NullInt64 for scan, any(nil) for insert"
  - "DeleteReviewsByPR runs 3 separate DELETE statements (reviews, review_comments, issue_comments)"

# Metrics
duration: 5min
completed: 2026-02-13
---

# Phase 4 Plan 1: Domain Model Expansion Summary

**Review, ReviewComment, IssueComment, and BotConfig domain entities with ReviewStore/BotConfigStore ports, 5 SQLite migrations, and adapter implementations with 17 new test cases**

## Performance

- **Duration:** 5 min
- **Started:** 2026-02-13T04:59:46Z
- **Completed:** 2026-02-13T05:04:56Z
- **Tasks:** 2
- **Files modified:** 25 (11 modified, 14 created)

## Accomplishments
- Expanded domain model with 4 entity types and HeadSHA on PullRequest for outdated review detection
- Created ReviewStore (8 methods) and BotConfigStore (4 methods) port interfaces following ISP
- Added 5 migration pairs (10 SQL files) creating 4 tables and 1 column with proper foreign keys and indexes
- Implemented ReviewRepo and BotConfigRepo SQLite adapters passing 17 new test cases (32 total in sqlite package)

## Task Commits

Each task was committed atomically:

1. **Task 1: Expand domain models, enums, and port interfaces** - `14265e3` (feat)
2. **Task 2: Create SQLite migrations and adapter implementations with tests** - `e489ac7` (feat)

## Files Created/Modified
- `internal/domain/model/review.go` - Added CommitID field for outdated detection
- `internal/domain/model/reviewcomment.go` - Added StartLine, SubjectType, CommitID fields
- `internal/domain/model/issuecomment.go` - New IssueComment entity for PR-level general comments
- `internal/domain/model/botconfig.go` - New BotConfig entity for configurable bot usernames
- `internal/domain/model/enums.go` - Added CommentType enum (inline, general, file)
- `internal/domain/model/pullrequest.go` - Added HeadSHA persisted field
- `internal/domain/port/driven/githubclient.go` - Added FetchIssueComments and FetchThreadResolution methods
- `internal/domain/port/driven/reviewstore.go` - New ReviewStore port interface (8 methods)
- `internal/domain/port/driven/botconfigstore.go` - New BotConfigStore port interface (4 methods)
- `internal/adapter/driven/github/client.go` - Added stub implementations for new interface methods
- `internal/application/pollservice_test.go` - Updated mock with new interface methods
- `internal/adapter/driven/sqlite/migrations/000003-000007` - 5 up/down migration pairs
- `internal/adapter/driven/sqlite/reviewrepo.go` - ReviewStore implementation with CRUD and scan helpers
- `internal/adapter/driven/sqlite/reviewrepo_test.go` - 6 test cases covering upsert, get, delete, resolution, idempotency
- `internal/adapter/driven/sqlite/botconfigrepo.go` - BotConfigStore implementation with seeded defaults
- `internal/adapter/driven/sqlite/botconfigrepo_test.go` - 6 test cases covering add, remove, duplicates, defaults, usernames

## Decisions Made
- GitHub review ID used as primary key (not autoincrement) for idempotent upsert matching the reviews table in prrepo.go
- bot_config table seeded with 3 default bot usernames during migration (coderabbitai, github-actions[bot], copilot[bot])
- CommentType enum added to enums.go for future use distinguishing inline, general, and file-level comments
- in_reply_to_id uses nullable INTEGER with sql.NullInt64 for scan/insert to properly represent NULL vs 0

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added stub methods to GitHub adapter and mock for new interface methods**
- **Found during:** Task 1 (port interface expansion)
- **Issue:** Adding FetchIssueComments and FetchThreadResolution to GitHubClient interface broke compilation of the concrete GitHub adapter and the mock in pollservice_test.go
- **Fix:** Added stub implementations (return nil, nil) to github/client.go and the mockGitHubClient in pollservice_test.go
- **Files modified:** internal/adapter/driven/github/client.go, internal/application/pollservice_test.go
- **Verification:** `go build ./...` compiles, all existing tests pass
- **Committed in:** 14265e3 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Auto-fix was necessary to maintain compilation after interface expansion. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All domain entities and port interfaces ready for Plan 04-02 (GitHub adapter: FetchReviews, FetchReviewComments, FetchIssueComments, HeadSHA mapping)
- ReviewStore and BotConfigStore adapters ready for Plan 04-03 (review intelligence service)
- PRRepo.Upsert and scanPR need head_sha column integration (Plan 04-02 scope)
- FetchReviews/FetchReviewComments/FetchIssueComments stubs need real implementations (Plan 04-02 scope)

## Self-Check: PASSED

All 25 files verified present. Both commit hashes (14265e3, e489ac7) confirmed in git log.

---
*Phase: 04-review-intelligence*
*Completed: 2026-02-13*
