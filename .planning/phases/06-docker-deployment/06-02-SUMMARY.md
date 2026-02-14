---
phase: 06-docker-deployment
plan: 02
subsystem: application
tags: [adaptive-polling, scheduling, rate-limiting, slog]

requires:
  - phase: 02-github-integration
    provides: "PollService with fixed-interval ticker and GitHub API adapter"
  - phase: 05-pr-health-signals
    provides: "PR health data fetching integrated into poll cycle"
provides:
  - "Per-repo adaptive polling based on PR activity tiers (Hot/Active/Warm/Stale)"
  - "ActivityTier type and classification functions"
  - "Schedules() accessor for observability"
affects: []

tech-stack:
  added: []
  patterns:
    - "Adaptive scheduling with activity tier classification"
    - "RWMutex-protected shared state for schedule map"
    - "1-minute resolution ticker with per-repo polling gates"

key-files:
  created:
    - "internal/application/adaptive.go"
    - "internal/application/adaptive_test.go"
  modified:
    - "internal/application/pollservice.go"
    - "internal/application/pollservice_test.go"

key-decisions:
  - "Named interval constants (intervalHot, intervalActive, etc.) instead of magic numbers to satisfy linter"
  - "RWMutex on schedules map for thread-safe Schedules() accessor from external goroutines"
  - "repoSchedule struct deferred from Task 1 to Task 2 to avoid unused-type linter error"
  - "ScheduleInfo exported type for observability -- separate from internal repoSchedule"

patterns-established:
  - "Activity tier pattern: classifyActivity + freshestActivity for time-based classification"
  - "Adaptive loop pattern: 1-minute ticker + per-repo nextPollAt gates"

duration: 5min
completed: 2026-02-14
---

# Phase 6 Plan 2: Adaptive Polling Summary

**Per-repo adaptive polling with 4 activity tiers replacing fixed-interval ticker -- Hot repos (2m), Active (5m), Warm (15m), Stale (30m)**

## Performance

- **Duration:** 5 min
- **Started:** 2026-02-14T20:46:45Z
- **Completed:** 2026-02-14T20:51:40Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- ActivityTier type with 4 tiers, each mapped to a polling interval via named constants
- PollService.Start now uses 1-minute resolution ticker with per-repo adaptive scheduling
- Manual refresh recalculates tier from fresh activity data
- Tier assignments logged via slog for observability
- All 30+ existing tests continue passing with race detection enabled

## Task Commits

Each task was committed atomically:

1. **Task 1: Activity tier classification and schedule types** - `1add094` (feat)
2. **Task 2: Integrate adaptive scheduling into PollService** - `fbe4c3d` (feat)

## Files Created/Modified

- `internal/application/adaptive.go` - ActivityTier type, classifyActivity, tierInterval, freshestActivity, repoSchedule, ScheduleInfo
- `internal/application/adaptive_test.go` - Table-driven tests for tier classification, intervals, freshest activity, String()
- `internal/application/pollservice.go` - Adaptive scheduling integration: schedules map, updateSchedule, pollDueRepos, initializeSchedules, Schedules() accessor
- `internal/application/pollservice_test.go` - TestAdaptiveScheduling with adaptiveMockPRStore for multi-repo tier verification

## Decisions Made

- Named interval constants (`intervalHot`, `intervalActive`, `intervalWarm`, `intervalStale`) instead of magic numbers to satisfy `mnd` linter rule
- `sync.RWMutex` protects schedules map since `Schedules()` accessor may be called from different goroutine than `Start()`
- Deferred `repoSchedule` struct creation from Task 1 to Task 2 to avoid unused-type linter error
- Exported `ScheduleInfo` struct for observability/testing, separate from internal `repoSchedule`
- `pollAll` updates schedules only on successful repo polls (errors skip schedule update)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Magic number linter error on interval constants**
- **Found during:** Task 1 (commit attempt)
- **Issue:** `golangci-lint` `mnd` rule flagged `15 * time.Minute` as a magic number
- **Fix:** Extracted all tier intervals into named `const` block (`intervalHot`, `intervalActive`, `intervalWarm`, `intervalStale`)
- **Files modified:** `internal/application/adaptive.go`
- **Verification:** `golangci-lint` passes with 0 issues
- **Committed in:** `1add094` (Task 1 commit)

**2. [Rule 3 - Blocking] Unused type linter error on repoSchedule**
- **Found during:** Task 1 (commit attempt)
- **Issue:** `golangci-lint` `unused` rule flagged `repoSchedule` struct since it was not yet used by pollservice.go
- **Fix:** Moved `repoSchedule` definition to Task 2 where it is consumed
- **Files modified:** `internal/application/adaptive.go`
- **Verification:** `golangci-lint` passes with 0 issues
- **Committed in:** `1add094` (Task 1 commit)

**3. [Rule 1 - Bug] Data race on schedules map access**
- **Found during:** Task 2 (pre-commit race detection)
- **Issue:** `Schedules()` accessor reads the map from the test goroutine while `Start()` writes from its goroutine, causing a data race
- **Fix:** Added `sync.RWMutex` (`schedulesMu`) protecting all reads/writes to the schedules map
- **Files modified:** `internal/application/pollservice.go`
- **Verification:** `go test -race` passes with 0 race warnings
- **Committed in:** `fbe4c3d` (Task 2 commit)

---

**Total deviations:** 3 auto-fixed (1 bug, 2 blocking)
**Impact on plan:** All auto-fixes necessary for correctness and CI compliance. No scope creep.

## Issues Encountered

None beyond the auto-fixed deviations above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 6 (Docker Deployment) is now complete with both plans executed
- Dockerfile, docker-compose, healthcheck binary, and adaptive polling all in place
- Project is feature-complete through all 6 phases

## Self-Check: PASSED

- All 4 files verified present
- Commit `1add094` found (Task 1)
- Commit `fbe4c3d` found (Task 2)
- `go test -race ./internal/application/...` passes
- `go build ./...` succeeds

---
*Phase: 06-docker-deployment*
*Completed: 2026-02-14*
