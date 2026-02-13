# Phase 3: Core API - Research

**Researched:** 2026-02-11
**Domain:** Go HTTP REST API with stdlib net/http, JSON serialization, hexagonal architecture
**Confidence:** HIGH

## Summary

Phase 3 exposes PR data and repository configuration as JSON HTTP endpoints consumed by a CLI agent. The project already uses Go 1.25 and stdlib throughout (slog, signal.NotifyContext), making Go's enhanced `net/http.ServeMux` (Go 1.22+) the natural choice -- no third-party router is needed. The existing codebase has a composition root with a 10s shutdown timeout pre-wired for HTTP server drain, port interfaces for PRStore and RepoStore already defined, and config with `ListenAddr` defaulting to `127.0.0.1:8080`.

The standard approach is: use `net/http.ServeMux` with method+path patterns (Go 1.22+ syntax), `encoding/json` for serialization, and `net/http/httptest` for testing. HTTP handlers are "driving adapters" in hexagonal architecture, placed at `internal/adapter/driving/http/`. A thin JSON helper layer (writeJSON/writeError) standardizes response formatting. The http.Server is created in the composition root with production timeouts and wired into the existing graceful shutdown flow.

**Primary recommendation:** Use Go stdlib `net/http.ServeMux` with Go 1.22+ routing patterns. Zero external dependencies for the HTTP layer. Handlers call existing port interfaces directly.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| net/http | stdlib (Go 1.25) | HTTP server, routing, request/response | Go 1.22+ ServeMux supports method matching + path params -- no router needed |
| net/http/httptest | stdlib | Handler testing | Standard Go HTTP test tooling, already used in Phase 2 tests |
| encoding/json | stdlib v1 | JSON marshaling/unmarshaling | Stable, well-understood; v2 is experimental and not subject to compat promise |
| log/slog | stdlib | Structured logging in middleware | Already used throughout the project |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| strconv | stdlib | Path parameter parsing (string to int) | Converting PR number from URL path to int |
| strings | stdlib | Repo name parsing/validation | Splitting owner/repo for path params |
| fmt | stdlib | Error message formatting | Error response messages |
| time | stdlib | Timestamp formatting in JSON responses | ISO 8601 output in API responses |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| net/http.ServeMux | chi v5 | Chi adds middleware chaining and route groups, but stdlib now has method+wildcard support. Extra dependency for minimal gain given this API's small surface area (8 endpoints). |
| net/http.ServeMux | gorilla/mux | Gorilla is in maintenance mode. stdlib is the path forward. |
| encoding/json v1 | encoding/json/v2 | v2 is experimental in Go 1.25 (GOEXPERIMENT=jsonv2). Not subject to Go 1 compat promise. Not worth the risk for a production tool. |

**Installation:**
```bash
# No additional dependencies needed -- all stdlib
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
  adapter/
    driving/
      http/
        handler.go          # HTTP handler methods (struct with deps)
        handler_test.go     # Table-driven httptest tests
        response.go         # writeJSON, writeError helpers
        middleware.go        # Logging, recovery middleware
    driven/
      sqlite/               # (existing)
      github/               # (existing)
  application/
    pollservice.go          # (existing)
  domain/
    model/                  # (existing, pure structs)
    port/
      driven/               # (existing: PRStore, RepoStore, GitHubClient)
```

**Note on hexagonal naming:** The existing project uses `adapter/driven/` for outbound adapters. HTTP handlers are inbound ("driving") adapters. Place them at `adapter/driving/http/` to maintain the hexagonal naming symmetry.

**No driving port interfaces needed:** In hexagonal architecture, driving ports define how the outside world interacts with the application. Since this API layer calls existing store ports directly (PRStore, RepoStore) and there is no application service to abstract, the handlers can depend directly on the driven port interfaces. This avoids unnecessary abstraction for a CRUD-style API. The PollService is already a concrete type with RefreshRepo/RefreshPR methods that can be called directly for manual refresh triggers if needed.

### Pattern 1: Handler Struct with Dependencies

