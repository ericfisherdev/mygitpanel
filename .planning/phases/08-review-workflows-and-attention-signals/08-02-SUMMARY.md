---
phase: 08-review-workflows-and-attention-signals
plan: 02
subsystem: api
tags: [go-github, graphql, github-api, write-operations, credential-management, mutex]

# Dependency graph
requires:
  - phase: 08-01
    provides: "Domain models (Review, ReviewComment, IssueComment), ReviewStore port, SQLite adapter, migrations"
provides:
  - "GitHubClient port with 4 write methods (CreateReview, CreateIssueComment, ReplyToReviewComment, SetDraftStatus)"
  - "GitHub adapter implementations for all 4 write methods"
  - "GitHubClientProvider for runtime credential hot-swap via RWMutex"
  - "Config.Load() tolerant of missing GitHub credentials"
  - "Config.HasGitHubCredentials() helper"
affects: [08-03, 08-04, composition-root, web-handlers]

# Tech tracking
tech-stack:
  added: []
  patterns: [raw-graphql-mutations, rwmutex-provider-pattern, optional-env-vars]

key-files:
  created:
    - internal/adapter/driven/github/reviews_write.go
    - internal/application/clientprovider.go
    - internal/application/clientprovider_test.go
  modified:
    - internal/domain/port/driven/githubclient.go
    - internal/adapter/driven/github/graphql.go
    - internal/config/config.go
    - internal/config/config_test.go
    - internal/application/pollservice_test.go

key-decisions:
  - "Raw GraphQL mutations for draft toggle (markReadyMutation, convertToDraftMutation) following existing FetchThreadResolution pattern"
  - "GitHubClientProvider uses sync.RWMutex for thread-safe Get/Replace/HasClient"
  - "Config credentials are optional with os.Getenv fallback (backward compatible)"

patterns-established:
  - "RWMutex provider pattern: GitHubClientProvider wraps driven.GitHubClient for runtime hot-swap without restart"
  - "Write method separation: reviews_write.go keeps write operations in a dedicated file from read operations in client.go"
  - "GraphQL mutation response type: graphqlMutationResponse for mutations that only need error checking"

# Metrics
duration: 3min
completed: 2026-02-15
---

# Phase 8 Plan 2: GitHub Write Operations and Credential Hot-Swap Summary

**4 GitHub write methods (review, comment, reply, draft toggle) on adapter with RWMutex-based client provider for runtime credential swap**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-16T04:14:22Z
- **Completed:** 2026-02-16T04:17:45Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments

- Extended GitHubClient port from 9 to 13 methods (4 new write methods)
- Implemented REST write operations (CreateReview, CreateIssueComment, ReplyToReviewComment) using go-github v82
- Implemented SetDraftStatus via raw GraphQL mutations following existing FetchThreadResolution pattern
- Created GitHubClientProvider with RWMutex-guarded hot-swap and concurrent safety tests
- Made Config.Load() backward-compatible with optional GitHub credentials

## Task Commits

Each task was committed atomically:

1. **Task 1: GitHubClient write methods and GitHubClientProvider** - `6990a38` (feat)
2. **Task 2: Make config tolerant of missing GitHub credentials** - `bfbfbac` (feat)

## Files Created/Modified

- `internal/domain/port/driven/githubclient.go` - Extended GitHubClient port with 4 write method signatures
- `internal/adapter/driven/github/reviews_write.go` - REST write implementations (CreateReview, CreateIssueComment, ReplyToReviewComment)
- `internal/adapter/driven/github/graphql.go` - GraphQL mutations for draft toggle (SetDraftStatus) and mutation response type
- `internal/application/clientprovider.go` - GitHubClientProvider with RWMutex Get/Replace/HasClient
- `internal/application/clientprovider_test.go` - Provider tests including concurrent safety
- `internal/application/pollservice_test.go` - Added write method stubs to mockGitHubClient
- `internal/config/config.go` - Optional GitHub credentials, HasGitHubCredentials() helper
- `internal/config/config_test.go` - Updated tests for optional credentials, added HasGitHubCredentials table-driven tests

## Decisions Made

- Raw GraphQL mutations for draft toggle (two const strings: markReadyMutation, convertToDraftMutation) rather than introducing shurcooL/githubv4 dependency
- Separate graphqlMutationResponse type for mutations (only checks errors, ignores payload) vs the existing graphqlResponse for queries
- Write methods in dedicated reviews_write.go file to keep client.go focused on read operations
- Config uses os.Getenv (empty string fallback) instead of LookupEnv with error for GitHub credentials

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed misspelling flagged by golangci-lint**
- **Found during:** Task 1 (commit attempt)
- **Issue:** "marshalling" flagged as misspelling by misspell linter (American English: "marshaling")
- **Fix:** Changed to "marshaling" in error message
- **Files modified:** internal/adapter/driven/github/graphql.go
- **Verification:** golangci-lint passes with 0 issues
- **Committed in:** 6990a38 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Trivial spelling fix required by linter. No scope creep.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- GitHubClient port now has all write methods needed for review workflows
- GitHubClientProvider ready to be wired into composition root and PollService
- Config flexibility allows app startup without env vars (GUI credential entry path)
- Next plan (08-03) can build web handlers that use these write methods

---
*Phase: 08-review-workflows-and-attention-signals*
*Completed: 2026-02-15*
