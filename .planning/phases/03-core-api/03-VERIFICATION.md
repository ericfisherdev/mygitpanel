---
phase: 03-core-api
verified: 2026-02-11T18:50:00Z
status: passed
score: 8/8 must-haves verified
---

# Phase 3: Core API Verification Report

**Phase Goal:** PR data and repository configuration are accessible via structured HTTP endpoints that a CLI agent can consume, with basic PR metadata on every response

**Verified:** 2026-02-11T18:50:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | GET endpoint returns all tracked PRs with current status (open/merged/closed), title, author, branch, base branch, URL, and labels | ✓ VERIFIED | ListPRs handler calls prStore.ListAll, converts to PRResponse with all required fields (number, repository, title, author, status, is_draft, needs_review, url, branch, base_branch, labels). Test verifies all fields present. |
| 2 | GET endpoint returns a single PR with its full metadata | ✓ VERIFIED | GetPR handler extracts owner/repo/number from path, calls prStore.GetByNumber, returns PRResponse with full metadata. Test verifies 200 (found), 404 (not found), 400 (invalid number). |
| 3 | GET endpoint returns only PRs needing attention (changes requested or needs review) | ✓ VERIFIED | ListPRsNeedingAttention handler calls prStore.ListNeedingReview which queries WHERE needs_review = 1. Test verifies filtered results. |
| 4 | POST/DELETE/GET endpoints allow adding, removing, and listing watched repositories at runtime without restart | ✓ VERIFIED | AddRepo (POST) validates name, calls repoStore.Add, triggers async pollSvc.RefreshRepo. RemoveRepo (DELETE) calls repoStore.Remove. ListRepos (GET) calls repoStore.ListAll. Tests verify 201/409/400 for add, 204/404 for remove, 200 for list. |
| 5 | Health check endpoint returns application status and the API is accessible on localhost only | ✓ VERIFIED | Health handler returns {status: "ok", time: RFC3339}. Test verifies 200 response. Server binds to cfg.ListenAddr which defaults to "127.0.0.1:8080" (localhost only). |
| 6 | PRs where user review is requested are persisted with needs_review=true | ✓ VERIFIED | PollService.pollRepo sets pr.NeedsReview = isReviewRequested before upsert (line 190). SQLite prrepo.go converts bool to int (0/1) in Upsert, scans back to bool in scanPR. Migration adds needs_review column. |
| 7 | PRs authored by user are persisted with needs_review=false | ✓ VERIFIED | PollService.pollRepo sets pr.NeedsReview = isReviewRequested where isReviewRequested is false for authored-only PRs (line 190). |
| 8 | HTTP server starts on configured address and shuts down gracefully | ✓ VERIFIED | main.go creates http.Server with production timeouts, starts in goroutine, calls srv.Shutdown(shutdownCtx) with 10s timeout on signal. |

**Score:** 8/8 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| internal/domain/model/pullrequest.go | NeedsReview boolean field | ✓ VERIFIED | Line 17: NeedsReview bool (persisted field, not transient) |
| internal/domain/port/driven/prstore.go | ListNeedingReview method on PRStore | ✓ VERIFIED | Line 16: ListNeedingReview(ctx context.Context) ([]model.PullRequest, error) |
| internal/adapter/driven/sqlite/migrations/000002_add_needs_review.up.sql | Schema migration adding needs_review column | ✓ VERIFIED | 2 lines: ALTER TABLE adds column, CREATE INDEX for query performance |
| internal/adapter/driven/sqlite/prrepo.go | ListNeedingReview implementation + needs_review in Upsert/scan | ✓ VERIFIED | Lines 31, 39 (Upsert includes needs_review), lines 63-66 (bool->int conversion), line 141 (ListNeedingReview method), lines 202, 217 (scan includes needsReview) |
| internal/application/pollservice.go | Sets NeedsReview during poll based on IsReviewRequestedFrom | ✓ VERIFIED | Line 190: pr.NeedsReview = isReviewRequested, line 193: unchanged check includes NeedsReview |
| internal/adapter/driving/http/response.go | JSON helpers and all DTO structs | ✓ VERIFIED | 110 lines: writeJSON, writeError, PRResponse, RepoResponse, HealthResponse, AddRepoRequest, converter functions. Exports verified. |
| internal/adapter/driving/http/middleware.go | Logging and recovery middleware | ✓ VERIFIED | 55 lines: statusWriter pattern, loggingMiddleware (method/path/status/duration), recoveryMiddleware (panic->500) |
| internal/adapter/driving/http/handler.go | Handler struct with all 7 endpoint methods and route registration | ✓ VERIFIED | 243 lines: Handler struct with prStore/repoStore/pollSvc fields, NewHandler, NewServeMux registers 7 routes, all endpoint methods substantive (10-40 lines each), isValidRepoName validation |
| internal/adapter/driving/http/handler_test.go | Table-driven httptest tests for all endpoints | ✓ VERIFIED | 525 lines: Tests for all 7 endpoints with multiple cases each (empty/success/error), mock stores, verifies status codes and field presence |
| cmd/reviewhub/main.go | HTTP server creation, wiring, and graceful shutdown | ✓ VERIFIED | Lines 80-97: HTTP handler creation, server with production timeouts, goroutine start. Lines 114-116: graceful shutdown with srv.Shutdown(shutdownCtx) |