**What:** A single handler struct holds all dependencies (stores, poll service) and has methods for each endpoint.
**When to use:** Always for this size of API (8 endpoints).
**Example:**
```go
// Source: Go stdlib patterns, verified against Go 1.22+ routing docs
type Handler struct {
    prStore   driven.PRStore
    repoStore driven.RepoStore
    pollSvc   *application.PollService
    username  string
    logger    *slog.Logger
}

func NewHandler(prStore driven.PRStore, repoStore driven.RepoStore, pollSvc *application.PollService, username string, logger *slog.Logger) *Handler {
    return &Handler{
        prStore:   prStore,
        repoStore: repoStore,
        pollSvc:   pollSvc,
        username:  username,
        logger:    logger,
    }
}
```

### Pattern 2: Go 1.22+ ServeMux Route Registration

**What:** Method-specific routes with path parameters using `{name}` syntax.
**When to use:** All route registration.
**Example:**
```go
// Source: https://go.dev/blog/routing-enhancements
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
    // PR endpoints
    mux.HandleFunc("GET /api/v1/prs", h.ListPRs)
    mux.HandleFunc("GET /api/v1/prs/attention", h.ListPRsNeedingAttention)
    mux.HandleFunc("GET /api/v1/repos/{owner}/{repo}/prs/{number}", h.GetPR)

    // Repository management endpoints
    mux.HandleFunc("GET /api/v1/repos", h.ListRepos)
    mux.HandleFunc("POST /api/v1/repos", h.AddRepo)
    mux.HandleFunc("DELETE /api/v1/repos/{owner}/{repo}", h.RemoveRepo)

    // Health check
    mux.HandleFunc("GET /api/v1/health", h.Health)
}
```

**Key routing details (Go 1.22+ verified):**
- `"GET /api/v1/prs"` matches GET only; auto-returns 405 for other methods
- `{owner}` and `{repo}` are single-segment wildcards, extracted via `r.PathValue("owner")`
- GET automatically matches HEAD requests
- No trailing slash means exact match (no subtree matching)
- Patterns with methods are more specific than without -- no conflicts

### Pattern 3: JSON Response Helpers

**What:** Small helper functions that standardize JSON response writing.
**When to use:** Every handler response.
**Example:**
```go
// Source: https://benhoyt.com/writings/web-service-stdlib/
func writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    b, err := json.Marshal(v)
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        w.Write([]byte(`{"error":"internal server error"}`))
        return
    }
    w.WriteHeader(status)
    w.Write(b)
}

type errorResponse struct {
    Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, message string) {
    writeJSON(w, status, errorResponse{Error: message})
}
```

**Why json.Marshal instead of json.NewEncoder:** Marshal-to-bytes-first allows checking for marshal errors before writing any response headers/status code. With NewEncoder, once Encode starts writing to the ResponseWriter, the status code is implicitly set to 200 and cannot be changed on error.

### Pattern 4: Graceful Shutdown Integration

**What:** Wire http.Server into the existing shutdown flow.
**When to use:** Composition root (main.go).
**Example:**
```go
// Source: Go net/http docs, existing main.go shutdown pattern
srv := &http.Server{
    Addr:              cfg.ListenAddr,
    Handler:           mux,
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       10 * time.Second,
    WriteTimeout:      30 * time.Second,
    IdleTimeout:       120 * time.Second,
}

// Start HTTP server in goroutine
go func() {
    slog.Info("http server starting", "addr", cfg.ListenAddr)
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        slog.Error("http server error", "error", err)
    }
}()

// Wait for shutdown signal
<-ctx.Done()

// Use the pre-wired 10s shutdown timeout
shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
if err := srv.Shutdown(shutdownCtx); err != nil {
    slog.Error("http server shutdown error", "error", err)
}
```

### Pattern 5: API Response DTOs (Separate from Domain Models)

**What:** Dedicated response structs with JSON tags, separate from domain model structs.
**When to use:** All API responses. Never marshal domain model structs directly.
**Why:** Domain models may have internal fields (ID, transient fields) that should not leak. JSON tags on domain structs would couple domain to serialization. Separate DTOs allow API evolution without domain changes.

