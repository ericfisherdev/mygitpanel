# Architecture Patterns

**Domain:** Go API with GitHub integration, hexagonal architecture, background polling
**Researched:** 2026-02-10
**Confidence note:** Web research tools were unavailable. All recommendations below are based on training knowledge of well-established Go patterns (hexagonal architecture, standard project layout, Go concurrency primitives). These patterns are mature and stable, making training-based knowledge reliable here. Individual library version claims should be verified at implementation time.

---

## Recommended Architecture

ReviewHub follows **hexagonal architecture** (ports and adapters), placing the domain model at the center with all infrastructure concerns pushed to the boundary. In Go, this maps naturally to interfaces (ports) defined in the domain layer and concrete implementations (adapters) in separate packages.

```
                    +---------------------------+
                    |      REST API (chi)       |  <-- Primary/Driving Adapter
                    +------------+--------------+
                                 |
                    +------------v--------------+
                    |     Application Layer     |  <-- Use cases / orchestration
                    |  (PRService, RepoService, |
                    |   PollingService)          |
                    +---+-----------+-----------+
                        |           |           |
           +------------+    +------+------+    +------------+
           |  Domain    |    |  Domain     |    |  Domain    |
           |  Ports     |    |  Model      |    |  Ports     |
           | (driven)   |    | (entities,  |    | (driving)  |
           |            |    |  value objs)|    |            |
           +-----+------+    +-------------+    +-----+------+
                 |                                    |
        +--------+--------+                  +--------+--------+
        | SQLite Adapter  |                  | GitHub Adapter  |
        | (persistence)   |                  | (API client)    |
        +-----------------+                  +-----------------+
```

### Directory Structure

```
reviewhub/
|-- cmd/
|   +-- reviewhub/
|       +-- main.go                 # Entry point, wiring, DI
|
|-- internal/
|   |-- domain/
|   |   |-- model/
|   |   |   |-- pullrequest.go      # PR entity
|   |   |   |-- repository.go       # Watched repository entity
|   |   |   |-- review.go           # Review/comment entity
|   |   |   |-- reviewcomment.go    # Review comment with code snippet
|   |   |   +-- checkstatus.go      # CI/CD check status value object
|   |   |
|   |   +-- port/
|   |       |-- driven/
|   |       |   |-- prstore.go      # PRStore interface (persistence port)
|   |       |   |-- repostore.go    # RepoStore interface (persistence port)
|   |       |   +-- githubclient.go # GitHubClient interface (external API port)
|   |       |
|   |       +-- driving/
|   |           |-- prservice.go    # PRService interface (use case port)
|   |           |-- reposervice.go  # RepoService interface (use case port)
|   |           +-- pollservice.go  # PollService interface (use case port)
|   |
|   |-- application/
|   |   |-- prservice.go            # PRService implementation
|   |   |-- reposervice.go          # RepoService implementation
|   |   |-- pollservice.go          # PollService / scheduling orchestration
|   |   +-- commentenricher.go      # Review comment enrichment logic
|   |
|   |-- adapter/
|   |   |-- driven/
|   |   |   |-- sqlite/
|   |   |   |   |-- prrepo.go       # SQLite PRStore adapter
|   |   |   |   |-- reporepo.go     # SQLite RepoStore adapter
|   |   |   |   |-- migrations.go   # Schema migrations
|   |   |   |   +-- db.go           # Connection management
|   |   |   |
|   |   |   +-- github/
|   |   |       |-- client.go       # GitHub API client adapter
|   |   |       |-- mapper.go       # GitHub API response -> domain model
|   |   |       +-- ratelimit.go    # Rate limit tracking
|   |   |
|   |   +-- driving/
|   |       +-- http/
|   |           |-- server.go       # HTTP server setup
|   |           |-- router.go       # Route definitions
|   |           |-- prhandler.go    # PR endpoint handlers
|   |           |-- repohandler.go  # Repository CRUD handlers
|   |           |-- statushandler.go # Health/status endpoints
|   |           +-- response.go     # JSON response helpers
|   |
|   +-- config/
|       +-- config.go               # Configuration loading (env vars)
|
|-- Dockerfile
|-- docker-compose.yml
|-- go.mod
+-- go.sum
```

### Why This Structure