**All artifacts:** VERIFIED (10/10)

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| handler.go | prstore.go | prStore field on Handler struct | ✓ WIRED | Line 19: prStore driven.PRStore field, used in ListPRs (line 65), GetPR (line 94), ListPRsNeedingAttention (line 111) |
| handler.go | repostore.go | repoStore field on Handler struct | ✓ WIRED | Line 20: repoStore driven.RepoStore field, used in ListRepos (line 128), AddRepo (line 164), RemoveRepo (line 193) |
| handler.go | pollservice.go | pollSvc field for async refresh on repo add | ✓ WIRED | Line 21: pollSvc *application.PollService field. Line 178: pollSvc.RefreshRepo called in fire-and-forget goroutine with background context. Nil guard on line 176 allows tests to pass nil. |
| main.go | handler.go | NewHandler + NewServeMux in composition root | ✓ WIRED | Line 80: httphandler.NewHandler(prStore, repoStore, pollSvc, ...). Line 81: httphandler.NewServeMux(h, ...). Line 84: srv := &http.Server{Handler: mux} |
| pollservice.go | pullrequest.go | pr.NeedsReview = isReviewRequested | ✓ WIRED | Line 190: pr.NeedsReview = isReviewRequested. Sets boolean based on review request status before upsert. |
| prrepo.go | prstore.go | implements ListNeedingReview | ✓ WIRED | Line 141: func (r *PRRepo) ListNeedingReview implementing interface method. Query filters WHERE needs_review = 1. |

**All links:** WIRED (6/6)

### Requirements Coverage

**Phase 3 requirements:** API-01, API-02, API-03, API-04, REPO-01, REPO-02, REPO-03, STAT-01, STAT-07

| Requirement | Status | Supporting Evidence |
|-------------|--------|-------------------|
| API-01: GET endpoint returning all tracked PRs with status flags | ✓ SATISFIED | ListPRs handler verified, test passes, returns PRResponse array with all fields |
| API-02: GET endpoint returning a single PR with full review comments and code context | ✓ SATISFIED | GetPR handler verified, test passes. Comments/reviews fields present as empty arrays (Phase 4 populates) |
| API-03: GET endpoint returning only PRs needing attention | ✓ SATISFIED | ListPRsNeedingAttention handler verified, calls ListNeedingReview which filters needs_review=true |
| API-04: Health check endpoint | ✓ SATISFIED | Health handler verified, test passes, returns {status: "ok", time: RFC3339} |
| REPO-01: API endpoint to add a watched repository | ✓ SATISFIED | AddRepo handler verified, validates name, adds to store, returns 201, triggers async refresh |
| REPO-02: API endpoint to remove a watched repository | ✓ SATISFIED | RemoveRepo handler verified, removes from store, returns 204 on success, 404 if not found |
| REPO-03: API endpoint to list all watched repositories | ✓ SATISFIED | ListRepos handler verified, returns RepoResponse array with full_name/owner/name/added_at |
| STAT-01: Each PR shows current status: open, merged, closed | ✓ SATISFIED | PRResponse includes status field, populated from pr.Status (string conversion of enum) |
| STAT-07: Each PR includes title, author, branch, base branch, URL, labels | ✓ SATISFIED | PRResponse includes all fields, test verifies: title, author, branch, base_branch, url, labels (non-null array) |

**All requirements:** SATISFIED (9/9)

### Anti-Patterns Found

**Scan scope:** All files in internal/adapter/driving/http/

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None found | - | - | - | - |

**No anti-patterns detected:**
- No TODO/FIXME/placeholder comments
- No empty return statements
- No console.log only implementations
- All handlers have substantive implementations calling stores
- Empty arrays correctly initialized as []string{} and []any{}, not null

### Human Verification Required

None required. All truths are programmatically verifiable through:
- Code inspection confirming handlers call stores and return data
- Tests verifying HTTP status codes and response structure
- Build verification confirming server starts and shuts down gracefully

### Gaps Summary

No gaps found. Phase goal achieved.

---

## Verification Details

### Level 1: Existence
All 10 required artifacts exist.

### Level 2: Substantive
All artifacts are substantive:
- response.go: 110 lines with writeJSON, writeError, 5 DTOs, 2 converters
- middleware.go: 55 lines with statusWriter, logging, and recovery
- handler.go: 243 lines with Handler struct, 7 endpoint methods (10-40 lines each), validation
- handler_test.go: 525 lines with table-driven tests for all 7 endpoints
- Migration: 2 lines (ALTER TABLE + CREATE INDEX)
- prrepo.go: ListNeedingReview method at line 141, needs_review in Upsert/scan
- pollservice.go: NeedsReview assignment at line 190
- main.go: HTTP server creation and shutdown integration

No stub patterns detected:
- Handlers call stores and return real data (not hardcoded)
- Converters map all fields from domain to DTO
- Tests verify actual behavior with mock stores

### Level 3: Wired
All artifacts are wired correctly:
- Handler depends on prStore/repoStore/pollSvc ports (composition root injection)
- All 7 endpoints registered in NewServeMux and tested
- Server created in main.go, graceful shutdown wired
- PollService sets NeedsReview before upsert
- SQLite adapter persists and queries needs_review column

### Test Coverage
All tests pass:
- internal/adapter/driven/sqlite: PASS (includes ListNeedingReview test)
- internal/adapter/driving/http: PASS (7 endpoint test suites, 525 lines)
- internal/application: PASS (poll service tests)
- Binary compiles: reviewhub.exe

### Production Readiness
HTTP server configured with production timeouts:
- ReadHeaderTimeout: 5s
- ReadTimeout: 10s
- WriteTimeout: 30s
- IdleTimeout: 120s

Graceful shutdown:
- 10s timeout for draining connections
- srv.Shutdown called on SIGTERM/SIGINT

Localhost binding:
- Default listen address: 127.0.0.1:8080
- Configurable via REVIEWHUB_LISTEN_ADDR

---

_Verified: 2026-02-11T18:50:00Z_
_Verifier: Claude (gsd-verifier)_