```go
type PRResponse struct {
    Number     int      `json:"number"`
    Repository string   `json:"repository"`
    Title      string   `json:"title"`
    Author     string   `json:"author"`
    Status     string   `json:"status"`
    IsDraft    bool     `json:"is_draft"`
    URL        string   `json:"url"`
    Branch     string   `json:"branch"`
    BaseBranch string   `json:"base_branch"`
    Labels     []string `json:"labels"`
    OpenedAt   string   `json:"opened_at"`
    UpdatedAt  string   `json:"updated_at"`
}
```

### Anti-Patterns to Avoid
- **Marshaling domain models directly to JSON:** Leaks internal fields, couples domain to HTTP. Always use DTO conversion functions.
- **Using json.NewEncoder(w).Encode() for API responses:** Loses ability to handle marshal errors before writing status code. Use json.Marshal to bytes first.
- **Global mux (http.DefaultServeMux):** Tests cannot isolate routes. Always create `http.NewServeMux()` explicitly.
- **Missing http.Server timeouts:** Default timeouts are zero (infinite). Always set ReadHeaderTimeout, ReadTimeout, WriteTimeout, IdleTimeout.
- **Putting business logic in handlers:** Handlers should parse request, call store/service, format response. No filtering, sorting, or computation in handlers.

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| HTTP routing with methods | Custom regex router | stdlib net/http.ServeMux (Go 1.22+) | Method matching, path params, 405 handling all built-in |
| HTTP test infrastructure | Custom test helpers | net/http/httptest (NewRequest, NewRecorder) | Standard, well-tested, already used in Phase 2 |
| JSON serialization | Custom marshaling | encoding/json with struct tags | Battle-tested, handles edge cases (nil slices, time formatting) |
| Content-Type negotiation | Custom header parsing | Hard-code `application/json` | Single format API -- no negotiation needed |
| Graceful shutdown | Custom drain logic | http.Server.Shutdown(ctx) | Handles listener close, idle conn close, in-flight drain |
| Method Not Allowed (405) | Custom 405 responses | Go 1.22+ ServeMux auto-handles | Automatic when method-specific patterns are registered |

**Key insight:** Go 1.22+ ServeMux eliminated the two main reasons teams used third-party routers (method matching and path parameters). For an 8-endpoint API, stdlib is all that is needed.

## Common Pitfalls

### Pitfall 1: Writing Status Code After Body Write
**What goes wrong:** Calling `w.WriteHeader()` after `w.Write()` has already been called silently fails -- the status code defaults to 200.
**Why it happens:** `w.Write()` implicitly calls `w.WriteHeader(200)` on first call if not yet set.
**How to avoid:** Always call `w.Header().Set(...)` then `w.WriteHeader(status)` then `w.Write(body)` in that exact order. The writeJSON helper enforces this.
**Warning signs:** All error responses return 200 status codes.

### Pitfall 2: Nil Slice Marshals to `null` Instead of `[]`
**What goes wrong:** `json.Marshal([]string(nil))` produces `null`, not `[]`. API consumers expect empty arrays.
**Why it happens:** Go distinguishes nil slices from empty slices, and encoding/json respects this distinction.
**How to avoid:** In DTO conversion, always initialize slices: `labels := pr.Labels; if labels == nil { labels = []string{} }`. The existing PRRepo already does this for Upsert; apply same discipline to API DTOs.
**Warning signs:** JSON responses containing `"labels": null` instead of `"labels": []`.

### Pitfall 3: Missing Server Timeouts
**What goes wrong:** Slowloris-style attacks or misbehaving clients hold connections open indefinitely, exhausting resources.
**Why it happens:** Go's default http.Server has zero (infinite) timeouts.
**How to avoid:** Always set ReadHeaderTimeout, ReadTimeout, WriteTimeout, IdleTimeout on http.Server. For a localhost-only API, shorter timeouts are fine (5s/10s/30s/120s).
**Warning signs:** Connections accumulate under load or during client disconnects.

### Pitfall 4: Not Handling Path Parameter Parse Errors
**What goes wrong:** `r.PathValue("number")` returns a string. If the PR number in the URL is not a valid integer, `strconv.Atoi` fails.
**Why it happens:** ServeMux wildcards match any string, not just integers.
**How to avoid:** Always validate path parameters and return 400 Bad Request with a clear error message.
**Warning signs:** Panics or 500 errors when URL contains non-numeric PR numbers.

