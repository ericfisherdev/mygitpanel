---
phase: 05-pr-health-signals
plan: 01
subsystem: database, domain
tags: [sqlite, migrations, hexagonal, health-signals, check-runs, ci-status]

requires:
  - phase: 01-foundation
    provides: domain model, SQLite adapter with PRRepo, migration framework
  - phase: 03-core-api
    provides: PRStore port interface, HTTP handlers consuming PR data
provides:
  - CheckRun, CombinedStatus, CommitStatus, PRDetail domain entities
  - MergeableStatus enum (mergeable, conflicted, unknown)
  - PullRequest health signal fields (Additions, Deletions, ChangedFiles, MergeableStatus, CIStatus)
  - CheckStore port interface with ReplaceCheckRunsForPR and GetCheckRunsByPR
  - GitHubClient port with FetchCheckRuns, FetchCombinedStatus, FetchPRDetail, FetchRequiredStatusChecks
  - SQLite migrations 000008 (health signal columns) and 000009 (check_runs table)
  - CheckRepo SQLite adapter with transactional full-replacement strategy
affects: [05-02-PLAN, 05-03-PLAN]

tech-stack:
  added: []
  patterns:
    - "Full replacement strategy for check runs (DELETE + INSERT in transaction)"
    - "Empty-to-unknown default for MergeableStatus and CIStatus on upsert"
    - "Nullable datetime columns scanned via sql.NullString for started_at/completed_at"

key-files:
  created:
    - internal/domain/port/driven/checkstore.go
    - internal/adapter/driven/sqlite/checkrepo.go
    - internal/adapter/driven/sqlite/checkrepo_test.go
    - internal/adapter/driven/sqlite/migrations/000008_add_health_signals.up.sql
    - internal/adapter/driven/sqlite/migrations/000008_add_health_signals.down.sql
    - internal/adapter/driven/sqlite/migrations/000009_add_check_runs.up.sql
    - internal/adapter/driven/sqlite/migrations/000009_add_check_runs.down.sql
  modified:
    - internal/domain/model/checkstatus.go
    - internal/domain/model/enums.go
    - internal/domain/model/pullrequest.go
    - internal/domain/port/driven/githubclient.go
    - internal/adapter/driven/sqlite/prrepo.go
    - internal/adapter/driven/sqlite/prrepo_test.go
    - internal/adapter/driven/github/client.go
    - internal/application/pollservice_test.go

key-decisions:
  - "Full replacement strategy for check runs (DELETE + INSERT in tx) rather than per-run upsert -- simpler and handles stale cleanup"
  - "Empty MergeableStatus/CIStatus default to 'unknown' in Upsert to avoid empty string in DB"
  - "GitHub adapter and test mock get stub implementations for new GitHubClient methods to keep build green"

patterns-established:
  - "CheckRepo full replacement: ReplaceCheckRunsForPR wraps DELETE + INSERT in a transaction"
  - "Nullable datetime scan: sql.NullString for optional started_at/completed_at columns"

duration: 5min
completed: 2026-02-13
---

# Phase 5 Plan 1: Domain & Persistence Foundation Summary

**Health signal domain entities (CheckRun, CombinedStatus, CommitStatus, PRDetail, MergeableStatus enum), expanded PullRequest with diff stats/CI/mergeable fields, CheckStore port, and SQLite migrations with CheckRepo adapter**

## Performance

- **Duration:** 5 min
- **Started:** 2026-02-13T22:13:22Z
- **Completed:** 2026-02-13T22:18:51Z
- **Tasks:** 2
- **Files modified:** 15

## Accomplishments

- Domain model expanded with CheckRun, CombinedStatus, CommitStatus, PRDetail entities and MergeableStatus enum -- all pure Go structs with zero external dependencies
- PullRequest gains 5 persisted health signal fields (Additions, Deletions, ChangedFiles, MergeableStatus, CIStatus) with SQLite migration 000008
- CheckStore port and CheckRepo adapter implement transactional full-replacement strategy for check runs with SQLite migration 000009
- GitHubClient port expanded with 4 new methods for health data fetching (stubs in adapter until Plan 02)

## Task Commits

Each task was committed atomically:

1. **Task 1: Domain model expansion and port interfaces** - `ba4e47b` (feat)
2. **Task 2: SQLite migrations and adapter implementations** - `16cc8bb` (feat)