**`cmd/`** -- Standard Go convention. The `main.go` here is the composition root where all dependencies are wired together. This is the only place that knows about all concrete types.

**`internal/`** -- Go's enforced encapsulation. Nothing outside the module can import these packages. This is critical for hexagonal architecture because it prevents leaking implementation details.

**`domain/model/`** -- Pure domain types with zero external dependencies. No imports from `adapter/`, `application/`, or any third-party library. These are plain Go structs and methods.

**`domain/port/`** -- Interfaces split into `driving` (primary, things that call into the domain) and `driven` (secondary, things the domain calls out to). The split makes dependency direction unambiguous.

**`application/`** -- Use case orchestration. These types implement the driving port interfaces and depend on the driven port interfaces. This is where business logic composition happens.

**`adapter/driven/`** -- Concrete implementations of driven ports. SQLite for persistence, GitHub API client for external data. These depend on the port interfaces and the domain model.

**`adapter/driving/http/`** -- HTTP handlers that translate HTTP requests into application service calls. Depends on driving port interfaces.

---

## Component Boundaries

| Component | Package | Responsibility | Depends On | Depended On By |
|-----------|---------|---------------|------------|----------------|
| **Domain Model** | `internal/domain/model` | PR, Repository, Review entities and value objects | Nothing (zero deps) | Everything |
| **Driven Ports** | `internal/domain/port/driven` | Interfaces for persistence and external APIs | `domain/model` | `application`, adapters |
| **Driving Ports** | `internal/domain/port/driving` | Interfaces for use cases | `domain/model` | HTTP adapter, `cmd/` |
| **Application Services** | `internal/application` | Use case orchestration, comment enrichment | `domain/model`, `domain/port/driven` | Driving adapters via interfaces |
| **SQLite Adapter** | `internal/adapter/driven/sqlite` | PR and repository persistence | `domain/model`, `domain/port/driven` | `cmd/` (wiring only) |
| **GitHub Adapter** | `internal/adapter/driven/github` | GitHub API communication, response mapping | `domain/model`, `domain/port/driven` | `cmd/` (wiring only) |
| **HTTP Adapter** | `internal/adapter/driving/http` | REST API, request/response handling | `domain/model`, `domain/port/driving` | `cmd/` (wiring only) |
| **Polling Scheduler** | `internal/application/pollservice.go` | Periodic GitHub polling, coordination | `domain/port/driven` (GitHubClient, stores) | `cmd/` starts it |
| **Config** | `internal/config` | Environment variable loading | Nothing external | `cmd/` |
| **Composition Root** | `cmd/reviewhub/main.go` | Wiring all components together | Everything (only place) | Nothing |

### Dependency Rule (STRICT)

```
Domain Model  <--  Domain Ports  <--  Application  <--  Adapters  <--  main.go
    (0 deps)      (model only)      (ports only)     (ports+model)   (everything)
```

Dependencies point INWARD only. The domain model never imports from application or adapter packages. Application services never import from adapter packages. Adapters know about ports and models but never about other adapters. Only `main.go` sees concrete types.

---

## Data Flow

### Flow 1: HTTP Request for PR Data

```
1. HTTP Request arrives at driving/http handler
2. Handler deserializes request, calls driving port interface method
3. Application service (implements driving port) executes use case:
   a. Queries PRStore (driven port) for stored PR data
   b. Returns domain model objects
4. Handler serializes domain model to JSON response
5. HTTP Response returned
```

### Flow 2: Background Polling Cycle

```
1. Polling scheduler tick fires (Go ticker)
2. PollService calls RepoStore to get list of watched repositories
3. For each repository:
   a. Calls GitHubClient (driven port) to fetch PRs
   b. GitHubClient adapter calls GitHub REST API v3
   c. Adapter maps GitHub API response to domain model
   d. PollService compares fetched PRs with stored PRs
   e. For new/changed PRs: fetches review comments via GitHubClient
   f. CommentEnricher formats comments with code snippets
   g. PollService calls PRStore to upsert PR data
4. Scheduler waits for next tick
```

### Flow 3: Repository CRUD

```
1. HTTP Request (POST/PUT/DELETE /repos)
2. Handler calls RepoService (driving port)
3. RepoService validates input (domain rules)
4. RepoService calls RepoStore (driven port) to persist
5. If adding new repo: triggers immediate poll for that repo
6. Response returned
```