### Pitfall 5: "Needs Attention" Logic Without Review Data
**What goes wrong:** API-03 requires PRs "needing attention (changes requested or needs review)". But review data is populated in Phase 4, not Phase 3.
**Why it happens:** The requirement references review-level status, but Phase 3 has no review data yet.
**How to avoid:** For Phase 3, implement "needs attention" based on available data: PRs where `RequestedReviewers` included the user (i.e., the user was asked to review). This is the transient field already populated during polling. However, this field is NOT persisted to SQLite. The planner must decide between: (a) adding a `needs_review` boolean column persisted during poll, or (b) returning all open PRs as "needing attention" in Phase 3 and refining the logic in Phase 4. Option (a) is recommended -- the poll service already checks `IsReviewRequestedFrom` and could persist a flag.
**Warning signs:** Empty "needs attention" endpoint or incorrect results.

### Pitfall 6: Repository Validation on Add
**What goes wrong:** User POSTs a repository with an invalid format (missing owner/name, extra slashes, empty string).
**Why it happens:** No input validation before calling `repoStore.Add()`.
**How to avoid:** Validate the repository full_name matches the `owner/repo` format before persisting. Check for duplicates (already exists) and return 409 Conflict.
**Warning signs:** Database constraint violations returning 500 instead of 400/409.

## Code Examples

Verified patterns from official sources:

### Handler: List All PRs (API-01)
```go
// Implements API-01: GET endpoint returning all tracked PRs with status flags
func (h *Handler) ListPRs(w http.ResponseWriter, r *http.Request) {
    prs, err := h.prStore.ListAll(r.Context())
    if err != nil {
        h.logger.Error("failed to list PRs", "error", err)
        writeError(w, http.StatusInternalServerError, "failed to list pull requests")
        return
    }

    resp := make([]PRResponse, 0, len(prs))
    for _, pr := range prs {
        resp = append(resp, toPRResponse(pr))
    }

    writeJSON(w, http.StatusOK, resp)
}
```

### Handler: Get Single PR (API-02)
```go
// Implements API-02: GET endpoint returning a single PR with full metadata
func (h *Handler) GetPR(w http.ResponseWriter, r *http.Request) {
    owner := r.PathValue("owner")
    repo := r.PathValue("repo")
    numberStr := r.PathValue("number")

    repoFullName := owner + "/" + repo

    number, err := strconv.Atoi(numberStr)
    if err != nil {
        writeError(w, http.StatusBadRequest, "invalid PR number")
        return
    }

    pr, err := h.prStore.GetByNumber(r.Context(), repoFullName, number)
    if err != nil {
        h.logger.Error("failed to get PR", "error", err)
        writeError(w, http.StatusInternalServerError, "failed to get pull request")
        return
    }
    if pr == nil {
        writeError(w, http.StatusNotFound, "pull request not found")
        return
    }

    writeJSON(w, http.StatusOK, toPRResponse(*pr))
}
```

### Handler: Add Repository (REPO-01)
```go
// Implements REPO-01: POST endpoint to add a watched repository
func (h *Handler) AddRepo(w http.ResponseWriter, r *http.Request) {
    var req AddRepoRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    // Validate owner/repo format
    if !isValidRepoName(req.FullName) {
        writeError(w, http.StatusBadRequest, "invalid repository name: expected owner/repo format")
        return
    }

    parts := strings.SplitN(req.FullName, "/", 2)
    repo := model.Repository{
        FullName: req.FullName,
        Owner:    parts[0],
        Name:     parts[1],
    }

    if err := h.repoStore.Add(r.Context(), repo); err != nil {
        // Check for unique constraint violation (duplicate)
        if strings.Contains(err.Error(), "UNIQUE constraint") {
            writeError(w, http.StatusConflict, "repository already exists")
            return
        }
        h.logger.Error("failed to add repo", "error", err)
        writeError(w, http.StatusInternalServerError, "failed to add repository")
        return
    }

    writeJSON(w, http.StatusCreated, toRepoResponse(repo))
}
```

