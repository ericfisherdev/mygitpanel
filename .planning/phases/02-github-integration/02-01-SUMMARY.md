---
phase: 02-github-integration
plan: 01
subsystem: github-adapter
tags: [go-github, httpcache, rate-limiting, hexagonal, adapter]
depends_on:
  requires: [01-01]
  provides: [github-client-adapter, fetch-pull-requests]
  affects: [02-02, 03-01]
tech-stack:
  added: [google/go-github/v82, gregjones/httpcache, gofri/go-github-ratelimit/v2]
  patterns: [driven-adapter, transport-stack, compile-time-interface-check]
key-files:
  created:
    - internal/adapter/driven/github/client.go
    - internal/adapter/driven/github/client_test.go
  modified:
    - go.mod
    - go.sum
decisions:
  - id: 02-01-01
    description: "Transport stack order: httpcache -> rate-limit -> go-github for ETag caching before rate limit middleware"
  - id: 02-01-02
    description: "NewClientWithHTTPClient constructor exported for test injection of httptest servers"
  - id: 02-01-03
    description: "FetchReviews and FetchReviewComments stubbed as nil/nil returns for Phase 4"
metrics:
  duration: 8min
  completed: 2026-02-11
---

# Phase 2 Plan 1: GitHub API Adapter Summary

**GitHub adapter implementing driven.GitHubClient port with go-github v82, httpcache ETag caching, and secondary rate limit middleware**

## What Was Done

### Task 1: Install dependencies and create GitHub adapter
**Commit:** `ae19667`

Created `internal/adapter/driven/github/client.go` implementing the `driven.GitHubClient` port:

- **Transport stack:** `httpcache.MemoryCacheTransport` (ETag-based conditional requests) -> `github_ratelimit.NewClient` (sleeps on 429/Retry-After) -> `go-github` client with PAT auth via `WithAuthToken()`
- **FetchPullRequests:** Full pagination loop using `resp.NextPage`, maps all go-github types to domain model using `GetXxx()` nil-safe accessors
- **mapPullRequest:** Correctly handles merged (via `GetMergedAt().IsZero()`), closed, and open status; draft detection via `GetDraft()`; labels initialized as empty slice not nil
- **Rate limit logging:** `slog.Debug` after every API call with rate remaining/limit, `slog.Warn` when remaining < 100
- **splitRepo:** Validates "owner/repo" format including empty parts
- **Interface satisfaction:** `var _ driven.GitHubClient = (*Client)(nil)` compile-time check
- **Stubs:** `FetchReviews` and `FetchReviewComments` return `nil, nil` (Phase 4)

### Task 2: Unit tests for GitHub adapter
**Commit:** `c56eee8`

Created `internal/adapter/driven/github/client_test.go` with 6 test functions (10 cases with subtests):

| Test | What It Verifies |
|------|-----------------|
| `TestFetchPullRequests_SinglePage` | All field mapping (Number, Title, Author, Status, IsDraft, URL, Branch, BaseBranch, Labels) |
| `TestFetchPullRequests_Pagination` | Multi-page collection via Link header parsing |
| `TestFetchPullRequests_DraftDetection` | `draft: true` maps to `IsDraft: true` and vice versa |
| `TestFetchPullRequests_EmptyRepo` | Empty repo returns `[]` (not nil), no error |
| `TestFetchPullRequests_InvalidRepoName` | Rejects "invalid", "/repo", "owner/", "" with descriptive error |
| `TestFetchPullRequests_StatusMapping` | open -> PRStatusOpen, closed -> PRStatusClosed, closed+merged_at -> PRStatusMerged |

Tests use `httptest.Server` with custom JSON handlers, no real GitHub API calls.

## Deviations from Plan

None -- plan executed exactly as written.

## Decisions Made

1. **Transport stack wiring** (02-01-01): httpcache transport sits below rate limit middleware so cached responses never trigger rate limit accounting. Order: cache -> rate-limit -> go-github.
2. **Test constructor** (02-01-02): Exported `NewClientWithHTTPClient(httpClient, baseURL, username)` to allow httptest injection without exposing internal fields.
3. **Stub returns** (02-01-03): `FetchReviews` and `FetchReviewComments` return `(nil, nil)` rather than `([]T{}, nil)` since callers should not depend on these until Phase 4.

## Verification Results

| Check | Result |
|-------|--------|
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `go test ./... -count=1` | PASS (all packages) |
| Compile-time interface check | Present and compiles |
| No go-github types in domain/ | Confirmed (zero imports) |
| Pagination tested | Yes (TestFetchPullRequests_Pagination) |
| Transport stack includes httpcache + rate limit | Yes (NewClient constructor) |

## Next Phase Readiness

- GitHub adapter is ready for integration with polling service (Phase 2 Plan 2)
- Transport stack is fully wired; no additional HTTP configuration needed
- FetchReviews/FetchReviewComments stubs will need real implementations in Phase 4