### Flow 4: Comment Enrichment (Core Domain Logic)

```
1. GitHub adapter fetches raw review comment (includes file path, line, body)
2. GitHub adapter fetches the PR diff/patch
3. CommentEnricher (application layer):
   a. Locates the relevant file in the diff
   b. Extracts the hunk containing the commented line
   c. Builds a CodeSnippet with surrounding context lines
   d. Attaches snippet to ReviewComment domain object
4. Enriched ReviewComment stored via PRStore
```

This is the core value proposition of the system: transforming raw GitHub review data into AI-consumable context.

---

## Patterns to Follow

### Pattern 1: Interface-Based Dependency Injection via Constructor

**What:** Define interfaces in the domain/port packages. Application services accept these interfaces via constructor parameters. Wire concrete implementations in `main.go`.

**Why:** This is the standard Go approach to hexagonal architecture. Go interfaces are implicitly satisfied (no `implements` keyword), so adapters satisfy port interfaces without importing the port package -- though explicit imports for documentation clarity are acceptable.

**Confidence:** HIGH -- this is idiomatic Go, well-established.

```go
// internal/domain/port/driven/prstore.go
package driven

import "reviewhub/internal/domain/model"

type PRStore interface {
    GetByRepository(ctx context.Context, repoFullName string) ([]model.PullRequest, error)
    Upsert(ctx context.Context, pr model.PullRequest) error
    GetByStatus(ctx context.Context, status model.PRStatus) ([]model.PullRequest, error)
}
```

```go
// internal/application/prservice.go
package application

type PRService struct {
    prStore  driven.PRStore
    ghClient driven.GitHubClient
}

func NewPRService(store driven.PRStore, gh driven.GitHubClient) *PRService {
    return &PRService{prStore: store, ghClient: gh}
}
```

```go
// cmd/reviewhub/main.go
func main() {
    db := sqlite.NewDB(cfg.DatabasePath)
    prStore := sqlite.NewPRRepo(db)
    ghClient := github.NewClient(cfg.GitHubToken)
    prService := application.NewPRService(prStore, ghClient)
    // ... wire HTTP handlers with prService
}
```

### Pattern 2: Context-Based Cancellation for Polling

**What:** Use `context.Context` throughout, with a cancellable context for the background polling goroutine. On shutdown, cancel the context to cleanly stop polling.

**Why:** Go's context package is the standard mechanism for lifecycle management. The polling scheduler runs as a goroutine; context cancellation ensures clean shutdown without leaked goroutines.

**Confidence:** HIGH -- standard Go concurrency pattern.

```go
// internal/application/pollservice.go
type PollService struct {
    interval time.Duration
    ghClient driven.GitHubClient
    prStore  driven.PRStore
    repoStore driven.RepoStore
    enricher *CommentEnricher
}

func (s *PollService) Start(ctx context.Context) {
    ticker := time.NewTicker(s.interval)
    defer ticker.Stop()

    // Poll immediately on start
    s.pollAll(ctx)

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.pollAll(ctx)
        }
    }
}
```

### Pattern 3: Graceful Shutdown Orchestration

**What:** Use `signal.NotifyContext` to create a context that cancels on SIGINT/SIGTERM. Pass this to both the HTTP server and the polling goroutine.

**Why:** Docker sends SIGTERM on container stop. The application must drain in-flight requests and stop polling cleanly. This is critical for SQLite (avoid corrupted writes).

**Confidence:** HIGH -- standard Go server pattern.

```go
func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    // Start polling in background
    go pollService.Start(ctx)

    // Start HTTP server (blocks)
    server.ListenAndServe(ctx)
}
```

### Pattern 4: Domain Model as Plain Structs with Methods

**What:** Domain entities are plain Go structs with behavior methods. No ORM tags, no JSON tags, no database annotations on domain types.

**Why:** Domain model purity is the core of hexagonal architecture. Serialization concerns belong in adapters. The domain model should be testable without any infrastructure.

**Confidence:** HIGH -- fundamental hexagonal architecture principle.