### Handler: Health Check (API-04)
```go
// Implements API-04: Health check endpoint
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
    resp := HealthResponse{
        Status: "ok",
        Time:   time.Now().UTC().Format(time.RFC3339),
    }
    writeJSON(w, http.StatusOK, resp)
}
```

### Table-Driven HTTP Test Pattern
```go
// Source: Go stdlib testing patterns + existing project conventions
func TestListPRs(t *testing.T) {
    tests := []struct {
        name       string
        stored     []model.PullRequest
        wantStatus int
        wantCount  int
    }{
        {
            name:       "empty list",
            stored:     nil,
            wantStatus: http.StatusOK,
            wantCount:  0,
        },
        {
            name: "two PRs",
            stored: []model.PullRequest{
                {Number: 1, Title: "First", Author: "alice", Status: model.PRStatusOpen, Labels: []string{}},
                {Number: 2, Title: "Second", Author: "bob", Status: model.PRStatusMerged, Labels: []string{"bug"}},
            },
            wantStatus: http.StatusOK,
            wantCount:  2,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            store := &mockPRStore{prs: tt.stored}
            h := NewHandler(store, nil, nil, "testuser", slog.Default())

            req := httptest.NewRequest("GET", "/api/v1/prs", nil)
            rec := httptest.NewRecorder()

            h.ListPRs(rec, req)

            assert.Equal(t, tt.wantStatus, rec.Code)

            var resp []PRResponse
            err := json.Unmarshal(rec.Body.Bytes(), &resp)
            require.NoError(t, err)
            assert.Len(t, resp, tt.wantCount)
        })
    }
}
```

### Logging Middleware
```go
// Source: standard Go middleware pattern
func loggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        // Wrap ResponseWriter to capture status code
        sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
        next.ServeHTTP(sw, r)
        logger.Info("request",
            "method", r.Method,
            "path", r.URL.Path,
            "status", sw.status,
            "duration", time.Since(start).Round(time.Microsecond),
        )
    })
}

type statusWriter struct {
    http.ResponseWriter
    status int
}

func (w *statusWriter) WriteHeader(status int) {
    w.status = status
    w.ResponseWriter.WriteHeader(status)
}
```

