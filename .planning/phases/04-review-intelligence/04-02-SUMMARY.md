---
phase: 04-review-intelligence
plan: 02
subsystem: api
tags: [go-github, graphql, github-api, rest-api, pagination, hexagonal]

# Dependency graph
requires:
  - phase: 02-github-integration
    provides: "GitHubClient adapter with go-github, transport stack, NewClientWithHTTPClient for testing"
  - phase: 04-01
    provides: "Review, ReviewComment, IssueComment domain entities; GitHubClient port with FetchIssueComments/FetchThreadResolution; HeadSHA on PullRequest"
provides:
  - "FetchReviews real implementation with pagination replacing stub"
  - "FetchReviewComments real implementation with InReplyToID mapping and pagination"
  - "FetchIssueComments real implementation with pagination"
  - "FetchThreadResolution via GitHub GraphQL API with graceful degradation"
  - "HeadSHA populated in mapPullRequest and persisted via PRRepo.Upsert"
  - "Client struct with token and graphqlURL fields for GraphQL auth"
affects: [04-03-PLAN, 04-04-PLAN]

# Tech tracking
tech-stack:
  added: []
  patterns: [graphql-supplementary-data, graceful-degradation-on-error, bearer-token-graphql-auth]

key-files:
  created:
    - internal/adapter/driven/github/graphql.go
    - internal/adapter/driven/github/graphql_test.go
  modified:
    - internal/adapter/driven/github/client.go
    - internal/adapter/driven/github/client_test.go
    - internal/adapter/driven/sqlite/prrepo.go

key-decisions:
  - "GraphQL used only for thread resolution (isResolved) -- REST API does not expose this field"
  - "All GraphQL error paths return empty map, never propagate errors -- supplementary data source"
  - "NewClientWithHTTPClient gains token as 4th parameter; derives graphqlURL from baseURL for testability"
  - "FetchThreadResolution skips HTTP call entirely when token is empty (graceful no-op)"

patterns-established:
  - "GraphQL as supplementary: never fail on GraphQL errors, log warning, return empty result"
  - "mapReview/mapReviewComment/mapIssueComment follow same GetXxx() pattern as mapPullRequest"
  - "PRID=0 convention: adapter sets PRID to 0, caller assigns actual DB ID before persisting"

# Metrics
duration: 4min
completed: 2026-02-13
---

# Phase 4 Plan 2: GitHub Adapter Implementation Summary

**FetchReviews, FetchReviewComments, FetchIssueComments with pagination replacing stubs, plus GraphQL thread resolution with graceful degradation and HeadSHA persistence**

## Performance

- **Duration:** 4 min
- **Started:** 2026-02-13T05:07:19Z
- **Completed:** 2026-02-13T05:11:39Z
- **Tasks:** 2
- **Files modified:** 5 (3 modified, 2 created)

## Accomplishments
- Replaced 3 stub methods (FetchReviews, FetchReviewComments, FetchIssueComments) with real GitHub API implementations using go-github with pagination
- Created minimal GraphQL client for thread resolution status with 4-layer graceful degradation (no token, HTTP error, GraphQL error, parse error)
- Added HeadSHA mapping in mapPullRequest and persistence in PRRepo (INSERT, UPDATE, SELECT, scan)
- Added 8 new test cases (4 REST adapter tests + 4 GraphQL tests) all passing via httptest mocks

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement FetchReviews, FetchReviewComments, FetchIssueComments, and HeadSHA** - `7665452` (feat)
2. **Task 2: Implement GraphQL thread resolution client** - `292f2ec` (feat)

## Files Created/Modified
- `internal/adapter/driven/github/client.go` - Real fetch implementations replacing stubs; Client struct with token/graphqlURL fields; mapReview, mapReviewComment, mapIssueComment mapping functions; HeadSHA in mapPullRequest
- `internal/adapter/driven/github/client_test.go` - 4 new tests (HeadSHA, FetchReviews, FetchReviewComments, FetchIssueComments); updated newTestClient to pass token parameter
- `internal/adapter/driven/github/graphql.go` - FetchThreadResolution method using net/http POST to GraphQL API; typed response structs; graceful degradation on all error paths
- `internal/adapter/driven/github/graphql_test.go` - 4 tests: success with 2 threads, GraphQL errors, no-token skip, HTTP 500 error
- `internal/adapter/driven/sqlite/prrepo.go` - Added head_sha to INSERT, ON CONFLICT UPDATE, all SELECT queries, and scanPR

## Decisions Made
- GraphQL used only for thread resolution (isResolved) since REST API does not expose this field
- All GraphQL error paths return empty map with logged warning -- never fail on supplementary data
- NewClientWithHTTPClient gains `token` as 4th parameter; derives graphqlURL from baseURL for test interceptability
- FetchThreadResolution skips HTTP call entirely when token is empty (graceful no-op for tests)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All 4 GitHub adapter fetch methods are real implementations ready for Plan 04-03 (enrichment service)
- FetchThreadResolution provides resolution map for enrichment service to merge into ReviewComment.IsResolved
- HeadSHA available for outdated review detection in enrichment service
- Client struct stores token for GraphQL; NewClient signature unchanged (Plan 04-04 composition root needs no change for NewClient)
- NewClientWithHTTPClient signature changed (4th param token) -- Plan 04-04 must update any direct calls if applicable

## Self-Check: PASSED

All 5 files verified present. Both commit hashes (7665452, 292f2ec) confirmed in git log.

---
*Phase: 04-review-intelligence*
*Completed: 2026-02-13*