```go
// internal/domain/model/pullrequest.go
package model

type PullRequest struct {
    ID            int64
    Number        int
    RepoFullName  string
    Title         string
    Author        string
    Status        PRStatus
    ReviewStatus  ReviewStatus
    CIStatus      CIStatus
    CoderabbitReviewed bool
    DiffStats     DiffStats
    OpenedAt      time.Time
    LastActivityAt time.Time
    Comments      []ReviewComment
}

func (pr *PullRequest) DaysSinceOpened() int {
    return int(time.Since(pr.OpenedAt).Hours() / 24)
}

func (pr *PullRequest) IsStale(thresholdDays int) bool {
    return pr.DaysSinceLastActivity() > thresholdDays
}
```

### Pattern 5: Adapter-Layer Mapping (Separate from Domain)

**What:** Each adapter defines its own data structures for serialization and maps to/from domain types. GitHub API responses map to domain model in the GitHub adapter. SQLite rows map to domain model in the SQLite adapter. HTTP responses map from domain model in the HTTP adapter.

**Why:** Prevents domain model contamination. GitHub API response shape, SQLite column names, and JSON API response format can all change independently without touching the domain.

**Confidence:** HIGH -- core hexagonal architecture principle.

```go
// internal/adapter/driven/github/mapper.go
package github

func toDomainPR(ghPR *ghPullRequest, repoFullName string) model.PullRequest {
    return model.PullRequest{
        Number:       ghPR.Number,
        RepoFullName: repoFullName,
        Title:        ghPR.Title,
        Author:       ghPR.User.Login,
        Status:       mapStatus(ghPR),
        // ...
    }
}
```

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Leaking GitHub Types into the Domain

**What:** Importing `github.com/google/go-github` types directly in domain or application packages.

**Why bad:** Couples the domain to a third-party API client library. If the GitHub library changes (or you want to swap it for raw HTTP calls), changes cascade through the entire codebase.

**Instead:** The GitHub adapter translates API responses to domain model types. The domain never sees a `github.PullRequest`; it only sees `model.PullRequest`.

### Anti-Pattern 2: "Hexagonal" with Shared Database Models

**What:** Using the same struct for domain entity and database row (e.g., adding `db:"column_name"` tags to domain types).

**Why bad:** Couples domain evolution to database schema. Adding a computed domain field forces a schema migration. Renaming a domain field breaks queries.

**Instead:** SQLite adapter defines its own row structs and maps to/from domain model. The mapping code is explicit and lives in the adapter.

### Anti-Pattern 3: Fat Handlers (Business Logic in HTTP Layer)

**What:** Putting polling logic, comment enrichment, or status computation directly in HTTP handlers.

**Why bad:** Makes logic untestable without HTTP. Violates separation of concerns. Handlers should be thin -- deserialize, delegate, serialize.

**Instead:** Handlers call application service methods. All logic lives in the application layer or domain model.

### Anti-Pattern 4: Global Database Connection

**What:** Using a package-level `var db *sql.DB` that all code imports.