### Recovery Middleware
```go
func recoveryMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if rec := recover(); rec != nil {
                logger.Error("panic recovered", "panic", rec, "path", r.URL.Path)
                writeError(w, http.StatusInternalServerError, "internal server error")
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

## API Design

### URL Structure

| Method | Path | Handler | Requirement |
|--------|------|---------|-------------|
| GET | /api/v1/prs | ListPRs | API-01, STAT-01, STAT-07 |
| GET | /api/v1/prs/attention | ListPRsNeedingAttention | API-03 |
| GET | /api/v1/repos/{owner}/{repo}/prs/{number} | GetPR | API-02 |
| GET | /api/v1/repos | ListRepos | REPO-03 |
| POST | /api/v1/repos | AddRepo | REPO-01 |
| DELETE | /api/v1/repos/{owner}/{repo} | RemoveRepo | REPO-02 |
| GET | /api/v1/health | Health | API-04 |

**Design rationale:**
- `/api/v1/` prefix allows API versioning without breaking changes
- PR detail uses `/repos/{owner}/{repo}/prs/{number}` for natural REST hierarchy and to match GitHub's URL structure
- `/prs/attention` is a filtered view of `/prs`, not a sub-resource -- hence sibling path
- Repository CRUD uses the repo full name embedded in the path for DELETE (idempotent) and POST body for creation (includes full_name)

### Response Envelope

No envelope (no `{ "data": ..., "meta": ... }`) for simplicity. Return arrays directly for list endpoints, objects directly for single resources. Error responses use `{ "error": "message" }`. This is the simplest approach for a CLI agent consumer.

### Time Format

All timestamps formatted as RFC 3339 (ISO 8601): `"2026-02-11T15:30:00Z"`. Use `time.Time.UTC().Format(time.RFC3339)`.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| gorilla/mux for routing | stdlib net/http.ServeMux | Go 1.22 (Feb 2024) | No external router dependency needed |
| chi/gin for method matching | stdlib method patterns ("GET /path") | Go 1.22 (Feb 2024) | Third-party routers add marginal value for small APIs |
| encoding/json v1 only | encoding/json/v2 experimental | Go 1.25 (2026) | v2 available via GOEXPERIMENT but not stable; stick with v1 |

**Deprecated/outdated:**
- gorilla/mux: In maintenance mode since 2022. Do not adopt.
- http.DefaultServeMux: Global state, makes testing hard. Always use `http.NewServeMux()`.
- GODEBUG=httpmuxgo121: Disables Go 1.22+ routing enhancements. Never set this.

## Open Questions

Things that could not be fully resolved:

1. **"Needs Attention" Definition at Phase 3**
   - What we know: API-03 requires PRs "needing attention (changes requested, needs review)". Phase 4 adds review state tracking.
   - What's unclear: Should Phase 3's attention endpoint include only "review requested" PRs (available from poll data), or also "changes requested" (which requires review data from Phase 4)?
   - Recommendation: Implement based on a persisted `needs_review` boolean flag set during polling (when `IsReviewRequestedFrom` returns true). The "changes requested" dimension will be added in Phase 4 when review data becomes available. Document this in the API response that the field may be refined in future phases.

2. **API-02 "Full Metadata" Scope at Phase 3**
   - What we know: API-02 says "single PR with full review comments and code context." But reviews and comments are Phase 4.
   - What's unclear: How much of API-02 should Phase 3 implement?
   - Recommendation: Phase 3 returns all PR metadata available (fields from STAT-01, STAT-07). Reviews and comments fields are present in the response schema but returned as empty arrays. Phase 4 populates them. This avoids breaking the API contract later.

3. **Triggering Repo Refresh After Add**
   - What we know: When a repo is added via POST, the poll service should discover PRs for it.
   - What's unclear: Should AddRepo synchronously trigger a poll, or let the next poll cycle pick it up?
   - Recommendation: Fire-and-forget refresh via `pollSvc.RefreshRepo()` in a goroutine after successful add. The POST returns 201 immediately with the repo data. PRs will appear on the next GET after the async refresh completes.

## Sources

### Primary (HIGH confidence)
- [Go 1.22 Routing Enhancements](https://go.dev/blog/routing-enhancements) - Method matching, wildcards, path parameters, precedence rules, 405 handling
- [Go net/http package docs](https://pkg.go.dev/net/http) - Server, ServeMux, Handler interface
- [Go net/http/httptest package docs](https://pkg.go.dev/net/http/httptest) - NewRequest, NewRecorder
- Existing codebase: main.go, domain/model/, port/driven/, adapter/driven/ - Architecture patterns, testing conventions

### Secondary (MEDIUM confidence)
- [Eli Bendersky: Better HTTP Routing in Go 1.22](https://eli.thegreenplace.net/2023/better-http-server-routing-in-go-122/) - Complete REST API example with ServeMux
- [Ben Hoyt: Improving Go RESTful API Tutorial](https://benhoyt.com/writings/web-service-stdlib/) - writeJSON/readJSON/jsonError patterns
- [Cloudflare: Complete Guide to Go net/http Timeouts](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/) - Server timeout configuration
- [VictoriaMetrics: Graceful Shutdown in Go](https://victoriametrics.com/blog/go-graceful-shutdown/) - Shutdown patterns

### Tertiary (LOW confidence)
- [Go Blog: JSON v2 Experimental](https://go.dev/blog/jsonv2-exp) - encoding/json/v2 status (experimental, not recommended for production)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All stdlib, verified against Go 1.22+ routing docs and existing codebase patterns
- Architecture: HIGH - Follows established hexagonal patterns in existing codebase, well-documented Go HTTP patterns
- Pitfalls: HIGH - Known Go HTTP gotchas verified against multiple authoritative sources
- API design: MEDIUM - URL structure is reasonable but no locked user decisions constrain it
- "Needs attention" logic: MEDIUM - Requires design decision about Phase 3 vs Phase 4 boundary

**Research date:** 2026-02-11
**Valid until:** 2026-03-11 (stable -- all stdlib, no fast-moving dependencies)