## Files Created/Modified

- `internal/domain/model/checkstatus.go` - CheckRun, CombinedStatus, CommitStatus, PRDetail domain entities (replaced Phase 1 placeholder)
- `internal/domain/model/enums.go` - MergeableStatus enum (mergeable, conflicted, unknown)
- `internal/domain/model/pullrequest.go` - 5 new health signal fields on PullRequest struct
- `internal/domain/port/driven/githubclient.go` - 4 new methods on GitHubClient interface
- `internal/domain/port/driven/checkstore.go` - CheckStore port interface with 2 methods
- `internal/adapter/driven/sqlite/migrations/000008_add_health_signals.up.sql` - ALTER TABLE for 5 new columns
- `internal/adapter/driven/sqlite/migrations/000008_add_health_signals.down.sql` - DROP COLUMN rollback
- `internal/adapter/driven/sqlite/migrations/000009_add_check_runs.up.sql` - CREATE TABLE check_runs with index
- `internal/adapter/driven/sqlite/migrations/000009_add_check_runs.down.sql` - DROP TABLE rollback
- `internal/adapter/driven/sqlite/prrepo.go` - Upsert, scanPR, and all SELECT queries updated for 5 new columns
- `internal/adapter/driven/sqlite/prrepo_test.go` - Health signal round-trip and defaults tests
- `internal/adapter/driven/sqlite/checkrepo.go` - CheckRepo with transactional ReplaceCheckRunsForPR and GetCheckRunsByPR
- `internal/adapter/driven/sqlite/checkrepo_test.go` - Replace, get, empty, and replace-with-empty tests
- `internal/adapter/driven/github/client.go` - Stub implementations for 4 new GitHubClient methods
- `internal/application/pollservice_test.go` - Stub implementations for mock GitHubClient

## Decisions Made

- Full replacement strategy for check runs (DELETE + INSERT in transaction) rather than per-run upsert -- simpler and handles stale check cleanup automatically
- Empty MergeableStatus/CIStatus fields default to "unknown" in PRRepo.Upsert to prevent empty string values in the database
- GitHub adapter and test mock receive stub (nil, nil) implementations for the 4 new GitHubClient methods to keep the full project building green during incremental development

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added stub implementations to GitHub adapter and test mock**
- **Found during:** Task 1 (Domain model expansion)
- **Issue:** Adding 4 new methods to GitHubClient interface broke the compile-time check in the GitHub adapter and the mock in pollservice_test.go. Pre-commit hook runs `go vet` across the full project, blocking commit.
- **Fix:** Added stub (nil, nil) implementations for FetchCheckRuns, FetchCombinedStatus, FetchPRDetail, FetchRequiredStatusChecks to both the GitHub adapter client and the test mock. These stubs will be replaced with real implementations in Plan 02.
- **Files modified:** internal/adapter/driven/github/client.go, internal/application/pollservice_test.go
- **Verification:** `go build ./...` and `go vet ./...` pass cleanly
- **Committed in:** ba4e47b (Task 1 commit)

**2. [Rule 1 - Bug] Fixed misspelling in comment flagged by golangci-lint**
- **Found during:** Task 1 (Domain model expansion)
- **Issue:** Comment on CheckRun.Conclusion used "cancelled" (British English), flagged by misspell linter
- **Fix:** Changed to "canceled" (American English) to match linter configuration
- **Files modified:** internal/domain/model/checkstatus.go
- **Verification:** golangci-lint passes with 0 issues
- **Committed in:** ba4e47b (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug)
**Impact on plan:** Both auto-fixes necessary for build correctness. No scope creep.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Domain models, ports, and persistence layer ready for Plan 02 (GitHub adapter implementation of health signal fetching)
- Plan 02 will replace the stub implementations in the GitHub adapter with real API calls
- Plan 03 will build the health computation service and HTTP endpoints on top of these foundations

## Self-Check: PASSED

- All 8 created files exist on disk
- Both task commits (ba4e47b, 16cc8bb) verified in git log
- All 37 SQLite adapter tests pass
- Domain layer compiles with zero non-stdlib imports
- go vet passes across all internal packages

---
*Phase: 05-pr-health-signals*
*Completed: 2026-02-13*