**Why bad:** Untestable (can't inject a test database), violates dependency inversion, creates hidden coupling.

**Instead:** Pass `*sql.DB` (or a wrapper) to adapter constructors. The adapter owns its database interactions.

### Anti-Pattern 5: Polling in the HTTP Handler Goroutine

**What:** Running the polling loop inside the HTTP server's goroutine pool or triggered by HTTP requests.

**Why bad:** Polling should be independent of HTTP traffic. If no one calls the API, polling still needs to happen. Mixing concerns makes shutdown complex.

**Instead:** Polling runs in its own goroutine, started from `main.go`, with its own context for lifecycle management.

### Anti-Pattern 6: Enormous Port Interfaces

**What:** A single `Repository` interface with 20+ methods covering PRs, repos, comments, checks, and diff stats.

**Why bad:** Violates Interface Segregation Principle. Mocking becomes painful. Changes to one area affect unrelated consumers.

**Instead:** Small, focused interfaces: `PRStore`, `RepoStore`, `ReviewStore`. Each has 3-5 methods max. Consumers depend only on the interface they need.

---

## Key Architecture Decisions

### Decision 1: GitHub REST API v3, Not GraphQL v4

**Recommendation:** Use GitHub REST API v3 with the `google/go-github` library.

**Rationale:**
- REST API is simpler to implement and debug
- `go-github` is the mature, well-maintained Go client (HIGH confidence)
- For the data we need (PRs, reviews, comments, checks), REST endpoints are straightforward
- GraphQL would reduce API calls but adds query complexity that is not justified for v1
- Rate limits are generous for authenticated REST (5,000 requests/hour)

**Confidence:** MEDIUM -- library recommendation based on training data; verify current version at implementation time.

### Decision 2: chi Router over Standard Library

**Recommendation:** Use `go-chi/chi` for HTTP routing.

**Rationale:**
- Lightweight, idiomatic Go router
- Middleware ecosystem (logging, recovery, request ID)
- Go 1.22+ improved `net/http` routing, so standard library is also viable
- chi adds route grouping and middleware chaining that simplify the handler organization
- If the team prefers zero dependencies, `net/http` with Go 1.22+ patterns is acceptable

**Alternative:** Standard library `net/http` with Go 1.22+ method routing. This is a valid choice that reduces dependencies.

**Confidence:** MEDIUM -- chi is well-established but Go's standard library routing has improved significantly. Verify Go version constraints.

### Decision 3: modernc.org/sqlite over mattn/go-sqlite3

**Recommendation:** Use `modernc.org/sqlite` (pure Go SQLite) rather than `mattn/go-sqlite3` (CGo wrapper).

**Rationale:**
- Pure Go: no CGo required, dramatically simpler cross-compilation and Docker builds
- Docker multi-stage builds become trivial (no need for gcc in build stage)
- Slightly slower than CGo version but ReviewHub's query volume is trivially small
- Removes entire class of build issues on different platforms

**Confidence:** MEDIUM -- based on training data. Verify current version and API compatibility at implementation time.

### Decision 4: Embedded Migrations, Not External Tool

**Recommendation:** Use Go 1.16+ `embed` package to embed SQL migration files, with a simple migration runner at startup.

**Rationale:**
- Single binary deployment (migrations are in the binary)
- No external migration tool needed in Docker image
- SQLite schema is simple enough that a lightweight approach works
- Run migrations in `main.go` before starting services

**Confidence:** HIGH -- `embed` is standard library, well-established pattern.

---

## Component Interaction Diagram

```
main.go (composition root)
  |
  |-- Creates: config.Load()
  |-- Creates: sqlite.NewDB(path)
  |-- Creates: sqlite.NewPRRepo(db)       --> implements driven.PRStore
  |-- Creates: sqlite.NewRepoRepo(db)     --> implements driven.RepoStore
  |-- Creates: github.NewClient(token)    --> implements driven.GitHubClient
  |-- Creates: application.NewCommentEnricher()
  |-- Creates: application.NewPRService(prRepo, ghClient, enricher)
  |-- Creates: application.NewRepoService(repoRepo)
  |-- Creates: application.NewPollService(interval, ghClient, prRepo, repoRepo, enricher)
  |-- Creates: http.NewServer(prService, repoService)
  |
  |-- Starts:  go pollService.Start(ctx)  // background goroutine
  |-- Starts:  server.ListenAndServe(ctx) // blocks until shutdown
  |
  +-- On SIGTERM: ctx cancels -> polling stops, server drains, DB closes
```

---

## Scalability Considerations

| Concern | Current (single user) | If Multi-User Later | Notes |
|---------|----------------------|---------------------|-------|
| GitHub rate limits | 5,000 req/hr, plenty for one user | Would need per-token tracking | Polling interval tuning is sufficient for v1 |
| SQLite concurrency | Single writer fine for polling + API reads | Would need Postgres or WAL tuning | WAL mode recommended from day 1 for concurrent reads |
| Polling frequency | Every 2-5 min is reasonable | Would need per-repo scheduling | Simple ticker is fine for v1 |
| Memory usage | Trivial for dozens of PRs | Pagination needed for 1000s of PRs | Go-github handles pagination; store only open PRs by default |

### SQLite Configuration (Important)

Enable WAL mode on database open. This allows concurrent reads while a write is in progress, which matters because the HTTP server reads while the poller writes.

```go
db.Exec("PRAGMA journal_mode=WAL")
db.Exec("PRAGMA busy_timeout=5000")
db.Exec("PRAGMA foreign_keys=ON")
```

**Confidence:** HIGH -- SQLite WAL mode is well-documented and critical for this use case.

---

## Suggested Build Order

The dependency graph dictates a natural build order. Each phase can be tested independently because hexagonal architecture enforces clean boundaries.

### Phase 1: Domain Model + Ports (Foundation)

Build first because everything else depends on these types.

- `internal/domain/model/` -- All entities and value objects
- `internal/domain/port/driven/` -- PRStore, RepoStore, GitHubClient interfaces
- `internal/domain/port/driving/` -- PRService, RepoService, PollService interfaces

**Zero external dependencies.** Can be written and tested with unit tests immediately.

### Phase 2: SQLite Adapter (Persistence)

Build second because the application layer needs a working store to integrate.

- `internal/adapter/driven/sqlite/` -- DB setup, migrations, PRRepo, RepoRepo
- Implements driven port interfaces
- Can be tested with an in-memory SQLite database

**Depends on:** Phase 1 (domain model and port interfaces)

### Phase 3: GitHub Adapter (External Integration)

Can be built in parallel with Phase 2 if desired.

- `internal/adapter/driven/github/` -- Client, mapper, rate limit tracking
- Implements GitHubClient driven port interface
- Testable with recorded HTTP responses

**Depends on:** Phase 1 (domain model and port interfaces)

### Phase 4: Application Services + Comment Enrichment

Build after adapters exist so integration testing is possible.

- `internal/application/` -- PRService, RepoService, CommentEnricher
- Implements driving port interfaces
- Core business logic: comment enrichment, status computation, staleness tracking
- Testable with mock driven ports

**Depends on:** Phase 1 (ports), testable without Phase 2/3 via mocks

### Phase 5: HTTP Adapter (API Layer)

Build after application services exist.

- `internal/adapter/driving/http/` -- Server, router, handlers, response formatting
- Thin layer: deserialize, delegate to application service, serialize
- Testable with `httptest` and mock driving ports

**Depends on:** Phase 4 (application services via driving port interfaces)

### Phase 6: Polling Scheduler + Wiring

Build last because it orchestrates all components.

- `internal/application/pollservice.go` -- Background polling loop
- `cmd/reviewhub/main.go` -- Composition root, graceful shutdown
- `Dockerfile`, `docker-compose.yml` -- Containerization

**Depends on:** All previous phases

### Build Order Rationale

This ordering follows the **dependency rule**: build the innermost ring first (domain), then work outward. Each phase is independently testable. Phases 2 and 3 can be parallelized because they both depend only on Phase 1 and don't depend on each other. The composition root (Phase 6) is last because it needs all concrete types to wire together.

---

## Testing Strategy by Layer

| Layer | Test Type | Approach |
|-------|-----------|----------|
| Domain Model | Unit tests | Pure logic, no mocks needed |
| Application Services | Unit tests with mocks | Mock driven ports, verify orchestration |
| SQLite Adapter | Integration tests | In-memory SQLite, verify queries |
| GitHub Adapter | Integration tests | Recorded HTTP responses (httptest or go-vcr) |
| HTTP Adapter | Integration tests | `httptest.NewServer`, mock application services |
| Full System | End-to-end | Docker container, mocked GitHub API |

The hexagonal structure means each layer can be tested in isolation. Domain and application tests run in milliseconds with no I/O. Adapter tests use lightweight fakes (in-memory SQLite, recorded HTTP). This makes the test suite fast and reliable.

---

## Sources

- Go hexagonal architecture patterns: Based on established community conventions (Alistair Cockburn's hexagonal architecture applied to Go; Three Dots Labs' "Wild Workouts" example; Go standard project layout conventions). MEDIUM confidence -- training data, not live-verified.
- `google/go-github` library: Well-known Go GitHub client. MEDIUM confidence -- verify current version.
- `modernc.org/sqlite`: Pure Go SQLite driver. MEDIUM confidence -- verify current API.
- `go-chi/chi`: Lightweight Go router. MEDIUM confidence -- verify vs Go 1.22+ stdlib routing.
- Go standard library (`context`, `signal`, `embed`, `net/http`): HIGH confidence -- stable standard library.
- SQLite WAL mode, PRAGMA settings: HIGH confidence -- stable SQLite behavior.
